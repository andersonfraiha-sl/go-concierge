package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/google/uuid"

	"go-concierge/internal/escopo"
	"go-concierge/internal/ganchos/regras"
)

// Exportar e importar regras. O arquivo é assinado com HMAC-SHA256 pelo segredo da
// instalação (GANCHOS_SEGREDO_EXPORTACAO): a instalação dedicada recebe as regras assinadas
// pela Superlógica e quem importa lá não consegue mudar o conteúdo.

const (
	formatoDasRegras       = "ganchos-regras/1"
	limiteArquivo          = 5 << 20
	limiteRegrasImportadas = 1000
	codigoArquivoGrande    = "ARQUIVO_GRANDE_DEMAIS"
	paginaDaExportacao     = limiteMaximo
)

var assinaturaInvalida = recusa{status: http.StatusUnprocessableEntity, codigo: "ASSINATURA_INVALIDA",
	mensagem: "assinatura não confere: o arquivo foi alterado ou veio de outra instalação"}

type arquivoDeRegras struct {
	Formato    string            `json:"formato"`
	Regras     []json.RawMessage `json:"regras"`
	Assinatura string            `json:"assinatura"`
}

// assinar calcula o HMAC sobre o conteúdo canônico de formato e regras: os valores
// decodificados e codificados de novo pelo encoding/json (chaves em ordem, sem espaços).
// Reformatar o arquivo não quebra a assinatura; mudar qualquer valor quebra.
func assinar(segredo []byte, formato string, itens []json.RawMessage) ([]byte, error) {
	valores := make([]any, len(itens))
	for i, item := range itens {
		d := json.NewDecoder(bytes.NewReader(item))
		d.UseNumber()
		if err := d.Decode(&valores[i]); err != nil {
			return nil, fmt.Errorf("regra %d: %w", i, err)
		}
	}
	canonico, err := json.Marshal(struct {
		Formato string `json:"formato"`
		Regras  []any  `json:"regras"`
	}{formato, valores})
	if err != nil {
		return nil, fmt.Errorf("conteúdo canônico: %w", err)
	}
	mac := hmac.New(sha256.New, segredo)
	mac.Write(canonico)
	return mac.Sum(nil), nil
}

func (s *servidor) exigirSegredo() error {
	if len(s.segredoExportacao) == 0 {
		return recusa{status: http.StatusServiceUnavailable, codigo: "SEGREDO_AUSENTE", mensagem: "GANCHOS_SEGREDO_EXPORTACAO não configurado"}
	}
	return nil
}

// ---- exportar ----------------------------------------------------------------

