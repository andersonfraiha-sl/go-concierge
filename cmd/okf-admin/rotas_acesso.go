package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"

	"go-concierge/internal/acesso"
	"go-concierge/internal/admin"
	"go-concierge/internal/escopo"
)

// Acesso às rotas: cada rota da API passa por autenticado (quem é e o que pode) e por exigir
// (a permissão no escopo do alvo da requisição). Ficam de fora o webhook do ATENDE (HMAC),
// o healthz, os arquivos estáticos e as páginas HTML.

// padraoEstaticos é a rota dos arquivos da interface (sem log por requisição).
const padraoEstaticos = "GET /"

// rotear monta o mux inteiro com o log por requisição. mcp é o repasse ao servidor MCP
// (nil quando não há MCP configurado).
func (s *servidor) rotear(mcp http.Handler) http.Handler {
	mux := http.NewServeMux()
	s.rotas(mux)
	s.rotasAtende(mux)
	s.rotasAdmin(mux)
	s.rotasGanchos(mux)
	s.rotasStudio(mux)
	s.rotasCopiloto(mux)
	if mcp != nil {
		// por método: um "/mcp/" sem método conflitaria com o "GET /" da interface
		protegido := s.protegida(acesso.StudioAvancado, alvoSuperlogica, mcp.ServeHTTP)
		for _, m := range []string{"GET", "POST", "DELETE", "OPTIONS"} {
			mux.Handle(m+" /mcp/", protegido)
		}
	}
	return s.registrar(comCabecalhosDeSeguranca(mux))
}

// alvoDe resolve o escopo sobre o qual a requisição age.
type alvoDe func(*http.Request) (acesso.Escopo, error)

// exigir responde 403 se o usuário do contexto não tem p no escopo do alvo.
func exigir(p acesso.Permissao, alvo alvoDe, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a, ok := acessoDe(r.Context())
		if !ok {
			responderNaoAutenticado(w)
			return
		}
		e, err := alvo(r)
		if err != nil {
			responderErroDeAlvo(w, r, err)
			return
		}
		if !acesso.Permitido(a, p, e) {
			responderSemPermissao(w)
			return
		}
		h(w, r)
	}
}

// exigirEmAlgum serve às leituras globais (catálogo, listas): basta ter p em algum escopo.
func exigirEmAlgum(p acesso.Permissao, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a, ok := acessoDe(r.Context())
		if !ok {
			responderNaoAutenticado(w)
			return
		}
		if !acesso.PermitidoEmAlgum(a, p) {
			responderSemPermissao(w)
			return
		}
		h(w, r)
	}
}

func (s *servidor) protegida(p acesso.Permissao, alvo alvoDe, h http.HandlerFunc) http.Handler {
	return s.autenticado(exigir(p, alvo, h))
}

func (s *servidor) protegidaEmAlgum(p acesso.Permissao, h http.HandlerFunc) http.Handler {
	return s.autenticado(exigirEmAlgum(p, h))
}

// permitidoAqui confere uma permissão a mais dentro do handler (quando a rota tem dois alvos).
func permitidoAqui(r *http.Request, p acesso.Permissao, e acesso.Escopo) bool {
	a, ok := acessoDe(r.Context())
	return ok && acesso.Permitido(a, p, e)
}

// ---- erros na resolução do alvo -------------------------------------------

// erroDeAlvo é o pedido que não diz sobre o que age: vira a resposta com status e código.
type erroDeAlvo struct {
	status           int
	codigo, mensagem string
}

func (e erroDeAlvo) Error() string { return e.mensagem }

func responderErroDeAlvo(w http.ResponseWriter, r *http.Request, err error) {
	if e, ok := errors.AsType[erroDeAlvo](err); ok {
		writeJSON(w, e.status, map[string]string{"codigo": e.codigo, "error": e.mensagem})
		return
	}
	loggerDe(r.Context()).ErrorContext(r.Context(), "resolver alvo da requisição", "operacao", operacaoDe(r), "erro", err.Error())
	writeJSON(w, http.StatusInternalServerError, map[string]string{"codigo": "ERRO_INTERNO", "error": "erro interno; tente de novo"})
}

// ---- alvos -----------------------------------------------------------------

var escopoSuperlogica = acesso.Escopo{Tipo: acesso.TipoSuperlogica}

func alvoSuperlogica(*http.Request) (acesso.Escopo, error) { return escopoSuperlogica, nil }

func escopoAdministradora(id string) acesso.Escopo {
	return acesso.Escopo{Tipo: acesso.TipoAdministradora, ID: id}
}

