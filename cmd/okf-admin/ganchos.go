package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/google/uuid"

	"go-concierge/internal/acesso"
	"go-concierge/internal/ganchos"
	"go-concierge/internal/ganchos/auditoria"
	"go-concierge/internal/ganchos/fixos"
	"go-concierge/internal/ganchos/regras"
)

// API das regras dos ganchos. Editar é só da Superlógica (ganchos.editar); o histórico de
// uma versão vale no escopo do alvo daquela versão; importar numa implantação dedicada
// (ADMINISTRADORA_ID) pede studio.avancado na administradora fixa.

const (
	limiteCorpoRegra  = 64 << 10
	limitePadrao      = 50
	limiteMaximo      = 200 // o teto de Postgres.Listar
	janelaDosDisparos = 7 * 24 * time.Hour
)

func (s *servidor) rotasGanchos(mux *http.ServeMux) {
	editar := func(h http.HandlerFunc) http.Handler { return s.protegida(acesso.GanchosEditar, alvoSuperlogica, h) }
	mux.Handle("GET /api/ganchos/fixos", editar(s.ganchosFixos))
	mux.Handle("GET /api/ganchos/regras", editar(s.ganchosListar))
	mux.Handle("GET /api/ganchos/regras/{id}", editar(s.ganchosLer))
	mux.Handle("POST /api/ganchos/regras", editar(s.ganchosCriar))
	mux.Handle("PUT /api/ganchos/regras/{id}", editar(s.ganchosAlterar))
	mux.Handle("GET /api/ganchos/regras/{id}/versoes/{versao}", s.autenticado(http.HandlerFunc(s.ganchosVersao)))
	mux.Handle("GET /api/ganchos/regras/exportar", editar(s.ganchosExportar))
	mux.Handle("POST /api/ganchos/regras/importar", s.importarProtegido())
	mux.Handle("GET /api/ganchos/historico", s.protegidaEmAlgum(acesso.GanchosHistorico, s.ganchosHistorico))
}

func (s *servidor) importarProtegido() http.Handler {
	fixa := s.implantacao.AdministradoraFixa
	if fixa == "" {
		return s.protegida(acesso.GanchosEditar, alvoSuperlogica, s.ganchosImportar)
	}
	alvo := func(*http.Request) (acesso.Escopo, error) { return escopoAdministradora(fixa), nil }
	return s.protegida(acesso.StudioAvancado, alvo, s.ganchosImportar)
}

// ---- respostas ---------------------------------------------------------------

// recusa é o pedido que não segue: vira a resposta com status, código e, na validação, os
// problemas.
type recusa struct {
	status           int
	codigo, mensagem string
	problemas        []problemaJSON
}

func (e recusa) Error() string { return e.mensagem }

type problemaJSON struct {
	Regra    *int   `json:"regra,omitempty"` // índice no arquivo importado
	Campo    string `json:"campo"`
	Codigo   string `json:"codigo"`
	Mensagem string `json:"mensagem"`
}

func problemasJSON(ps []regras.Problema, indice *int) []problemaJSON {
	out := make([]problemaJSON, len(ps))
	for i, p := range ps {
		out[i] = problemaJSON{Regra: indice, Campo: p.Campo, Codigo: p.Codigo, Mensagem: p.Mensagem}
	}
	return out
}

func regraInvalida(ps []problemaJSON) recusa {
	return recusa{http.StatusUnprocessableEntity, "REGRA_INVALIDA", "regra inválida", ps}
}

