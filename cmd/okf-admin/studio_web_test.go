package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"go-concierge/internal/okf"
)

func TestStudioEstatico(t *testing.T) {
	compilado := fstest.MapFS{
		"index.html":  {Data: []byte("<html>studio</html>")},
		"assets/a.js": {Data: []byte("console.log(1)")},
	}
	casos := []struct {
		nome       string
		arquivos   fstest.MapFS
		caminho    string
		querStatus int
		querCorpo  string
		querCache  string
	}{
		{"raiz serve o index", compilado, "/studio/", 200, "<html>studio</html>", "no-store"},
		{"rota do cliente cai no index", compilado, "/studio/skills/agendar-mudanca", 200, "<html>studio</html>", "no-store"},
		{"asset existente fica em cache", compilado, "/studio/assets/a.js", 200, "console.log(1)", "public, max-age=31536000, immutable"},
		{"index pelo nome sai sem cache", compilado, "/studio/index.html", 200, "<html>studio</html>", "no-store"},
		{"asset inexistente", compilado, "/studio/assets/nao.js", 404, "404 page not found", ""},
		{"asset inexistente sem extensão", compilado, "/studio/assets/nao", 404, "404 page not found", ""},
		{"arquivo inexistente com extensão", compilado, "/studio/favicon.ico", 404, "404 page not found", ""},
		{"sem compilar", fstest.MapFS{}, "/studio/", 503, "STUDIO_NAO_COMPILADO", ""},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			s := &servidor{studio: c.arquivos}
			mux := http.NewServeMux()
			s.rotasStudio(mux)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, c.caminho, nil))

			if rec.Code != c.querStatus {
				t.Fatalf("GET %s = %d, quer %d (%s)", c.caminho, rec.Code, c.querStatus, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), c.querCorpo) {
				t.Errorf("corpo %q não contém %q", rec.Body.String(), c.querCorpo)
			}
			if c.querCache != "" && rec.Header().Get("Cache-Control") != c.querCache {
				t.Errorf("Cache-Control = %q, quer %q", rec.Header().Get("Cache-Control"), c.querCache)
			}
		})
	}
}

func TestStudioNaoCompiladoRespondeErroDaAPI(t *testing.T) {
	s := &servidor{studio: fstest.MapFS{}}
	mux := http.NewServeMux()
	s.rotasStudio(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/studio/skills", nil))

	var got erroAPI
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("corpo não é JSON: %v (%s)", err, rec.Body.String())
	}
	quer := erroAPI{Codigo: "STUDIO_NAO_COMPILADO", Mensagem: "O Studio não foi compilado: rode npm run build em web/studio/."}
	if got.Codigo != quer.Codigo || got.Mensagem != quer.Mensagem {
		t.Fatalf("erro = %+v, quer %+v", got, quer)
	}
}

func TestRaizVaiParaStudio(t *testing.T) {
	s := &servidor{web: fstest.MapFS{"console.html": {Data: []byte("console")}, "index.html": {Data: []byte("admin antigo")}}, studio: fstest.MapFS{}}
	mux := http.NewServeMux()
	s.rotas(mux)
	s.rotasStudio(mux)
	s.rotasAtende(mux)
	s.rotasAdmin(mux)

	casos := []struct {
		caminho      string
		querStatus   int
		querLocation string
	}{
		{"/", http.StatusFound, "/studio/"},
		{"/studio", http.StatusMovedPermanently, "/studio/"},
		{"/console", http.StatusOK, ""},
		{"/admin", http.StatusOK, ""},
	}
	for _, c := range casos {
		t.Run(c.caminho, func(t *testing.T) {
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, c.caminho, nil))
			if rec.Code != c.querStatus {
				t.Fatalf("GET %s = %d, quer %d", c.caminho, rec.Code, c.querStatus)
			}
			if got := rec.Header().Get("Location"); got != c.querLocation {
				t.Errorf("Location = %q, quer %q", got, c.querLocation)
			}
		})
	}
}

