package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"go-concierge/internal/acesso"
	"go-concierge/internal/admin"
	"go-concierge/internal/harness"
	"go-concierge/internal/okf"
)

// API do mapeamento administradoras → condomínios → skills (habilitação, papéis,
// orientações, ajustes), restrita aos condomínios da administradora do pedido
// (?administradora=<id>; RLS). O Studio é quem usa. Até o Studio cobrir tudo, ficam três
// telas antigas: /admin (matriz skills × condomínios), /backdoor/administradoras
// (renomear administradora) e /backdoor/payloads (definições livres por recurso).

func (s *servidor) rotasAdmin(mux *http.ServeMux) {
	mux.HandleFunc("GET /admin", s.telaAntiga("index.html"))
	mux.HandleFunc("GET /backdoor", s.telaAntiga("backdoor.html"))
	mux.HandleFunc("GET /backdoor/orquestracao", s.telaAntiga("orquestracao.html"))
	mux.HandleFunc("GET /backdoor/payloads", s.telaAntiga("payloads.html"))
	mux.HandleFunc("GET /backdoor/administradoras", s.telaAntiga("administradoras.html"))
	mux.Handle("GET /api/admin/orquestracao", s.protegidaEmAlgum(acesso.StudioAvancado, s.adminOrquestracao))
	mux.Handle("PUT /api/admin/orquestracao", s.protegida(acesso.StudioAvancado, alvoSuperlogica, s.adminSalvarOrquestracao))
	// a resposta já vem recortada às administradoras visíveis ao usuário
	mux.Handle("GET /api/admin/estado", s.protegidaEmAlgum(acesso.SkillsLer, s.adminEstado))
	// administradoras: POST cria (409 se o id existe), PUT edita (404 se não existe)
	mux.Handle("POST /api/admin/administradora", s.protegida(acesso.AdministradorasEditar, alvoSuperlogica, s.adminCriarAdministradora))
	mux.Handle("PUT /api/admin/administradora", s.protegida(acesso.AdministradorasEditar, alvoSuperlogica, s.adminEditarAdministradora))
	mux.Handle("DELETE /api/admin/administradora", s.protegida(acesso.AdministradorasEditar, alvoSuperlogica, s.adminRemoverAdministradora))
	// condomínios: a dona que o corpo põe tem de estar no escopo; editar e remover conferem
	// também a dona atual
	mux.Handle("POST /api/admin/condominio", s.protegida(acesso.CondominiosEditar, alvoDonaDoCorpo, s.adminCriarCondominio))
	mux.Handle("PUT /api/admin/condominio", s.protegida(acesso.CondominiosEditar, alvoDonaDoCorpo, s.adminEditarCondominio))
	mux.Handle("DELETE /api/admin/condominio", s.protegida(acesso.CondominiosEditar, alvoCondominioDaConsulta, s.adminRemoverCondominio))
	// por condomínio (o padrão "*" é da Superlógica)
	mux.Handle("PUT /api/admin/habilitacao", s.protegida(acesso.CondominiosHabilitar, alvoCondominioDoCorpo, s.adminHabilitacao))
	mux.Handle("PUT /api/admin/orientacao", s.protegida(acesso.CondominiosHabilitar, alvoCondominioDoCorpo, s.adminOrientacao))
	mux.Handle("PUT /api/admin/ajustes", s.protegida(acesso.CondominiosHabilitar, alvoCondominioDoCorpo, s.adminAjustes))
	mux.Handle("POST /api/admin/sugerir-payload", s.protegida(acesso.CondominiosHabilitar, alvoCondominioDoCorpo, s.adminSugerirPayload))
	mux.Handle("PUT /api/admin/payload", s.protegida(acesso.FerramentasEditar, alvoCondominioDoCorpo, s.adminPayload))
}

