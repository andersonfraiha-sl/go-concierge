package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
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
		{"asset inexistente", compilado, "/studio/assets/nao.js", 404, "", ""},
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
	s := &servidor{web: fstest.MapFS{"console.html": {Data: []byte("console")}}, studio: fstest.MapFS{}}
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
		{"/backdoor", http.StatusNotFound, ""},
		{"/backdoor/orquestracao", http.StatusNotFound, ""},
		{"/console", http.StatusOK, ""},
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
