package main

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"regexp"
	"sort"
	"strings"

	"go-concierge/internal/admin"
	"go-concierge/internal/okf"
)

// Admin da administradora: a matriz condomínio × caso de uso (habilitação e papéis),
// restrita aos condomínios dela (?administradora=<id> emula o login; RLS). O resto é da
// Superlógica, no Backdoor: administradoras e seus condomínios, casos de uso, tools,
// recursos e o payload de cada recurso por condomínio.

func (s *servidor) rotasAdmin(mux *http.ServeMux) {
	mux.HandleFunc("GET /backdoor", func(w http.ResponseWriter, r *http.Request) {
		raw, err := fs.ReadFile(s.web, "backdoor.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Write(raw)
	})
	mux.HandleFunc("GET /backdoor/payloads", func(w http.ResponseWriter, r *http.Request) {
		raw, err := fs.ReadFile(s.web, "payloads.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Write(raw)
	})
	mux.HandleFunc("GET /backdoor/administradoras", func(w http.ResponseWriter, r *http.Request) {
		raw, err := fs.ReadFile(s.web, "administradoras.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Write(raw)
	})
	mux.HandleFunc("GET /api/admin/estado", s.adminEstado)
	mux.HandleFunc("PUT /api/admin/administradora", s.adminAdministradora)
	mux.HandleFunc("DELETE /api/admin/administradora", s.adminRemoverAdministradora)
	mux.HandleFunc("PUT /api/admin/condominio", s.adminCondominio)
	mux.HandleFunc("DELETE /api/admin/condominio", s.adminRemoverCondominio)
	mux.HandleFunc("PUT /api/admin/habilitacao", s.adminHabilitacao)
	mux.HandleFunc("PUT /api/admin/payload", s.adminPayload)
}

// casoDeUso é como o Admin vê um subagente do concierge.
type casoDeUso struct {
	Nome      string `json:"nome"` // pasta (a chave da habilitação)
	ID        string `json:"id"`   // id do caso.yaml, se houver
	Titulo    string `json:"titulo"`
	Descricao string `json:"descricao"`
	Escrita   bool   `json:"escrita"`
	Tipo      string `json:"tipo"`
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
		c := casoDeUso{Nome: sub.Nome, Titulo: sub.Nome, Descricao: sub.Descricao, Tipo: sub.Tipo}
		if sub.Caso != nil {
			c.ID = sub.Caso.ID
			if sub.Caso.Name != "" {
				c.Titulo = sub.Caso.Name
			}
			for _, r := range sub.Caso.Recursos {
				if reg[r].Write {
					c.Escrita = true
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