// casoDeUso é como o Admin vê um subagente do concierge.
type casoDeUso struct {
	Nome      string   `json:"nome"` // pasta (a chave da habilitação)
	ID        string   `json:"id"`   // id do caso.yaml, se houver
	Titulo    string   `json:"titulo"`
	Descricao string   `json:"descricao"`
	Resumo    string   `json:"resumo"` // caso: deriva de summary/description; sem caso: agent.yaml resumo
	Icone     string   `json:"icone"`  // caso.icon; sem caso, vazio
	Escrita   bool     `json:"escrita"`
	Tipo      string   `json:"tipo"`
	Recursos  []string `json:"recursos"` // o que o caso usa (agent.yaml e caso.yaml)
}

func (s *servidor) casosDeUso() ([]casoDeUso, error) {
	dir, err := s.cat.ResolveAgente(s.atende.Agente)
	if err != nil {
		return nil, err
	}
	raiz, err := s.cat.Load(dir)
	if err != nil {
		return nil, err
	}
	reg := map[string]okf.Recurso{}
	for _, r := range s.cat.RecursosNexus() {
		reg[r.Name] = r
	}
	// caso de uso = subagente que o concierge alcança por rota, ou que tem caso.yaml.
	// Agentes de bastidor (o observador da sessão) não são casos de uso.
	rotas := map[string]bool{}
	for _, m := range reRotaAdmin.FindAllStringSubmatch(s.cat.Roteador(raiz), -1) {
		rotas[m[1]] = true
	}
	out := []casoDeUso{}
	for _, sub := range raiz.Subagentes {
		if !rotas[sub.Nome] && sub.Caso == nil {
			continue
		}
		c := casoDeUso{Nome: sub.Nome, Titulo: sub.Nome, Descricao: sub.Descricao, Tipo: sub.Tipo, Recursos: []string{}}
		if resumo, ok := sub.Config["resumo"].(string); ok {
			c.Resumo = resumo
		}
		visto := map[string]bool{}
		for _, r := range sub.Recursos {
			if !visto[r] {
				visto[r] = true
				c.Recursos = append(c.Recursos, r)
			}
		}
		if sub.Caso != nil {
			c.ID = sub.Caso.ID
			if sub.Caso.Name != "" {
				c.Titulo = sub.Caso.Name
			}
			if d := strings.TrimSpace(sub.Caso.Description); d != "" {
				c.Descricao = d // a descrição do caso fala com a administradora; a do agente é critério de rota
			}
			c.Resumo = okf.ResumoDe(sub.Caso.Summary, sub.Caso.Description)
			c.Icone = sub.Caso.Icon
			for _, r := range sub.Caso.Recursos {
				if reg[r].Write {
					c.Escrita = true
				}
				if !visto[r] {
					visto[r] = true
					c.Recursos = append(c.Recursos, r)
				}
			}
		}
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Titulo < out[j].Titulo })
	return out, nil
}

func (s *servidor) adminEstado(w http.ResponseWriter, r *http.Request) {
	m, err := s.admin.Ler(r.Context())
	if err != nil {
		s.responderErroInterno(w, r, "ler mapeamento", err)
		return
	}
	// RLS: cada um vê só as administradoras que pode ler (a Superlógica, todas)
	recorte, err := recorteDoUsuario(r, m.Administradoras)
	if err != nil {
		responderErroDeAlvo(w, r, err)
		return
	}
	m, visiveis := visaoPara(r, m, recorte)
	casos, err := s.casosDeUso()
	if err != nil {
		s.responderErroInterno(w, r, "casos de uso", err)
		return
	}
	recursos := s.cat.RecursosNexus()
	if recursos == nil {
		recursos = []okf.Recurso{}
	}
	writeJSON(w, 200, map[string]any{
		"mapeamento":      m,
		"administradoras": visiveis, // para o seletor de administradora
		"casos":           casos,
		"recursos":        recursos,
		"papeis":          []string{"morador", "proprietario", "sindico", "publico"},
		"account":         s.account,
		"gruvi":           map[string]any{"company_id": s.gruviCompany},
	})
}