// responderGanchos traduz o erro de uma rota dos ganchos; o que não é esperado vira 500.
func (s *servidor) responderGanchos(w http.ResponseWriter, r *http.Request, operacao string, err error) {
	if e, ok := errors.AsType[recusa](err); ok {
		corpo := map[string]any{"codigo": e.codigo, "error": e.mensagem}
		if len(e.problemas) > 0 {
			corpo["problemas"] = e.problemas
		}
		writeJSON(w, e.status, corpo)
		return
	}
	switch {
	case errors.Is(err, regras.ErrVersaoVelha):
		writeJSON(w, http.StatusConflict, map[string]string{"codigo": "VERSAO_VELHA", "error": "a regra foi alterada por outra pessoa; recarregue"})
	case errors.Is(err, regras.ErrRegraDesconhecida):
		writeJSON(w, http.StatusNotFound, map[string]string{"codigo": "REGRA_DESCONHECIDA", "error": "regra não encontrada"})
	default:
		s.responderErroInterno(w, r, operacao, err)
	}
}

// lerCorpoBruto lê até limite bytes; acima disso, 413 com o código dado.
func lerCorpoBruto(w http.ResponseWriter, r *http.Request, limite int64, codigoGrande string) ([]byte, error) {
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, limite))
	if _, grande := errors.AsType[*http.MaxBytesError](err); grande {
		return nil, recusa{status: http.StatusRequestEntityTooLarge, codigo: codigoGrande, mensagem: "corpo grande demais"}
	}
	if err != nil {
		return nil, recusa{status: http.StatusBadRequest, codigo: "CORPO_INVALIDO", mensagem: "corpo ilegível"}
	}
	return raw, nil
}

func usuarioDe(r *http.Request) string {
	a, _ := acessoDe(r.Context())
	return a.UsuarioID
}

// ---- visão das regras --------------------------------------------------------

type regraVisao struct {
	ID          string               `json:"id"`
	Versao      int                  `json:"versao"`
	Nome        string               `json:"nome"`
	Evento      ganchos.NomeDoEvento `json:"evento"`
	Ativa       bool                 `json:"ativa"`
	Definicao   json.RawMessage      `json:"definicao"`
	AlteradaPor string               `json:"alterada_por"`
	AlteradaEm  time.Time            `json:"alterada_em"`
}

type regraNaLista struct {
	regraVisao
	Disparos7d int `json:"disparos_7d"`
}

func visaoDe(g regras.RegraGravada) regraVisao {
	return regraVisao{ID: g.ID, Versao: g.Versao, Nome: g.Nome, Evento: g.Evento, Ativa: g.Ativa,
		Definicao: g.Definicao, AlteradaPor: g.AlteradaPor, AlteradaEm: g.AlteradaEm}
}

// ---- leitura -----------------------------------------------------------------

func (s *servidor) ganchosFixos(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"fixos": nomesDosFixos()})
}

// nomesDosFixos: por evento, os fixos de checagem e depois os de saneamento.
func nomesDosFixos() map[ganchos.NomeDoEvento][]string {
	nomes := ganchos.NovoConjunto(fixos.Checagem(), nil).NomesDosFixos()
	for ev, ns := range ganchos.NovoConjunto(fixos.Saneamento(), nil).NomesDosFixos() {
		nomes[ev] = append(nomes[ev], ns...)
	}
	return nomes
}

func (s *servidor) ganchosLer(w http.ResponseWriter, r *http.Request) {
	g, err := s.regras.Ler(r.Context(), r.PathValue("id"))
	if err != nil {
		s.responderGanchos(w, r, "ler regra", err)
		return
	}
	writeJSON(w, http.StatusOK, visaoDe(g))
}

type paginaDeRegras struct {
	Regras  []regraNaLista `json:"regras"`
	Proximo string         `json:"proximo,omitempty"` // id para ?depois= quando a página veio cheia
}

func (s *servidor) ganchosListar(w http.ResponseWriter, r *http.Request) {
	depois, limite, err := paginaPedida(r)
	if err != nil {
		s.responderGanchos(w, r, "listar regras", err)
		return
	}
	pagina, err := s.regras.Listar(r.Context(), depois, limite)
	if err != nil {
		s.responderGanchos(w, r, "listar regras", err)
		return
	}
	disparos, err := auditoria.DisparosPorRegra(r.Context(), s.banco.Ent, time.Now().Add(-janelaDosDisparos), idsDe(pagina))
	if err != nil {
		s.responderGanchos(w, r, "contar disparos", err)
		return
	}
	writeJSON(w, http.StatusOK, montarPagina(pagina, disparos, limite))
}

