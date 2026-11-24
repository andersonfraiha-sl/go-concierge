package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go-concierge/internal/httpapp"
)

func TestBackdoorSeparaTelasDoNucleo(t *testing.T) {
	chamadas := 0
	nucleo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		chamadas++
		if r.URL.Path != "/api/skills" || r.Header.Get("X-Dev-Usuario") != "operador" {
			t.Errorf("requisição inesperada: %v", r)
		}
		httpapp.JSON(w, http.StatusForbidden, map[string]string{"codigo": "SEM_PERMISSAO"})
	}))
	defer nucleo.Close()
	h, err := Novo(httpapp.Config{ConciergeURL: nucleo.URL, ConsoleURL: "http://console.local"})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		path     string
		status   int
		location string
	}{
		{"/", 302, "/studio/"}, {"/admin", 200, ""}, {"/backdoor", 200, ""}, {"/admin.js", 200, ""}, {"/console", 302, "http://console.local/console"}, {"/webhook/atende", 404, ""},
	} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, c.path, nil))
		if w.Code != c.status || w.Header().Get("Location") != c.location {
			t.Errorf("%s: %d %s", c.path, w.Code, w.Header().Get("Location"))
		}
	}
	if chamadas != 0 {
		t.Fatal("telas locais dependeram do núcleo")
	}
	r := httptest.NewRequest(http.MethodGet, "/api/skills", nil)
	r.Header.Set("X-Dev-Usuario", "operador")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 || !strings.Contains(w.Body.String(), "SEM_PERMISSAO") || chamadas != 1 {
		t.Fatalf("API: %d %s", w.Code, w.Body.String())
	}
}