func (s *servidor) adminCriarAdministradora(w http.ResponseWriter, r *http.Request) {
	var a admin.Administradora
	if err := decodificarJSON(w, r, &a, limiteCorpoJSON); err != nil {
		writeErr(w, 400, err)
		return
	}
	if a.ID == "" {
		a.ID = slug(a.Nome)
	}
	s.escrever(w, r, http.StatusCreated, func(admin.Mapeamento) (admin.Mapeamento, error) {
		return s.admin.CriarAdministradora(r.Context(), a)
	})
}

func (s *servidor) adminEditarAdministradora(w http.ResponseWriter, r *http.Request) {
	var a admin.Administradora
	if err := decodificarJSON(w, r, &a, limiteCorpoJSON); err != nil {
		writeErr(w, 400, err)
		return
	}
	s.escrever(w, r, http.StatusOK, func(admin.Mapeamento) (admin.Mapeamento, error) {
		return s.admin.EditarAdministradora(r.Context(), a)
	})
}

func (s *servidor) adminRemoverAdministradora(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeErr(w, 400, errors.New("informe id"))
		return
	}
	s.escrever(w, r, http.StatusOK, func(admin.Mapeamento) (admin.Mapeamento, error) {
		return s.admin.RemoverAdministradora(r.Context(), id)
	})
}

func (s *servidor) adminCriarCondominio(w http.ResponseWriter, r *http.Request) {
	var c admin.Condominio
	if err := decodificarJSON(w, r, &c, limiteCorpoJSON); err != nil {
		writeErr(w, 400, err)
		return
	}
	if c.ID == "" {
		c.ID = slug(c.Nome)
	}
	if (c.CompanyID != "" || c.Tenant != "") && !podeGravarChavesDeRoteamento(r) {
		responderSemPermissao(w)
		return
	}
	s.escrever(w, r, http.StatusCreated, func(admin.Mapeamento) (admin.Mapeamento, error) {
		return s.admin.CriarCondominio(r.Context(), c)
	})
}

// adminEditarCondominio confere, além da dona nova (no alvo da rota), a dona atual: ninguém
// edita ou toma para si o condomínio de uma administradora fora do seu escopo. Company e
// tenant são chaves globais de roteamento: só a Superlógica as muda. A dona e as chaves vistas
// aqui vão para o repositório, que recusa se mudaram desde esta conferência.
func (s *servidor) adminEditarCondominio(w http.ResponseWriter, r *http.Request) {
	var c admin.Condominio
	if err := decodificarJSON(w, r, &c, limiteCorpoJSON); err != nil {
		writeErr(w, 400, err)
		return
	}
	c.ID = normalizarID(c.ID)
	if !permitidoAqui(r, acesso.CondominiosEditar, escopoCondominio(c.ID)) {
		responderSemPermissao(w)
		return
	}
	s.escrever(w, r, http.StatusOK, func(atual admin.Mapeamento) (admin.Mapeamento, error) {
		antes, _ := atual.Condominio(c.ID)
		visto := admin.Chaves{Administradora: alvoDaEscrita(r, c.ID).Dona, CompanyID: antes.CompanyID, Tenant: antes.Tenant}
		mudaChaves := c.CompanyID != antes.CompanyID || c.Tenant != antes.Tenant
		if mudaChaves && !podeGravarChavesDeRoteamento(r) {
			return admin.Mapeamento{}, errSemPermissao
		}
		return s.admin.EditarCondominio(r.Context(), c, visto)
	})
}

// podeGravarChavesDeRoteamento: company e tenant decidem que condomínio (e administradora)
// atende cada mensagem; só a Superlógica os grava.
func podeGravarChavesDeRoteamento(r *http.Request) bool {
	return permitidoAqui(r, acesso.CondominiosEditar, escopoSuperlogica)
}

func (s *servidor) adminRemoverCondominio(w http.ResponseWriter, r *http.Request) {
	id := normalizarID(r.URL.Query().Get("id"))
	if id == "" {
		writeErr(w, 400, errors.New("informe id"))
		return
	}
	s.escrever(w, r, http.StatusOK, func(admin.Mapeamento) (admin.Mapeamento, error) {
		return s.admin.RemoverCondominio(r.Context(), alvoDaEscrita(r, id))
	})
}