func montarPagina(pagina []regras.RegraGravada, disparos map[string]int, limite int) paginaDeRegras {
	out := paginaDeRegras{Regras: make([]regraNaLista, len(pagina))}
	for i, g := range pagina {
		out.Regras[i] = regraNaLista{visaoDe(g), disparos[g.ID]}
	}
	if len(pagina) == limite {
		out.Proximo = pagina[len(pagina)-1].ID
	}
	return out
}

func paginaPedida(r *http.Request) (depois string, limite int, err error) {
	q := r.URL.Query()
	limite = limitePadrao
	if bruto := q.Get("limite"); bruto != "" {
		limite, err = strconv.Atoi(bruto)
		if err != nil || limite < 1 || limite > limiteMaximo {
			return "", 0, recusa{status: http.StatusBadRequest, codigo: "LIMITE_INVALIDO", mensagem: "limite vai de 1 a 200"}
		}
	}
	depois = q.Get("depois")
	if depois != "" && uuid.Validate(depois) != nil {
		return "", 0, recusa{status: http.StatusBadRequest, codigo: "CURSOR_INVALIDO", mensagem: "depois precisa ser o id de uma regra"}
	}
	return depois, limite, nil
}

func idsDe(rs []regras.RegraGravada) []string {
	ids := make([]string, len(rs))
	for i, g := range rs {
		ids[i] = g.ID
	}
	return ids
}

// ganchosVersao devolve uma versão gravada a quem tem ganchos.editar ou ganchos.historico no
// alvo daquela versão (o da definição gravada, não o atual da regra).
func (s *servidor) ganchosVersao(w http.ResponseWriter, r *http.Request) {
	a, _ := acessoDe(r.Context())
	if !acesso.PermitidoEmAlgum(a, acesso.GanchosEditar) && !acesso.PermitidoEmAlgum(a, acesso.GanchosHistorico) {
		responderSemPermissao(w)
		return
	}
	v, regra, err := s.lerVersao(r)
	if err != nil {
		s.responderGanchos(w, r, "ler versão da regra", err)
		return
	}
	if !podeVerAlvo(r, s.escoposDoAlvo(regra.Alvo)) {
		responderSemPermissao(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"regra_id": v.RegraID, "versao": v.Versao, "definicao": v.Definicao,
		"alterada_por": v.AlteradaPor, "alterada_em": v.AlteradaEm,
	})
}

func (s *servidor) lerVersao(r *http.Request) (regras.VersaoGravada, regras.Regra, error) {
	versao, err := strconv.Atoi(r.PathValue("versao"))
	if err != nil || versao < 1 {
		return regras.VersaoGravada{}, regras.Regra{}, recusa{status: http.StatusBadRequest, codigo: "VERSAO_INVALIDA", mensagem: "versão é um número a partir de 1"}
	}
	v, err := s.regras.LerVersao(r.Context(), r.PathValue("id"), versao)
	if err != nil {
		return regras.VersaoGravada{}, regras.Regra{}, err
	}
	regra, err := regras.Decodificar(v.Definicao)
	if err != nil {
		return regras.VersaoGravada{}, regras.Regra{}, err
	}
	return v, regra, nil
}

// podeVerAlvo exige ganchos.editar ou ganchos.historico em cada escopo do alvo.
func podeVerAlvo(r *http.Request, escopos []acesso.Escopo) bool {
	return !slices.ContainsFunc(escopos, func(e acesso.Escopo) bool {
		return !permitidoAqui(r, acesso.GanchosEditar, e) && !permitidoAqui(r, acesso.GanchosHistorico, e)
	})
}

