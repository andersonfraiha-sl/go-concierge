package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"

	"go-concierge/internal/admin"
)

// PUT /integracao/v1/mapeamento: o Nexus cria e atualiza administradoras, condomínios,
// habilitações, orientações e payloads. Cada item grava pelo mesmo repositório que o Studio
// usa (s.admin), então o cache de mapeamento do turno recebe a escrita pelo mesmo caminho
// (Postgres.aoAlterar, ligado por admin.NovoCache).
//
// O lote não é uma transação: cada item é uma transação própria, aplicada na ordem do desenho
// até o primeiro erro. Antes do primeiro item o lote inteiro é ensaiado (forma, recorte e as
// regras do repositório contra uma cópia em memória), então o erro previsível nunca deixa o
// lote pela metade. Os itens são upserts: reenviar o mesmo lote não muda nada.
const (
	padraoMapeamento       = "PUT /integracao/v1/mapeamento"
	tetoDoLoteDeMapeamento = 1 << 20
	// cada item é uma transação serializável que relê o mapeamento inteiro
	maximoDeItensDoLote = 500
)

type loteDeMapeamento struct {
	Administradoras []admin.Administradora                  `json:"administradoras"`
	Condominios     []admin.Condominio                      `json:"condominios"`
	Habilitacoes    map[string]map[string]admin.Habilitacao `json:"habilitacoes"`
	Orientacoes     map[string]admin.Orientacao             `json:"orientacoes"`
	// Payloads: condomínio → recurso → definições; null apaga o payload do recurso.
	Payloads map[string]map[string]map[string]any `json:"payloads"`
	Remover  remocoesDoLote                       `json:"remover"`
}

type remocoesDoLote struct {
	Condominios     []string            `json:"condominios"`
	Administradoras []string            `json:"administradoras"`
	Habilitacoes    map[string][]string `json:"habilitacoes"`
}

func (s *servidor) rotasIntegracaoMapeamento(mux *http.ServeMux) {
	mux.HandleFunc(padraoMapeamento, s.comChaveDeIntegracao(s.integracaoMapeamento))
}

func (s *servidor) integracaoMapeamento(w http.ResponseWriter, r *http.Request) {
	adm, err := s.administradoraDaExportacao(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	var lote loteDeMapeamento
	if err := decodificarEstrito(w, r, &lote, tetoDoLoteDeMapeamento); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	atual, err := s.admin.Ler(r.Context())
	if err != nil {
		s.responderErroInterno(w, r, "ler mapeamento", err)
		return
	}
	passos, err := prepararLote(r.Context(), lote, atual, adm)
	if err != nil {
		s.responderFalhaDoLote(w, r, 0, err)
		return
	}
	aplicados, falha := aplicarPassos(r.Context(), s.admin, passos)
	if falha != nil {
		s.responderFalhaDoLote(w, r, aplicados, falha)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"aplicados": aplicados})
}

// decodificarEstrito recusa campo desconhecido: num PUT de sincronização, um campo com
// erro de digitação seria um pedaço do lote ignorado em silêncio.
func decodificarEstrito(w http.ResponseWriter, r *http.Request, destino any, teto int64) error {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, teto))
	d.DisallowUnknownFields()
	return d.Decode(destino)
}

// ---- validação do lote (sem I/O) -------------------------------------------

// prepararLote confere forma e recorte, monta os passos e ensaia todos antes de devolvê-los. O
// erro é um erroDeAlvo (nada foi pedido de válido) ou o *falhaNoLote do ensaio.
func prepararLote(ctx context.Context, l loteDeMapeamento, atual admin.Mapeamento, adm string) ([]passoDoLote, error) {
	if err := conferirForma(l); err != nil {
		return nil, err
	}
	if adm != "" {
		if err := conferirRecorte(l, atual, adm); err != nil {
			return nil, err
		}
	}
	passos := l.passos(atual)
	if len(passos) > maximoDeItensDoLote {
		return nil, erroDeAlvo{http.StatusBadRequest, "LOTE_GRANDE_DEMAIS", fmt.Sprintf("lote com %d itens; o máximo é %d", len(passos), maximoDeItensDoLote)}
	}
	if err := ensaiar(ctx, passos, atual); err != nil {
		return nil, err
	}
	return passos, nil
}