func (s *servidor) adminHabilitacao(w http.ResponseWriter, r *http.Request) {
	var corpo struct {
		Condominio string   `json:"condominio"`
		Caso       string   `json:"caso"`
		Habilitado bool     `json:"habilitado"`
		Papeis     []string `json:"papeis"`
		Limpar     bool     `json:"limpar"`
	}
	if err := decodificarJSON(w, r, &corpo, limiteCorpoJSON); err != nil {
		writeErr(w, 400, err)
		return
	}
	corpo.Condominio = cmp.Or(normalizarID(corpo.Condominio), admin.Padrao)
	s.escrever(w, r, http.StatusOK, func(admin.Mapeamento) (admin.Mapeamento, error) {
		return s.admin.SalvarHabilitacao(r.Context(), alvoDaEscrita(r, corpo.Condominio), corpo.Caso, admin.Habilitacao{Habilitado: corpo.Habilitado, Papeis: corpo.Papeis}, corpo.Limpar)
	})
}

func (s *servidor) adminPayload(w http.ResponseWriter, r *http.Request) {
	var corpo struct {
		Condominio string         `json:"condominio"`
		Recurso    string         `json:"recurso"`
		Payload    map[string]any `json:"payload"`
		Apagar     bool           `json:"apagar"`
	}
	if err := decodificarJSON(w, r, &corpo, limiteCorpoJSON); err != nil {
		writeErr(w, 400, err)
		return
	}
	corpo.Condominio = cmp.Or(normalizarID(corpo.Condominio), admin.Padrao)
	if corpo.Apagar {
		corpo.Payload = nil
	} else if corpo.Payload == nil {
		corpo.Payload = map[string]any{}
	}
	s.escrever(w, r, http.StatusOK, func(admin.Mapeamento) (admin.Mapeamento, error) {
		return s.admin.SalvarPayload(r.Context(), alvoDaEscrita(r, corpo.Condominio), corpo.Recurso, corpo.Payload)
	})
}

// adminSugerirPayload: o modelo barato traduz um texto da administradora nas definições
// do recurso (o formulário do payload), para a pessoa revisar antes de salvar.
func (s *servidor) adminSugerirPayload(w http.ResponseWriter, r *http.Request) {
	var corpo struct {
		Condominio string         `json:"condominio"`
		Recurso    string         `json:"recurso"`
		Texto      string         `json:"texto"`
		Atual      map[string]any `json:"atual"`
	}
	if err := decodificarJSON(w, r, &corpo, limiteCorpoJSON); err != nil {
		writeErr(w, 400, err)
		return
	}
	var rec *okf.Recurso
	for _, x := range s.cat.RecursosNexus() {
		if x.Name == corpo.Recurso {
			rec = &x
			break
		}
	}
	if rec == nil {
		writeErr(w, 404, fmt.Errorf("recurso desconhecido: %s", corpo.Recurso))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	sug, err := harness.SugerirPayload(ctx, s.atende.Config, *rec, corpo.Atual, corpo.Texto)
	if err != nil {
		s.logger().WarnContext(r.Context(), "sugerir payload", "operacao", operacaoDe(r), "recurso", corpo.Recurso, "erro", err.Error())
		writeJSON(w, http.StatusBadRequest, map[string]string{"codigo": "SUGESTAO_FALHOU", "error": "não foi possível sugerir o payload; tente reescrever o texto"})
		return
	}
	writeJSON(w, 200, sug)
}

func (s *servidor) adminOrientacao(w http.ResponseWriter, r *http.Request) {
	var corpo struct {
		Condominio string            `json:"condominio"`
		Geral      string            `json:"geral"`
		PorCaso    map[string]string `json:"por_caso"`
	}
	if err := decodificarJSON(w, r, &corpo, limiteCorpoJSON); err != nil {
		writeErr(w, 400, err)
		return
	}
	corpo.Condominio = cmp.Or(normalizarID(corpo.Condominio), admin.Padrao)
	s.escrever(w, r, http.StatusOK, func(admin.Mapeamento) (admin.Mapeamento, error) {
		return s.admin.SalvarOrientacao(r.Context(), alvoDaEscrita(r, corpo.Condominio), admin.Orientacao{Geral: corpo.Geral, PorCaso: corpo.PorCaso})
	})
}

func (s *servidor) adminAjustes(w http.ResponseWriter, r *http.Request) {
	var corpo struct {
		Condominio string                    `json:"condominio"`
		Ajustes    map[string]map[string]any `json:"ajustes"`
	}
	if err := decodificarJSON(w, r, &corpo, limiteCorpoJSON); err != nil {
		writeErr(w, 400, err)
		return
	}
	corpo.Condominio = cmp.Or(normalizarID(corpo.Condominio), admin.Padrao)
	s.escrever(w, r, http.StatusOK, func(admin.Mapeamento) (admin.Mapeamento, error) {
		return s.admin.SalvarAjustes(r.Context(), alvoDaEscrita(r, corpo.Condominio), corpo.Ajustes)
	})
}

// telaAntiga serve uma página de web/okf que o Studio ainda não substituiu.
func (s *servidor) telaAntiga(arquivo string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		raw, err := fs.ReadFile(s.web, arquivo)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(raw)
	}
}