func (s *servidor) ganchosExportar(w http.ResponseWriter, r *http.Request) {
	adm, err := s.administradoraDaExportacao(r)
	if err != nil {
		s.responderGanchos(w, r, "exportar regras", err)
		return
	}
	arquivo, err := s.exportarRegras(r.Context(), adm)
	if err != nil {
		s.responderGanchos(w, r, "exportar regras", err)
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="ganchos-regras.json"`)
	writeJSON(w, http.StatusOK, arquivo)
}

// administradoraDaExportacao: a fixa da implantação, sempre; senão a de ?administradora=
// (vazia = todas as regras).
func (s *servidor) administradoraDaExportacao(r *http.Request) (string, error) {
	if fixa := s.implantacao.AdministradoraFixa; fixa != "" {
		return fixa, nil
	}
	adm := strings.TrimSpace(r.URL.Query().Get("administradora"))
	if adm != "" && !escopo.AdministradoraValida(adm) {
		return "", recusa{status: http.StatusBadRequest, codigo: "ADMINISTRADORA_INVALIDA", mensagem: "administradora com formato inválido"}
	}
	return adm, nil
}

// exportarRegras assina as regras da administradora adm e as globais; adm vazia exporta todas.
func (s *servidor) exportarRegras(ctx context.Context, adm string) (arquivoDeRegras, error) {
	if err := s.exigirSegredo(); err != nil {
		return arquivoDeRegras{}, err
	}
	todas, err := s.todasAsRegras(ctx)
	if err != nil {
		return arquivoDeRegras{}, err
	}
	if adm != "" {
		todas = slices.DeleteFunc(todas, func(g regras.RegraGravada) bool {
			return g.AdministradoraID != "" && g.AdministradoraID != adm
		})
	}
	return s.assinarRegras(todas)
}

func (s *servidor) assinarRegras(todas []regras.RegraGravada) (arquivoDeRegras, error) {
	var err error
	itens := make([]json.RawMessage, len(todas))
	for i, g := range todas {
		if itens[i], err = comID(g.Definicao, g.ID); err != nil {
			return arquivoDeRegras{}, fmt.Errorf("regra %s: %w", g.ID, err)
		}
	}
	mac, err := assinar(s.segredoExportacao, formatoDasRegras, itens)
	if err != nil {
		return arquivoDeRegras{}, err
	}
	return arquivoDeRegras{Formato: formatoDasRegras, Regras: itens, Assinatura: hex.EncodeToString(mac)}, nil
}

// todasAsRegras percorre a lista inteira, ativas ou não, página a página.
func (s *servidor) todasAsRegras(ctx context.Context) ([]regras.RegraGravada, error) {
	var todas []regras.RegraGravada
	depois := ""
	for {
		pagina, err := s.regras.Listar(ctx, depois, paginaDaExportacao)
		if err != nil {
			return nil, err
		}
		todas = append(todas, pagina...)
		if len(pagina) < paginaDaExportacao {
			return todas, nil
		}
		depois = pagina[len(pagina)-1].ID
	}
}

// comID põe o id na definição: importar o mesmo arquivo atualiza a mesma regra.
func comID(definicao json.RawMessage, id string) (json.RawMessage, error) {
	idJSON, err := json.Marshal(id)
	if err != nil {
		return nil, err
	}
	return comCampos(definicao, func(campos map[string]json.RawMessage) { campos["id"] = idJSON })
}

// ---- importar ----------------------------------------------------------------

// regraImportada é uma regra do arquivo já decodificada, com o id que vai gravar.
type regraImportada struct {
	regra     regras.Regra
	definicao json.RawMessage
}

// ganchosImportar grava tudo ou nada: assinatura, decodificação, alvo e validação de todas
// vêm antes da primeira escrita, e a gravação é uma transação só.
func (s *servidor) ganchosImportar(w http.ResponseWriter, r *http.Request) {
	itens, err := s.lerArquivoAssinado(w, r)
	if err != nil {
		s.responderGanchos(w, r, "importar regras", err)
		return
	}
	importadas, err := s.decodificarImportadas(itens)
	if err == nil {
		err = s.validarImportadas(r.Context(), importadas)
	}
	if err != nil {
		s.responderGanchos(w, r, "importar regras", err)
		return
	}
	salvas, err := s.gravarImportadas(r.Context(), importadas, usuarioDe(r))
	if err != nil {
		s.responderGanchos(w, r, "importar regras", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"importadas": len(salvas)})
}

// lerArquivoAssinado confere tamanho, assinatura e formato, nessa ordem, antes de olhar as
// regras.
func (s *servidor) lerArquivoAssinado(w http.ResponseWriter, r *http.Request) ([]json.RawMessage, error) {
	if err := s.exigirSegredo(); err != nil {
		return nil, err
	}
	raw, err := lerCorpoBruto(w, r, limiteArquivo, codigoArquivoGrande)
	if err != nil {
		return nil, err
	}
	var arquivo arquivoDeRegras
	if err := json.Unmarshal(raw, &arquivo); err != nil {
		return nil, recusa{status: http.StatusBadRequest, codigo: "CORPO_INVALIDO", mensagem: "arquivo JSON inválido"}
	}
	if len(arquivo.Regras) > limiteRegrasImportadas {
		return nil, recusa{status: http.StatusUnprocessableEntity, codigo: codigoArquivoGrande, mensagem: "no máximo 1000 regras por arquivo"}
	}
	if err := s.conferirAssinatura(arquivo); err != nil {
		return nil, err
	}
	if arquivo.Formato != formatoDasRegras {
		return nil, recusa{status: http.StatusUnprocessableEntity, codigo: "FORMATO_DESCONHECIDO", mensagem: "formato esperado: " + formatoDasRegras}
	}
	return arquivo.Regras, nil
}

func (s *servidor) conferirAssinatura(arquivo arquivoDeRegras) error {
	if arquivo.Assinatura == "" {
		return recusa{status: http.StatusUnprocessableEntity, codigo: "ARQUIVO_NAO_ASSINADO", mensagem: "arquivo sem assinatura"}
	}
	recebida, err := hex.DecodeString(arquivo.Assinatura)
	if err != nil {
		return assinaturaInvalida
	}
	esperada, err := assinar(s.segredoExportacao, arquivo.Formato, arquivo.Regras)
	if err != nil || !hmac.Equal(esperada, recebida) {
		return assinaturaInvalida
	}
	return nil
}

// decodificarImportadas decodifica todas e junta os problemas com o índice de cada uma.
// Regra sem id ganha um; id repetido no arquivo é problema. Numa implantação dedicada, alvo
// de outra administradora recusa o arquivo inteiro com ALVO_FORA_DA_IMPLANTACAO.
func (s *servidor) decodificarImportadas(itens []json.RawMessage) ([]regraImportada, error) {
	importadas := make([]regraImportada, len(itens))
	var problemas, foraDaFixa []problemaJSON
	vistos := map[string]bool{}
	for i, item := range itens {
		regra, definicao, err := decodificarRegra(item, &i)
		if err != nil {
			problemas = append(problemas, problemasDe(err, i)...)
			continue
		}
		regra.ID, err = idDaImportada(regra.ID, vistos)
		if err != nil {
			problemas = append(problemas, problemaJSON{Regra: &i, Campo: "id", Codigo: "ID_INVALIDO", Mensagem: err.Error()})
		}
		if s.foraDaImplantacao(regra.Alvo) {
			foraDaFixa = append(foraDaFixa, problemaJSON{Regra: &i, Campo: "alvo.administradora", Codigo: "ALVO_FORA_DA_IMPLANTACAO", Mensagem: "alvo de outra administradora"})
		}
		importadas[i] = regraImportada{regra, definicao}
	}
	if len(foraDaFixa) > 0 {
		return nil, recusa{http.StatusUnprocessableEntity, "ALVO_FORA_DA_IMPLANTACAO", "o arquivo tem regras de outra administradora", foraDaFixa}
	}
	if len(problemas) > 0 {
		return nil, regraInvalida(problemas)
	}
	return importadas, nil
}

// foraDaImplantacao: numa implantação dedicada, alvo de outra administradora. Alvo global
// vale para todos os condomínios dela.
func (s *servidor) foraDaImplantacao(alvo regras.Alvo) bool {
	fixa := s.implantacao.AdministradoraFixa
	return fixa != "" && alvo.Administradora != "" && alvo.Administradora != fixa
}

// problemasDe devolve os problemas de uma recusa de decodificação; outro erro vira um
// problema genérico da regra i.
func problemasDe(err error, i int) []problemaJSON {
	if e, ok := errors.AsType[recusa](err); ok {
		return e.problemas
	}
	return []problemaJSON{{Regra: &i, Codigo: "JSON_INVALIDO", Mensagem: err.Error()}}
}

func idDaImportada(id string, vistos map[string]bool) (string, error) {
	if id == "" {
		return uuid.NewString(), nil
	}
	if uuid.Validate(id) != nil {
		return id, fmt.Errorf("id %q não é um uuid", id)
	}
	if vistos[id] {
		return id, fmt.Errorf("id %q repetido no arquivo", id)
	}
	vistos[id] = true
	return id, nil
}

// validarImportadas valida cada uma contra o catálogo e os limites do conjunto final: as
// ativas de agora com as importadas no lugar das de mesmo id.
// ponytail: O(importadas × ativas) nos limites, com teto de 1000 × 10 mil; um validador em
// lote no pacote regras resolve se pesar.
func (s *servidor) validarImportadas(ctx context.Context, importadas []regraImportada) error {
	cat, ativas, err := s.contextoDaValidacao(ctx)
	if err != nil {
		return err
	}
	final := conjuntoFinal(ativas, importadas)
	var problemas []problemaJSON
	for i, imp := range importadas {
		ps := append(regras.ValidarRegra(imp.regra, cat), regras.ValidarLimites(final, imp.regra, cat)...)
		problemas = append(problemas, problemasJSON(ps, &i)...)
	}
	if len(problemas) > 0 {
		return regraInvalida(problemas)
	}
	return nil
}

func conjuntoFinal(ativas []regras.Regra, importadas []regraImportada) []regras.Regra {
	porID := make(map[string]regras.Regra, len(ativas)+len(importadas))
	for _, r := range ativas {
		porID[r.ID] = r
	}
	for _, imp := range importadas {
		porID[imp.regra.ID] = imp.regra
	}
	final := make([]regras.Regra, 0, len(porID))
	for _, r := range porID {
		final = append(final, r)
	}
	return final
}

// gravarImportadas grava cada uma como versão nova da regra de mesmo id (ou cria), numa
// transação só. Quem gravou no meio faz a importação inteira dar 409.
func (s *servidor) gravarImportadas(ctx context.Context, importadas []regraImportada, autor string) ([]regras.RegraGravada, error) {
	ids := make([]string, len(importadas))
	for i, imp := range importadas {
		ids[i] = imp.regra.ID
	}
	atuais, err := s.regras.VersoesAtuais(ctx, ids)
	if err != nil {
		return nil, err
	}
	gravadas := make([]regras.RegraGravada, len(importadas))
	lidas := make([]int, len(importadas))
	for i, imp := range importadas {
		gravadas[i] = gravadaDe(imp.regra, imp.definicao, autor)
		lidas[i] = atuais[imp.regra.ID]
	}
	return s.regras.SalvarVarias(ctx, gravadas, lidas)
}