func loteInvalido(formato string, args ...any) error {
	return erroDeAlvo{http.StatusBadRequest, "LOTE_INVALIDO", fmt.Sprintf(formato, args...)}
}

func conferirForma(l loteDeMapeamento) error {
	var administradoras, condominios []string
	for _, a := range l.Administradoras {
		administradoras = append(administradoras, normalizarID(a.ID))
	}
	for _, c := range l.Condominios {
		condominios = append(condominios, normalizarID(c.ID))
	}
	if id, repetido := primeiroRepetido(administradoras); repetido {
		return loteInvalido("administradora %q repetida no lote", id)
	}
	if id, repetido := primeiroRepetido(condominios); repetido {
		return loteInvalido("condomínio %q repetido no lote", id)
	}
	return nil
}

func primeiroRepetido(ids []string) (string, bool) {
	vistos := make(map[string]bool, len(ids))
	for _, id := range ids {
		if vistos[id] {
			return id, true
		}
		vistos[id] = true
	}
	return "", false
}

func foraDoRecorte(formato string, args ...any) error {
	return erroDeAlvo{http.StatusForbidden, "FORA_DO_RECORTE", fmt.Sprintf(formato, args...)}
}

// conferirRecorte: quem chama com recorte (?administradora= ou ADMINISTRADORA_ID) só toca o
// que é dessa administradora. Nada do lote é gravado se um item sai do recorte.
func conferirRecorte(l loteDeMapeamento, atual admin.Mapeamento, adm string) error {
	if err := conferirAdministradoras(l, adm); err != nil {
		return err
	}
	if err := conferirCondominios(l.Condominios, atual, adm); err != nil {
		return err
	}
	donos := donosProjetados(l, atual)
	if err := conferirDonoDasEscritas(l, donos, adm); err != nil {
		return err
	}
	return conferirDonoDasRemocoes(l.Remover, donos, adm)
}

func conferirAdministradoras(l loteDeMapeamento, adm string) error {
	ids := slices.Clone(l.Remover.Administradoras)
	for _, a := range l.Administradoras {
		ids = append(ids, a.ID)
	}
	for _, id := range ids {
		if normalizarID(id) != adm {
			return foraDoRecorte("administradora %q fora do recorte %q", id, adm)
		}
	}
	return nil
}

// conferirCondominios: a dona do corpo e a atual têm de ser a do recorte (ninguém toma o
// condomínio de outra), e company e tenant, que decidem o roteamento das mensagens, só mudam
// pela Superlógica (a mesma regra do Studio).
func conferirCondominios(condominios []admin.Condominio, atual admin.Mapeamento, adm string) error {
	for _, c := range condominios {
		id := normalizarID(c.ID)
		if strings.TrimSpace(c.Administradora) != adm {
			return foraDoRecorte("condomínio %q não é da administradora %q", id, adm)
		}
		existente, existe := atual.Condominio(id)
		if existe && existente.Administradora != adm {
			return foraDoRecorte("condomínio %q não é da administradora %q", id, adm)
		}
		if c.CompanyID != existente.CompanyID || c.Tenant != existente.Tenant {
			return foraDoRecorte("condomínio %q: company_id e tenant só a Superlógica grava", id)
		}
	}
	return nil
}

// conferirDonoDasEscritas: habilitação, orientação e payload só de condomínio da administradora
// do recorte; o padrão "*" e o condomínio que não existe ficam de fora.
func conferirDonoDasEscritas(l loteDeMapeamento, donos map[string]string, adm string) error {
	alvos := slices.Concat(chavesNormalizadas(l.Habilitacoes), chavesNormalizadas(l.Orientacoes), chavesNormalizadas(l.Payloads))
	for _, id := range alvos {
		if donos[id] != adm {
			return foraDoRecorte("condomínio %q não é da administradora %q", id, adm)
		}
	}
	return nil
}