// escopoCondominio: o padrão "*" (ou vazio) é da Superlógica; um condomínio vale pela dona
// atual dele, que o Acesso já traz.
func escopoCondominio(id string) acesso.Escopo {
	if id == "" || id == admin.Padrao {
		return escopoSuperlogica
	}
	return acesso.Escopo{Tipo: acesso.TipoCondominio, ID: id}
}

// administradoraPedida: a do parâmetro ?administradora=, ou a fixa da implantação. Sem
// nenhuma ou fora do formato, 400; fora da implantação, 403.
func administradoraPedida(r *http.Request, imp escopo.Implantacao) (string, error) {
	adm := strings.TrimSpace(r.URL.Query().Get("administradora"))
	if adm == "" {
		adm = imp.AdministradoraFixa
	}
	if adm == "" {
		return "", erroDeAlvo{http.StatusBadRequest, "ADMINISTRADORA_OBRIGATORIA", "informe administradora"}
	}
	if !escopo.AdministradoraValida(adm) {
		return "", erroDeAlvo{http.StatusBadRequest, "ADMINISTRADORA_INVALIDA", "administradora com formato inválido"}
	}
	if !imp.Permite(adm) {
		return "", erroDeAlvo{http.StatusForbidden, "FORA_DA_IMPLANTACAO", "administradora fora desta implantação"}
	}
	return adm, nil
}

func (s *servidor) alvoAdministradora(r *http.Request) (acesso.Escopo, error) {
	adm, err := administradoraPedida(r, s.implantacao)
	if err != nil {
		return acesso.Escopo{}, err
	}
	return escopoAdministradora(adm), nil
}

// alvoAdministradoraOuSuperlogica: sem administradora pedida (e sem implantação fixa), a
// visão é a da Superlógica inteira.
func (s *servidor) alvoAdministradoraOuSuperlogica(r *http.Request) (acesso.Escopo, error) {
	if !s.temAdministradoraPedida(r) {
		return escopoSuperlogica, nil
	}
	return s.alvoAdministradora(r)
}

func (s *servidor) temAdministradoraPedida(r *http.Request) bool {
	return strings.TrimSpace(r.URL.Query().Get("administradora")) != "" || s.implantacao.AdministradoraFixa != ""
}

// escopoDaSessao: a administradora gravada na conversa quando ela resolveu o condomínio; ela
// não muda se o condomínio trocar de dona. Conversa sem administradora, ou que não existe,
// só a Superlógica vê (e não se revela a existência a quem não pode).
func (s *servidor) escopoDaSessao(ctx context.Context, identificador string) (acesso.Escopo, error) {
	sess, ok, err := s.atende.Sessao(ctx, identificador)
	if err != nil {
		return acesso.Escopo{}, fmt.Errorf("sessão %s: %w", identificador, err)
	}
	if !ok || sess.Administradora() == "" {
		return escopoSuperlogica, nil
	}
	return escopoAdministradora(sess.Administradora()), nil
}

func identificadorDaSessao(r *http.Request) string {
	q := r.URL.Query()
	if id := q.Get("id"); id != "" {
		return id
	}
	return q.Get("protocolo")
}

func (s *servidor) alvoSessaoDaConsulta(r *http.Request) (acesso.Escopo, error) {
	return s.escopoDaSessao(r.Context(), identificadorDaSessao(r))
}

type corpoComSessao struct {
	Acao      string `json:"acao"`
	Protocolo string `json:"protocolo"`
	SessaoID  string `json:"sessao_id"`
}

func (c corpoComSessao) identificador() string {
	if c.SessaoID != "" {
		return c.SessaoID
	}
	return c.Protocolo
}

func (s *servidor) alvoSessaoDoCorpo(r *http.Request) (acesso.Escopo, error) {
	var c corpoComSessao
	if err := espiarCorpo(r, &c); err != nil {
		return acesso.Escopo{}, err
	}
	return s.escopoDaSessao(r.Context(), c.identificador())
}

// alvoAcaoDePedido: preencher age sobre a conversa (o contato vem dela); cancelar e remover,
// sobre o contato da administradora pedida.
func (s *servidor) alvoAcaoDePedido(r *http.Request) (acesso.Escopo, error) {
	var c corpoComSessao
	if err := espiarCorpo(r, &c); err != nil {
		return acesso.Escopo{}, err
	}
	if c.Acao == "preencher" {
		return s.escopoDaSessao(r.Context(), c.identificador())
	}
	return s.alvoAdministradora(r)
}

func alvoCondominioDoCorpo(r *http.Request) (acesso.Escopo, error) {
	var c struct {
		Condominio string `json:"condominio"`
	}
	if err := espiarCorpo(r, &c); err != nil {
		return acesso.Escopo{}, err
	}
	return escopoCondominio(normalizarID(c.Condominio)), nil
}