// escoposDoAlvo: cada condomínio do alvo, ou a administradora, ou, no alvo de todos, a
// Superlógica. Numa implantação dedicada o alvo de todos só alcança a administradora fixa
// (a importação aceita regra global ali), então vale o escopo dela.
func (s *servidor) escoposDoAlvo(alvo regras.Alvo) []acesso.Escopo {
	if len(alvo.Condominios) > 0 {
		escopos := make([]acesso.Escopo, len(alvo.Condominios))
		for i, c := range alvo.Condominios {
			escopos[i] = escopoCondominio(c)
		}
		return escopos
	}
	if alvo.Administradora != "" {
		return []acesso.Escopo{escopoAdministradora(alvo.Administradora)}
	}
	if fixa := s.implantacao.AdministradoraFixa; fixa != "" {
		return []acesso.Escopo{escopoAdministradora(fixa)}
	}
	return []acesso.Escopo{escopoSuperlogica}
}

// ---- gravação ----------------------------------------------------------------

func (s *servidor) ganchosCriar(w http.ResponseWriter, r *http.Request) {
	regra, definicao, err := lerRegra(w, r)
	if err != nil {
		s.responderGanchos(w, r, "criar regra", err)
		return
	}
	regra.ID = "" // regra nova ganha id do banco
	salva, err := s.gravarRegra(r.Context(), regra, definicao, 0, usuarioDe(r))
	if err != nil {
		s.responderGanchos(w, r, "criar regra", err)
		return
	}
	writeJSON(w, http.StatusCreated, visaoDe(salva))
}

// ganchosAlterar exige no corpo a versao lida: quem gravou depois dela ganha (409).
func (s *servidor) ganchosAlterar(w http.ResponseWriter, r *http.Request) {
	regra, definicao, err := lerRegra(w, r)
	if err == nil {
		err = conferirAlteracao(regra, r.PathValue("id"))
	}
	if err != nil {
		s.responderGanchos(w, r, "alterar regra", err)
		return
	}
	regra.ID = r.PathValue("id")
	salva, err := s.gravarRegra(r.Context(), regra, definicao, regra.Versao, usuarioDe(r))
	if err != nil {
		s.responderGanchos(w, r, "alterar regra", err)
		return
	}
	writeJSON(w, http.StatusOK, visaoDe(salva))
}

func conferirAlteracao(regra regras.Regra, id string) error {
	if regra.Versao < 1 {
		return recusa{status: http.StatusUnprocessableEntity, codigo: "VERSAO_OBRIGATORIA", mensagem: "informe a versao que você leu"}
	}
	if regra.ID != "" && regra.ID != id {
		return recusa{status: http.StatusUnprocessableEntity, codigo: "ID_DIVERGENTE", mensagem: "o id do corpo não é o da rota"}
	}
	return nil
}

// lerRegra decodifica o corpo (estrito: alterada_por e outros campos fora do esquema são
// recusados) e devolve a definição a gravar.
func lerRegra(w http.ResponseWriter, r *http.Request) (regras.Regra, json.RawMessage, error) {
	raw, err := lerCorpoBruto(w, r, limiteCorpoRegra, "CORPO_GRANDE_DEMAIS")
	if err != nil {
		return regras.Regra{}, nil, err
	}
	return decodificarRegra(raw, nil)
}

func decodificarRegra(raw []byte, indice *int) (regras.Regra, json.RawMessage, error) {
	regra, err := regras.Decodificar(raw)
	if err != nil {
		return regras.Regra{}, nil, regraInvalida(problemasJSON([]regras.Problema{problemaDeEsquema(err)}, indice))
	}
	definicao, err := definicaoDe(raw)
	if err != nil {
		return regras.Regra{}, nil, err
	}
	return regra, definicao, nil
}

func problemaDeEsquema(err error) regras.Problema {
	if e, ok := errors.AsType[*regras.ErroDeEsquema](err); ok {
		return regras.Problema{Campo: e.Campo, Codigo: e.Codigo, Mensagem: e.Mensagem}
	}
	return regras.Problema{Codigo: "JSON_INVALIDO", Mensagem: err.Error()}
}