func TestTelasAntigasContinuamNoAr(t *testing.T) {
	web := fstest.MapFS{}
	for _, f := range []string{"index.html", "backdoor.html", "orquestracao.html", "administradoras.html", "payloads.html"} {
		web[f] = &fstest.MapFile{Data: []byte(f)}
	}
	s := &servidor{web: web, studio: fstest.MapFS{}}
	mux := http.NewServeMux()
	s.rotas(mux)
	s.rotasStudio(mux)
	s.rotasAdmin(mux)

	casos := []struct {
		caminho string
		arquivo string
	}{
		{"/admin", "index.html"},
		{"/backdoor", "backdoor.html"},
		{"/backdoor/orquestracao", "orquestracao.html"},
		{"/backdoor/administradoras", "administradoras.html"},
		{"/backdoor/payloads", "payloads.html"},
	}
	for _, c := range casos {
		t.Run(c.caminho, func(t *testing.T) {
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, c.caminho, nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("GET %s = %d, quer 200", c.caminho, rec.Code)
			}
			if rec.Body.String() != c.arquivo {
				t.Errorf("GET %s corpo = %q, quer %q", c.caminho, rec.Body.String(), c.arquivo)
			}
		})
	}

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusFound {
		t.Fatalf("GET / = %d, quer 302", rec.Code)
	}
	if got := rec.Header().Get("Location"); got != "/studio/" {
		t.Errorf("Location = %q, quer /studio/", got)
	}
}

func TestRotasDaTelaAntigaRegistradas(t *testing.T) {
	s := &servidor{}
	mux := http.NewServeMux()
	s.rotas(mux)

	casos := []struct {
		metodo  string
		caminho string
		padrao  string
	}{
		{http.MethodGet, "/api/anatomia", "GET /api/anatomia"},
		{http.MethodPost, "/api/agentes", "POST /api/agentes"},
		{http.MethodPut, "/api/agente/config", "PUT /api/agente/config"},
		{http.MethodDelete, "/api/agente", "DELETE /api/agente"},
		{http.MethodPost, "/api/agentes/mover", "POST /api/agentes/mover"},
	}
	for _, c := range casos {
		t.Run(c.padrao, func(t *testing.T) {
			_, padrao := mux.Handler(httptest.NewRequest(c.metodo, c.caminho, nil))
			if padrao != c.padrao {
				t.Errorf("padrão de %s %s = %q, quer %q", c.metodo, c.caminho, padrao, c.padrao)
			}
		})
	}
}

// TestExcluirDevolveMudancas confere que o corpo de DELETE /api/agente sempre tem
// "mudancas" (mesmo vazia): backdoor.js:464 lê r.mudancas.length sem checar undefined, e
// um corpo sem o campo faz o delete bem-sucedido aparecer como erro na tela.
func TestExcluirDevolveMudancas(t *testing.T) {
	raiz := t.TempDir()
	if err := os.CopyFS(filepath.Join(raiz, "agentes"), os.DirFS(filepath.Join("..", "..", "okf", "agentes"))); err != nil {
		t.Fatal(err)
	}
	cat, err := okf.NewCatalogo(raiz)
	if err != nil {
		t.Fatal(err)
	}
	s := &servidor{cat: cat}

	rec := httptest.NewRecorder()
	s.excluir(rec, httptest.NewRequest(http.MethodDelete, "/api/agente?agente=agentes/concierge/agentes/faq", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("DELETE /api/agente = %d, corpo %s", rec.Code, rec.Body.String())
	}

	var corpo struct {
		Mudancas *[]string `json:"mudancas"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &corpo); err != nil {
		t.Fatalf("corpo não é JSON: %v (%s)", err, rec.Body.String())
	}
	if corpo.Mudancas == nil {
		t.Fatalf("resposta sem mudancas: %s", rec.Body.String())
	}
}
