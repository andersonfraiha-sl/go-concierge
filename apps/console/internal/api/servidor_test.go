package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go-concierge/internal/httpapp"
)

func TestConsoleValidaEntregaAntesDeArmazenar(t *testing.T) {
	const body = `{"protocol":"123","session_id":"sessao-a","message":"resposta do concierge"}`
	for _, status := range []int{204, 401, 403, 404, 500} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			emulador := NewEmulador()
			nucleo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/atende/sessao" {
					httpapp.JSON(w, 200, map[string]any{"sessao": map[string]string{"id": "sessao-a"}, "reply_emulada": true})
					return
				}
				if r.Method != http.MethodPost || r.URL.RequestURI() != "/api/atende/entrega-emulada/validar?company_id=empresa-a" || r.Header.Get("Authorization") != "Bearer exemplo" || r.Header.Get("X-Dev-Usuario") != "operador" {
					t.Errorf("validação sem contexto: %s %v", r.URL, r.Header)
				}
				w.WriteHeader(status)
			}))
			defer nucleo.Close()
			h, err := novo(httpapp.Config{ConciergeURL: nucleo.URL}, emulador)
			if err != nil {
				t.Fatal(err)
			}
			r := httptest.NewRequest(http.MethodPost, "/emulador/atende/messages?company_id=empresa-a", strings.NewReader(body))
			r.Header.Set("Authorization", "Bearer exemplo")
			r.Header.Set("X-Dev-Usuario", "operador")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			quer := status
			quantidade := 0
			if status == 204 {
				quer = 200
				quantidade = 1
			}
			if w.Code != quer || len(emulador.Recebidas("sessao-a")) != quantidade {
				t.Fatalf("entrega: %d %s", w.Code, w.Body.String())
			}
			w = httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/atende/sessao?id=sessao-a", nil))
			var resp struct {
				Recebidas []Entregue `json:"recebidas"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatal(err)
			}
			if w.Code != 200 || len(resp.Recebidas) != quantidade {
				t.Fatalf("consulta: %d %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestConsoleNaoRecebeEntregaComNucleoIndisponivel(t *testing.T) {
	emulador := NewEmulador()
	nucleo := httptest.NewServer(http.NotFoundHandler())
	nucleo.Close()
	h, err := novo(httpapp.Config{ConciergeURL: nucleo.URL}, emulador)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/emulador/atende/messages", strings.NewReader(`{"protocol":"123","message":"texto"}`)))
	if w.Code != 502 || len(emulador.Recebidas("123")) != 0 {
		t.Fatalf("entrega sem validação: %d", w.Code)
	}
}

func TestConsolePreservaAcoesSemSessao(t *testing.T) {
	for _, corpo := range []string{`{"pedido":{"id":"pedido-a"}}`, `{"removido":"pedido-a"}`} {
		nucleo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(corpo))
		}))
		h, err := Novo(httpapp.Config{ConciergeURL: nucleo.URL})
		if err != nil {
			nucleo.Close()
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/pedidos/acao", strings.NewReader(`{}`)))
		nucleo.Close()
		if w.Code != 200 || w.Body.String() != corpo {
			t.Errorf("ação alterada: %d %s", w.Code, w.Body.String())
		}
	}
}
