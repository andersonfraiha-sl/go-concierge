package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

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
	mux.HandleFunc("GET /backdoor/payloads", s.telaAntiga("payloads.html"))
	mux.HandleFunc("GET /backdoor/administradoras", s.telaAntiga("administradoras.html"))
	mux.HandleFunc("GET /api/admin/orquestracao", s.adminOrquestracao)
	mux.HandleFunc("PUT /api/admin/orquestracao", s.adminSalvarOrquestracao)
	mux.HandleFunc("GET /api/admin/estado", s.adminEstado)
	mux.HandleFunc("PUT /api/admin/administradora", s.adminAdministradora)
	mux.HandleFunc("DELETE /api/admin/administradora", s.adminRemoverAdministradora)
	mux.HandleFunc("PUT /api/admin/condominio", s.adminCondominio)
	mux.HandleFunc("DELETE /api/admin/condominio", s.adminRemoverCondominio)
	mux.HandleFunc("PUT /api/admin/habilitacao", s.adminHabilitacao)
	mux.HandleFunc("PUT /api/admin/payload", s.adminPayload)
	mux.HandleFunc("PUT /api/admin/orientacao", s.adminOrientacao)
	mux.HandleFunc("POST /api/admin/sugerir-payload", s.adminSugerirPayload)
	mux.HandleFunc("PUT /api/admin/ajustes", s.adminAjustes)
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
	m, err := s.admin.Ler()
	if err != nil {
		writeErr(w, 500, err)
		return
	}
	// RLS: a administradora só vê o que é dela. Sem o parâmetro, é o Backdoor (tudo).
	todas := m.Administradoras
	m = m.Visao(r.URL.Query().Get("administradora"))
	casos, err := s.casosDeUso()
	if err != nil {
		writeErr(w, 500, err)
		return
	}
	recursos := s.cat.RecursosNexus()
	if recursos == nil {
		recursos = []okf.Recurso{}
	}
	writeJSON(w, 200, map[string]any{
		"mapeamento":      m,
		"administradoras": todas, // a lista inteira, para o seletor que emula o login
		"casos":           casos,
		"recursos":        recursos,
		"papeis":          []string{"morador", "proprietario", "sindico", "publico"},
		"account":         s.account,
		"gruvi":           map[string]any{"company_id": s.gruviCompany},
	})
}

func (s *servidor) adminAdministradora(w http.ResponseWriter, r *http.Request) {
	var a admin.Administradora
	if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
		writeErr(w, 400, err)
		return
	}
	if a.ID == "" {
		a.ID = slug(a.Nome)
	}
	m, err := s.admin.SalvarAdministradora(a)
	if err != nil {
		writeErr(w, 400, err)
		return
	}
	s.responderMapeamento(w, r, m)
}

func (s *servidor) adminRemoverAdministradora(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeErr(w, 400, errors.New("informe id"))
		return
	}
	m, err := s.admin.RemoverAdministradora(id)
	if err != nil {
		writeErr(w, 400, err)
		return
	}
	s.responderMapeamento(w, r, m)
}

func (s *servidor) adminCondominio(w http.ResponseWriter, r *http.Request) {
	var c admin.Condominio
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		writeErr(w, 400, err)
		return
	}
	if c.ID == "" {
		c.ID = slug(c.Nome)
	}
	m, err := s.admin.SalvarCondominio(c)
	if err != nil {
		writeErr(w, 400, err)
		return
	}
	s.responderMapeamento(w, r, m)
}

func (s *servidor) adminRemoverCondominio(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeErr(w, 400, errors.New("informe id"))
		return
	}
	m, err := s.admin.RemoverCondominio(id)
	if err != nil {
		writeErr(w, 400, err)
		return
	}
	s.responderMapeamento(w, r, m)
}