// codigoConflitoConcorrente é o código que o Admin recebe para repetir o pedido.
const codigoConflitoConcorrente = "CONFLITO_CONCORRENTE"

// responderErroAdmin: o que a regra do mapeamento recusa é erro do pedido (400, 404 ou 409);
// conflito com outra escrita é temporário (503, sem o texto do driver, que fica só no log);
// o resto é do servidor (500 com mensagem fixa, o erro só no log).
func (s *servidor) responderErroAdmin(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, admin.ErrJaExiste):
		writeJSON(w, http.StatusConflict, map[string]string{"codigo": "JA_EXISTE", "error": err.Error()})
	case errors.Is(err, admin.ErrNaoEncontrado):
		writeJSON(w, http.StatusNotFound, map[string]string{"codigo": "NAO_ENCONTRADO", "error": err.Error()})
	case errors.Is(err, admin.ErrInvalido):
		writeErr(w, http.StatusBadRequest, err)
	case errors.Is(err, admin.ErrConflitoConcorrente):
		s.logger().WarnContext(r.Context(), "conflito ao gravar o mapeamento", "operacao", operacaoDe(r), "erro", err.Error())
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error":  "outra alteração do mapeamento aconteceu ao mesmo tempo; tente de novo",
			"codigo": codigoConflitoConcorrente,
		})
	default:
		s.responderErroInterno(w, r, "gravar mapeamento", err)
	}
}

// errSemPermissao: a escrita recusada por permissão depois de ler o estado atual (403).
var errSemPermissao = erroDeAlvo{http.StatusForbidden, "SEM_PERMISSAO", "sem permissão para esta operação"}

// escrever lê o mapeamento atual, decide o recorte da resposta ANTES de gravar (o que pode
// falhar falha sem efeito), aplica a escrita e devolve o mapeamento recortado ao que o
// usuário pode ler: uma escrita gravada nunca responde erro.
func (s *servidor) escrever(w http.ResponseWriter, r *http.Request, status int, escrita func(atual admin.Mapeamento) (admin.Mapeamento, error)) {
	atual, err := s.admin.Ler(r.Context())
	if err != nil {
		s.responderErroInterno(w, r, "ler mapeamento", err)
		return
	}
	recorte, err := recorteDoUsuario(r, atual.Administradoras)
	if err != nil {
		responderErroDeAlvo(w, r, err)
		return
	}
	m, err := escrita(atual)
	if _, ehDeAlvo := errors.AsType[erroDeAlvo](err); ehDeAlvo {
		responderErroDeAlvo(w, r, err)
		return
	}
	if err != nil {
		s.responderErroAdmin(w, r, err)
		return
	}
	v, visiveis := visaoPara(r, m, recorte)
	writeJSON(w, status, map[string]any{"mapeamento": v, "administradoras": visiveis})
}

