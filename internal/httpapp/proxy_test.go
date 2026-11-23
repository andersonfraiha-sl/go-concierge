package httpapp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProxyPreservaContextoERecusaAlvoInvalido(t *testing.T) {
	alvo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RequestURI() != "/api/skills?company_id=empresa-a" || r.Header.Get("Authorization") != "Bearer exemplo" || r.Header.Get("X-Dev-Usuario") != "operador" {
			t.Errorf("contexto HTTP perdido: %s, headers %v", r.URL, r.Header)
		}
		JSON(w, http.StatusForbidden, map[string]string{"codigo": "SEM_PERMISSAO"})
	}))
	defer alvo.Close()
	proxy, err := NovoProxy(alvo.URL)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodGet, "/api/skills?company_id=empresa-a", nil)
	r.Header.Set("Authorization", "Bearer exemplo")
	r.Header.Set("X-Dev-Usuario", "operador")
	w := httptest.NewRecorder()
	proxy.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "SEM_PERMISSAO") {
		t.Fatalf("resposta alterada: %d %s", w.Code, w.Body.String())
	}
	for _, base := range []string{"", "file:///tmp/dados", "http://usuario:senha@host", "http://host?token=segredo", "http://host#fragmento"} {
		if _, err := NovoProxy(base); err == nil {
			t.Errorf("aceitou alvo inválido %q", base)
		}
	}
}