// definicaoDe é o JSON da regra sem id e versao, que vivem nas colunas.
func definicaoDe(raw []byte) (json.RawMessage, error) {
	return comCampos(raw, func(campos map[string]json.RawMessage) {
		delete(campos, "id")
		delete(campos, "versao")
	})
}

func comCampos(raw []byte, mudar func(map[string]json.RawMessage)) (json.RawMessage, error) {
	var campos map[string]json.RawMessage
	if err := json.Unmarshal(raw, &campos); err != nil {
		return nil, err
	}
	mudar(campos)
	return json.Marshal(campos)
}

func (s *servidor) gravarRegra(ctx context.Context, regra regras.Regra, definicao json.RawMessage, versaoLida int, autor string) (regras.RegraGravada, error) {
	cat, ativas, err := s.contextoDaValidacao(ctx)
	if err != nil {
		return regras.RegraGravada{}, err
	}
	ps := append(regras.ValidarRegra(regra, cat), regras.ValidarLimites(ativas, regra, cat)...)
	if len(ps) > 0 {
		return regras.RegraGravada{}, regraInvalida(problemasJSON(ps, nil))
	}
	return s.regras.Salvar(ctx, gravadaDe(regra, definicao, autor), versaoLida)
}

// contextoDaValidacao: o catálogo e as ativas de agora, para os limites.
// ponytail: lê as ativas a cada gravação (até 10 mil, spec §7); só o caminho de escrita paga.
func (s *servidor) contextoDaValidacao(ctx context.Context) (regras.Catalogo, []regras.Regra, error) {
	cat, err := s.catalogoDosGanchos(ctx)
	if err != nil {
		return regras.Catalogo{}, nil, err
	}
	gravadas, err := s.regras.Ativas(ctx)
	if err != nil {
		return regras.Catalogo{}, nil, err
	}
	ativas := make([]regras.Regra, len(gravadas))
	for i, g := range gravadas {
		ativas[i] = paraLimites(g)
	}
	return cat, ativas, nil
}

// paraLimites: os limites só olham id, evento, alvo e ativa, que as colunas já trazem.
func paraLimites(g regras.RegraGravada) regras.Regra {
	return regras.Regra{ID: g.ID, Evento: g.Evento, Alvo: regras.Alvo{Administradora: g.AdministradoraID, Condominios: g.Condominios}, Ativa: g.Ativa}
}

func gravadaDe(regra regras.Regra, definicao json.RawMessage, autor string) regras.RegraGravada {
	return regras.RegraGravada{
		ID: regra.ID, Nome: regra.Nome, Evento: regra.Evento, AdministradoraID: regra.Alvo.Administradora,
		Condominios: append([]string{}, regra.Alvo.Condominios...), Ativa: regra.Ativa,
		Definicao: definicao, AlteradaPor: autor,
	}
}

// catalogoDosGanchos monta o que a validação consulta: ferramentas do catálogo de recursos,
// casos de uso do concierge, donas dos condomínios e nomes dos fixos.
func (s *servidor) catalogoDosGanchos(ctx context.Context) (regras.Catalogo, error) {
	m, err := s.admin.Ler(ctx)
	if err != nil {
		return regras.Catalogo{}, err
	}
	casos, err := s.casosDeUso()
	if err != nil {
		return regras.Catalogo{}, err
	}
	cat := regras.Catalogo{Ferramentas: map[string]bool{}, Casos: map[string]bool{}, AdministradoraDe: map[string]string{},
		NomesDosFixos: slices.Concat(slices.Collect(maps.Values(nomesDosFixos()))...)}
	for _, r := range s.cat.RecursosNexus() {
		cat.Ferramentas[r.Name] = true
	}
	for _, c := range casos {
		cat.Casos[c.Nome] = true // a pasta, chave da habilitação; o id do caso.yaml também vale
		if c.ID != "" {
			cat.Casos[c.ID] = true
		}
	}
	for _, c := range m.Condominios {
		cat.AdministradoraDe[c.ID] = c.Administradora
	}
	return cat, nil
}
