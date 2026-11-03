package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"testing"

	"go-concierge/internal/admin"
)

// termoTecnico pega vocabulário de implementação (Nexus, MCP, YAML, caso de uso,
// subagente, orquestrador, roteador, backdoor) que não deveria chegar ao Studio: quem usa
// a tela é a administradora, não quem construiu o agente.
var termoTecnico = regexp.MustCompile(`(?i)\bnexus\b|\bmcp\b|\byaml\b|caso de uso|subagente|orquestrador|roteador|backdoor`)

// handlerComAdminDeTeste é handlerDeTeste com as rotas do Admin também registradas, para
// testar o vocabulário que a tela da administradora recebe.
func handlerComAdminDeTeste(t *testing.T) http.Handler {
	t.Helper()
	s := servidorDeTeste(t, func(string) {})
	s.admin = admin.NewStore(filepath.Join(t.TempDir(), "mapeamento.json"))
	mux := http.NewServeMux()
	s.rotasAdmin(mux)
	s.rotasStudio(mux)
	return mux
}

func TestDadosDoStudioSemVocabularioTecnico(t *testing.T) {
	h := handlerComAdminDeTeste(t)

	checarValor := func(rota, campo, valor string) {
		if termoTecnico.MatchString(valor) {
			t.Errorf("%s %s: %q", rota, campo, valor)
		}
	}

	rotaEstado := "GET /api/admin/estado"
	r := httptest.NewRequest("GET", "/api/admin/estado?administradora=adm-exemplo", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("%s: status %d, corpo %s", rotaEstado, w.Code, w.Body.String())
	}
	var estado struct {
		Casos []struct {
			Titulo    string `json:"titulo"`
			Resumo    string `json:"resumo"`
			Descricao string `json:"descricao"`
		} `json:"casos"`
		Recursos []struct {
			Titulo     string `json:"titulo"`
			Definicoes []struct {
				Label       string `json:"label"`
				Description string `json:"description"`
			} `json:"definicoes"`
		} `json:"recursos"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &estado); err != nil {
		t.Fatalf("%s: decodificar corpo: %v", rotaEstado, err)
	}
	for _, c := range estado.Casos {
		checarValor(rotaEstado, "casos[].titulo", c.Titulo)
		checarValor(rotaEstado, "casos[].resumo", c.Resumo)
		checarValor(rotaEstado, "casos[].descricao", c.Descricao)
	}
	for _, rec := range estado.Recursos {
		checarValor(rotaEstado, "recursos[].titulo", rec.Titulo)
		for _, d := range rec.Definicoes {
			checarValor(rotaEstado, "recursos[].definicoes[].label", d.Label)
			checarValor(rotaEstado, "recursos[].definicoes[].description", d.Description)
		}
	}

	rotaSkills := "GET /api/skills"
	r = httptest.NewRequest("GET", "/api/skills", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("%s: status %d, corpo %s", rotaSkills, w.Code, w.Body.String())
	}
	var skills struct {
		Skills []struct {
			Titulo    string `json:"titulo"`
			Resumo    string `json:"resumo"`
			Descricao string `json:"descricao"`
		} `json:"skills"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &skills); err != nil {
		t.Fatalf("%s: decodificar corpo: %v", rotaSkills, err)
	}
	for _, sk := range skills.Skills {
		checarValor(rotaSkills, "skills[].titulo", sk.Titulo)
		checarValor(rotaSkills, "skills[].resumo", sk.Resumo)
		checarValor(rotaSkills, "skills[].descricao", sk.Descricao)
	}
}