func alvoCondominioDaConsulta(r *http.Request) (acesso.Escopo, error) {
	return escopoCondominio(normalizarID(r.URL.Query().Get("id"))), nil
}

// alvoDonaDoCorpo: a administradora que o corpo põe como dona do condomínio; sem dona, só a
// Superlógica.
func alvoDonaDoCorpo(r *http.Request) (acesso.Escopo, error) {
	var c struct {
		Administradora string `json:"administradora_id"`
	}
	if err := espiarCorpo(r, &c); err != nil {
		return acesso.Escopo{}, err
	}
	if adm := strings.TrimSpace(c.Administradora); adm != "" {
		return escopoAdministradora(adm), nil
	}
	return escopoSuperlogica, nil
}

// normalizarID segue a regra de id do mapeamento (minúsculas, sem espaços nas pontas), para o
// alvo conferido ser o mesmo que a escrita grava.
func normalizarID(id string) string { return strings.ToLower(strings.TrimSpace(id)) }

// limiteCorpoEspiado cobre os corpos JSON do Admin e do Console; o importar (zip) não é espiado.
const limiteCorpoEspiado = 2 << 20

// espiarCorpo decodifica o corpo JSON para resolver o alvo e o devolve intacto à requisição,
// para o handler ler de novo.
func espiarCorpo(r *http.Request, v any) error {
	raw, err := io.ReadAll(io.LimitReader(r.Body, limiteCorpoEspiado))
	if err != nil {
		return erroDeAlvo{http.StatusBadRequest, "CORPO_INVALIDO", "corpo ilegível"}
	}
	r.Body = io.NopCloser(bytes.NewReader(raw))
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return erroDeAlvo{http.StatusBadRequest, "CORPO_INVALIDO", "corpo JSON inválido"}
	}
	return nil
}

// ---- visão do mapeamento por usuário --------------------------------------

// recorteDoUsuario decide, antes de qualquer escrita, que administradora a resposta mostra:
// a Superlógica vê tudo ("") ou a pedida; os demais, a pedida entre as suas ou a única que
// têm. Com várias e nenhuma pedida, 400; pedida fora das suas, 403.
func recorteDoUsuario(r *http.Request, adms []admin.Administradora) (string, error) {
	a, ok := acessoDe(r.Context())
	if !ok {
		return "", erroDeAlvo{http.StatusUnauthorized, "NAO_AUTENTICADO", "usuário não autenticado"}
	}
	pedida := strings.TrimSpace(r.URL.Query().Get("administradora"))
	if acesso.Permitido(a, acesso.SkillsLer, escopoSuperlogica) {
		return pedida, nil
	}
	visiveis := administradorasVisiveis(a, adms)
	if pedida == "" && len(visiveis) == 1 {
		return visiveis[0].ID, nil
	}
	if pedida == "" && len(visiveis) > 1 {
		return "", erroDeAlvo{http.StatusBadRequest, "ADMINISTRADORA_OBRIGATORIA", "informe administradora"}
	}
	if !slices.ContainsFunc(visiveis, func(adm admin.Administradora) bool { return adm.ID == pedida }) {
		return "", erroDeAlvo{http.StatusForbidden, "SEM_PERMISSAO", "sem permissão para esta administradora"}
	}
	return pedida, nil
}

// visaoPara aplica o recorte já decidido; não falha, para uma escrita gravada nunca responder
// erro. Quem não é da Superlógica nunca recebe Visao("") (que é tudo).
func visaoPara(r *http.Request, m admin.Mapeamento, recorte string) (admin.Mapeamento, []admin.Administradora) {
	a, _ := acessoDe(r.Context())
	if acesso.Permitido(a, acesso.SkillsLer, escopoSuperlogica) {
		return m.Visao(recorte), m.Administradoras
	}
	if recorte == "" {
		return admin.Mapeamento{}, []admin.Administradora{}
	}
	return m.Visao(recorte), administradorasVisiveis(a, m.Administradoras)
}

func administradorasVisiveis(a acesso.Acesso, adms []admin.Administradora) []admin.Administradora {
	return slices.DeleteFunc(slices.Clone(adms), func(adm admin.Administradora) bool {
		return !acesso.Permitido(a, acesso.SkillsLer, escopoAdministradora(adm.ID))
	})
}

// alvoDaEscrita é o condomínio da escrita com a dona que a autorização viu (o repositório
// recusa se ela mudou no meio).
func alvoDaEscrita(r *http.Request, condoID string) admin.Alvo {
	a, _ := acessoDe(r.Context())
	return admin.Alvo{Condominio: condoID, Dona: a.AdministradoraDe[condoID]}
}
