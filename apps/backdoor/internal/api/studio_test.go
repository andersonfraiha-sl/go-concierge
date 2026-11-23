package api

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
		{"index pelo nome sai sem cache", compilado, "/studio/index.html", 200, "<html>studio</html>", "no-store"},
		{"asset inexistente", compilado, "/studio/assets/nao.js", 404, "404 page not found", ""},
		{"asset inexistente sem extensão", compilado, "/studio/assets/nao", 404, "404 page not found", ""},
		{"arquivo inexistente com extensão", compilado, "/studio/favicon.ico", 404, "404 page not found", ""},
		{"sem compilar", fstest.MapFS{}, "/studio/", 503, "STUDIO_NAO_COMPILADO", ""},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.Handle("GET /studio/", studioEstatico(c.arquivos))
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
	mux := http.NewServeMux()
	mux.Handle("GET /studio/", studioEstatico(fstest.MapFS{}))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/studio/skills", nil))

	var got struct {
		Codigo   string `json:"codigo"`
		Mensagem string `json:"mensagem"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("corpo não é JSON: %v (%s)", err, rec.Body.String())
	}
	quer := erroAPI{Codigo: "STUDIO_NAO_COMPILADO", Mensagem: "O Studio não foi compilado: rode npm run build em apps/backdoor/web/studio/."}
	if got.Codigo != quer.Codigo || got.Mensagem != quer.Mensagem {
		t.Fatalf("erro = %+v, quer %+v", got, quer)
	}
}