// conferirDonoDasRemocoes: remover o que já não existe é permitido (o reenvio do lote tem de
// passar); remover o de outra administradora ou o padrão "*" não.
func conferirDonoDasRemocoes(r remocoesDoLote, donos map[string]string, adm string) error {
	alvos := slices.Concat(chavesNormalizadas(r.Habilitacoes), normalizados(r.Condominios))
	for _, id := range alvos {
		dono, existe := donos[id]
		if id == admin.Padrao || existe && dono != adm {
			return foraDoRecorte("condomínio %q não é da administradora %q", id, adm)
		}
	}
	return nil
}

func chavesNormalizadas[V any](m map[string]V) []string {
	return normalizados(slices.Collect(maps.Keys(m)))
}

func normalizados(ids []string) []string {
	return slices.Collect(func(yield func(string) bool) {
		for _, id := range ids {
			if !yield(normalizarID(id)) {
				return
			}
		}
	})
}

// donosProjetados: a dona de cada condomínio como ficará depois dos condomínios do lote.
func donosProjetados(l loteDeMapeamento, atual admin.Mapeamento) map[string]string {
	donos := make(map[string]string, len(atual.Condominios)+len(l.Condominios))
	for _, c := range atual.Condominios {
		donos[c.ID] = c.Administradora
	}
	for _, c := range l.Condominios {
		donos[normalizarID(c.ID)] = strings.TrimSpace(c.Administradora)
	}
	return donos
}

// ---- passos: o que o lote grava, na ordem do desenho -----------------------

// passoDoLote é uma escrita do lote. conflitaComRegra: a recusa do repositório é conflito
// (409) e não campo inválido (400), como remover administradora que ainda tem condomínio.
type passoDoLote struct {
	rotulo           string
	conflitaComRegra bool
	aplicar          func(ctx context.Context, repo admin.Repositorio) error
}

func passo(rotulo string, escrever func(context.Context, admin.Repositorio) (admin.Mapeamento, error)) passoDoLote {
	return passoDoLote{rotulo: rotulo, aplicar: func(ctx context.Context, repo admin.Repositorio) error {
		_, err := escrever(ctx, repo)
		return err
	}}
}

// passos: administradoras, condomínios, habilitações, orientações, payloads e remoções.
func (l loteDeMapeamento) passos(atual admin.Mapeamento) []passoDoLote {
	donos := donosProjetados(l, atual)
	passos := passosDeAdministradoras(l.Administradoras)
	passos = append(passos, passosDeCondominios(l.Condominios, atual)...)
	passos = append(passos, passosPorCondominio(l, donos)...)
	return append(passos, passosDeRemocao(l.Remover, donos)...)
}

func passosDeAdministradoras(administradoras []admin.Administradora) []passoDoLote {
	var passos []passoDoLote
	for _, a := range administradoras {
		passos = append(passos, passo("administradoras["+normalizarID(a.ID)+"]", func(ctx context.Context, repo admin.Repositorio) (admin.Mapeamento, error) {
			return repo.SalvarAdministradora(ctx, a)
		}))
	}
	return passos
}