var reRotaAdmin = regexp.MustCompile("rota `([^`]+)`")

func slug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	r := strings.NewReplacer("á", "a", "à", "a", "ã", "a", "â", "a", "é", "e", "ê", "e", "í", "i", "ó", "o", "ô", "o", "õ", "o", "ú", "u", "ç", "c", " ", "-")
	s = r.Replace(s)
	var b strings.Builder
	for _, c := range s {
		if c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '_' {
			b.WriteRune(c)
		}
	}
	return strings.Trim(b.String(), "-")
}

// ---- orquestração: router | planning | react, no agent.yaml do concierge ----

func semAmbiente(string) string { return "" }

// orquestracaoAtual lê o agent.yaml do concierge e devolve o que está no arquivo, o que
// vale de fato (ORQUESTRACAO_MODO do ambiente vence) e se o ambiente está forçando.
func (s *servidor) orquestracaoAtual() (map[string]any, error) {
	dir, err := s.cat.ResolveAgente(s.atende.Agente)
	if err != nil {
		return nil, err
	}
	raiz, err := s.cat.Load(dir)
	if err != nil {
		return nil, err
	}
	arquivo := harness.OrquestracaoDe(raiz, semAmbiente)
	efetivo := harness.OrquestracaoDe(raiz, os.Getenv)
	ambiente := strings.ToLower(strings.TrimSpace(os.Getenv("ORQUESTRACAO_MODO")))
	return map[string]any{
		"agente":   s.atende.Agente,
		"arquivo":  arquivo,
		"efetivo":  efetivo,
		"ambiente": ambiente,
	}, nil
}

func (s *servidor) adminOrquestracao(w http.ResponseWriter, r *http.Request) {
	out, err := s.orquestracaoAtual()
	if err != nil {
		s.responderErroInterno(w, r, "ler orquestração", err)
		return
	}
	writeJSON(w, 200, out)
}

func (s *servidor) adminSalvarOrquestracao(w http.ResponseWriter, r *http.Request) {
	var corpo harness.Orquestracao
	if err := decodificarJSON(w, r, &corpo, limiteCorpoJSON); err != nil {
		writeErr(w, 400, err)
		return
	}
	corpo.Modo = strings.ToLower(strings.TrimSpace(corpo.Modo))
	switch corpo.Modo {
	case harness.ModoRouter, harness.ModoPlanning, harness.ModoReAct:
	default:
		writeErr(w, 400, errors.New("modo deve ser router, planning ou react"))
		return
	}
	if corpo.BindConfidence < 0.5 || corpo.BindConfidence > 1 {
		writeErr(w, 400, errors.New("bind_confidence deve ficar entre 0,5 e 1"))
		return
	}
	if corpo.MaxItens < 1 || corpo.MaxItens > 10 {
		writeErr(w, 400, errors.New("max_itens deve ficar entre 1 e 10"))
		return
	}
	if corpo.MaxIteracoes < 1 || corpo.MaxIteracoes > 15 {
		writeErr(w, 400, errors.New("max_iteracoes deve ficar entre 1 e 15"))
		return
	}
	campos := map[string]any{"orquestracao": map[string]any{
		"modo":            corpo.Modo,
		"bind_confidence": corpo.BindConfidence,
		"max_itens":       corpo.MaxItens,
		"max_iteracoes":   corpo.MaxIteracoes,
	}}
	if err := s.cat.SalvarConfig(s.atende.Agente, campos); err != nil {
		writeErr(w, 400, err)
		return
	}
	out, err := s.orquestracaoAtual()
	if err != nil {
		s.responderErroInterno(w, r, "ler orquestração", err)
		return
	}
	writeJSON(w, 200, out)
}