func (s *servidor) adminHabilitacao(w http.ResponseWriter, r *http.Request) {
	var corpo struct {
		Condominio string   `json:"condominio"`
		Caso       string   `json:"caso"`
		Habilitado bool     `json:"habilitado"`
		Papeis     []string `json:"papeis"`
		Limpar     bool     `json:"limpar"`
	}
	if err := json.NewDecoder(r.Body).Decode(&corpo); err != nil {
		writeErr(w, 400, err)
		return
	}
	if corpo.Condominio == "" {
		corpo.Condominio = admin.Padrao
	}
	m, err := s.admin.SalvarHabilitacao(corpo.Condominio, corpo.Caso, admin.Habilitacao{Habilitado: corpo.Habilitado, Papeis: corpo.Papeis}, corpo.Limpar)
	if err != nil {
		writeErr(w, 400, err)
		return
	}
	s.responderMapeamento(w, r, m)
}

func (s *servidor) adminPayload(w http.ResponseWriter, r *http.Request) {
	var corpo struct {
		Condominio string         `json:"condominio"`
		Recurso    string         `json:"recurso"`
		Payload    map[string]any `json:"payload"`
		Apagar     bool           `json:"apagar"`
	}
	if err := json.NewDecoder(r.Body).Decode(&corpo); err != nil {
		writeErr(w, 400, err)
		return
	}
	if corpo.Condominio == "" {
		corpo.Condominio = admin.Padrao
	}
	if corpo.Apagar {
		corpo.Payload = nil
	} else if corpo.Payload == nil {
		corpo.Payload = map[string]any{}
	}
	m, err := s.admin.SalvarPayload(corpo.Condominio, corpo.Recurso, corpo.Payload)
	if err != nil {
		writeErr(w, 400, err)
		return
	}
	s.responderMapeamento(w, r, m)
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
	if err := json.NewDecoder(r.Body).Decode(&corpo); err != nil {
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
		writeErr(w, 400, err)
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
	if err := json.NewDecoder(r.Body).Decode(&corpo); err != nil {
		writeErr(w, 400, err)
		return
	}
	if corpo.Condominio == "" {
		corpo.Condominio = admin.Padrao
	}
	m, err := s.admin.SalvarOrientacao(corpo.Condominio, admin.Orientacao{Geral: corpo.Geral, PorCaso: corpo.PorCaso})
	if err != nil {
		writeErr(w, 400, err)
		return
	}
	s.responderMapeamento(w, r, m)
}

func (s *servidor) adminAjustes(w http.ResponseWriter, r *http.Request) {
	var corpo struct {
		Condominio string                    `json:"condominio"`
		Ajustes    map[string]map[string]any `json:"ajustes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&corpo); err != nil {
		writeErr(w, 400, err)
		return
	}
	if corpo.Condominio == "" {
		corpo.Condominio = admin.Padrao
	}
	m, err := s.admin.SalvarAjustes(corpo.Condominio, corpo.Ajustes)
	if err != nil {
		writeErr(w, 400, err)
		return
	}
	s.responderMapeamento(w, r, m)
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

// responderMapeamento devolve o mapeamento já recortado pela administradora do
// pedido (?administradora=), para o Admin nunca receber o que não é dele.
func (s *servidor) responderMapeamento(w http.ResponseWriter, r *http.Request, m admin.Mapeamento) {
	todas := m.Administradoras
	writeJSON(w, 200, map[string]any{"mapeamento": m.Visao(r.URL.Query().Get("administradora")), "administradoras": todas})
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

func (s *servidor) adminOrquestracao(w http.ResponseWriter, _ *http.Request) {
	out, err := s.orquestracaoAtual()
	if err != nil {
		writeErr(w, 500, err)
		return
	}
	writeJSON(w, 200, out)
}

func (s *servidor) adminSalvarOrquestracao(w http.ResponseWriter, r *http.Request) {
	var corpo harness.Orquestracao
	if err := json.NewDecoder(r.Body).Decode(&corpo); err != nil {
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
		writeErr(w, 500, err)
		return
	}
	writeJSON(w, 200, out)
}