// passosDeCondominios: edita o que existe e cria o que não existe, em vez de um upsert cego. A
// edição leva as chaves que a conferência viu e o repositório recusa se mudaram no meio; a
// criação recusa um id que apareceu no meio. Assim a conferência de recorte não perde a corrida.
func passosDeCondominios(condominios []admin.Condominio, atual admin.Mapeamento) []passoDoLote {
	var passos []passoDoLote
	for _, c := range condominios {
		id := normalizarID(c.ID)
		rotulo := "condominios[" + id + "]"
		existente, existe := atual.Condominio(id)
		if !existe {
			passos = append(passos, passo(rotulo, func(ctx context.Context, repo admin.Repositorio) (admin.Mapeamento, error) {
				return repo.CriarCondominio(ctx, c)
			}))
			continue
		}
		passos = append(passos, passo(rotulo, func(ctx context.Context, repo admin.Repositorio) (admin.Mapeamento, error) {
			return repo.EditarCondominio(ctx, c, existente.Chaves())
		}))
	}
	return passos
}

func alvoProjetado(donos map[string]string, condominio string) admin.Alvo {
	return admin.Alvo{Condominio: condominio, Dona: donos[condominio]}
}

func passosPorCondominio(l loteDeMapeamento, donos map[string]string) []passoDoLote {
	var passos []passoDoLote
	for _, chave := range slices.Sorted(maps.Keys(l.Habilitacoes)) {
		for _, caso := range slices.Sorted(maps.Keys(l.Habilitacoes[chave])) {
			id, h := normalizarID(chave), l.Habilitacoes[chave][caso]
			passos = append(passos, passo(fmt.Sprintf("habilitacoes[%s][%s]", id, caso), func(ctx context.Context, repo admin.Repositorio) (admin.Mapeamento, error) {
				return repo.SalvarHabilitacao(ctx, alvoProjetado(donos, id), caso, h, false)
			}))
		}
	}
	for _, chave := range slices.Sorted(maps.Keys(l.Orientacoes)) {
		id, o := normalizarID(chave), l.Orientacoes[chave]
		passos = append(passos, passo("orientacoes["+id+"]", func(ctx context.Context, repo admin.Repositorio) (admin.Mapeamento, error) {
			return repo.SalvarOrientacao(ctx, alvoProjetado(donos, id), o)
		}))
	}
	for _, chave := range slices.Sorted(maps.Keys(l.Payloads)) {
		for _, recurso := range slices.Sorted(maps.Keys(l.Payloads[chave])) {
			id, payload := normalizarID(chave), l.Payloads[chave][recurso]
			passos = append(passos, passo(fmt.Sprintf("payloads[%s][%s]", id, recurso), func(ctx context.Context, repo admin.Repositorio) (admin.Mapeamento, error) {
				return repo.SalvarPayload(ctx, alvoProjetado(donos, id), recurso, payload)
			}))
		}
	}
	return passos
}

// passosDeRemocao: habilitações, depois condomínios, depois administradoras (a administradora
// só sai sem condomínio). Habilitação de condomínio que não existe mais já está removida.
func passosDeRemocao(r remocoesDoLote, donos map[string]string) []passoDoLote {
	var passos []passoDoLote
	for _, chave := range slices.Sorted(maps.Keys(r.Habilitacoes)) {
		id := normalizarID(chave)
		if _, existe := donos[id]; !existe && id != admin.Padrao {
			continue
		}
		for _, caso := range r.Habilitacoes[chave] {
			passos = append(passos, passo(fmt.Sprintf("remover.habilitacoes[%s][%s]", id, caso), func(ctx context.Context, repo admin.Repositorio) (admin.Mapeamento, error) {
				return repo.SalvarHabilitacao(ctx, alvoProjetado(donos, id), caso, admin.Habilitacao{}, true)
			}))
		}
	}
	for _, bruto := range r.Condominios {
		id := normalizarID(bruto)
		passos = append(passos, passo("remover.condominios["+id+"]", func(ctx context.Context, repo admin.Repositorio) (admin.Mapeamento, error) {
			return repo.RemoverCondominio(ctx, alvoProjetado(donos, id))
		}))
	}
	for _, bruto := range r.Administradoras {
		id := normalizarID(bruto)
		p := passo("remover.administradoras["+id+"]", func(ctx context.Context, repo admin.Repositorio) (admin.Mapeamento, error) {
			return repo.RemoverAdministradora(ctx, id)
		})
		p.conflitaComRegra = true
		passos = append(passos, p)
	}
	return passos
}

// ---- aplicação -------------------------------------------------------------

// falhaNoLote é o passo que o repositório recusou.
type falhaNoLote struct {
	rotulo           string
	conflitaComRegra bool
	causa            error
}

func (f *falhaNoLote) Error() string { return f.rotulo + ": " + f.causa.Error() }
func (f *falhaNoLote) Unwrap() error { return f.causa }

// aplicarPassos grava na ordem e para no primeiro erro; devolve quantos passos foram aplicados.
func aplicarPassos(ctx context.Context, repo admin.Repositorio, passos []passoDoLote) (int, *falhaNoLote) {
	for i, p := range passos {
		if err := p.aplicar(ctx, repo); err != nil {
			return i, &falhaNoLote{rotulo: p.rotulo, conflitaComRegra: p.conflitaComRegra, causa: err}
		}
	}
	return len(passos), nil
}

// ensaiar roda os passos contra uma cópia do mapeamento atual, com as mesmas regras do
// repositório real, sem gravar nada.
func ensaiar(ctx context.Context, passos []passoDoLote, atual admin.Mapeamento) error {
	copia, err := admin.NovaEmMemoriaCom(atual)
	if err != nil {
		return fmt.Errorf("preparar o ensaio do lote: %w", err)
	}
	if _, falha := aplicarPassos(ctx, copia, passos); falha != nil {
		return falha
	}
	return nil
}

// responderFalhaDoLote traduz o erro como o Studio traduz o das escritas avulsas e diz quantos
// passos já estavam aplicados e qual falhou, para o reenvio ser uma decisão informada.
func (s *servidor) responderFalhaDoLote(w http.ResponseWriter, r *http.Request, aplicados int, err error) {
	if _, ehDeAlvo := errors.AsType[erroDeAlvo](err); ehDeAlvo {
		responderErroDeAlvo(w, r, err)
		return
	}
	falha, ok := errors.AsType[*falhaNoLote](err)
	if !ok {
		s.responderErroInterno(w, r, "aplicar lote de mapeamento", err)
		return
	}
	status, codigo, mensagem := classificarFalhaDoLote(falha)
	switch {
	case status == http.StatusServiceUnavailable:
		s.logger().WarnContext(r.Context(), "conflito ao gravar o lote de mapeamento", "operacao", operacaoDe(r), "passo", falha.rotulo, "erro", falha.Error())
	case status >= http.StatusInternalServerError:
		anotarFalha(r.Context(), w, fmt.Errorf("aplicar lote de mapeamento: %w", falha), 1)
	}
	writeJSON(w, status, map[string]any{"codigo": codigo, "error": mensagem, "aplicados": aplicados, "falhou": falha.rotulo})
}

func classificarFalhaDoLote(f *falhaNoLote) (status int, codigo, mensagem string) {
	switch {
	case errors.Is(f.causa, admin.ErrJaExiste):
		return http.StatusConflict, "JA_EXISTE", f.causa.Error()
	case errors.Is(f.causa, admin.ErrNaoEncontrado):
		return http.StatusConflict, "NAO_ENCONTRADO", f.causa.Error()
	case errors.Is(f.causa, admin.ErrInvalido) && f.conflitaComRegra:
		return http.StatusConflict, "CONFLITO_DE_REGRA", f.causa.Error()
	case errors.Is(f.causa, admin.ErrInvalido):
		return http.StatusBadRequest, "LOTE_INVALIDO", f.causa.Error()
	case errors.Is(f.causa, admin.ErrConflitoConcorrente):
		return http.StatusServiceUnavailable, codigoConflitoConcorrente, "outra alteração do mapeamento aconteceu ao mesmo tempo; reenvie o lote"
	default:
		return http.StatusInternalServerError, "ERRO_INTERNO", "erro interno; reenvie o lote"
	}
}
