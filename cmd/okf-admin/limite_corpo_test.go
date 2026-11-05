package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go-concierge/internal/okf"
)

// corpoJSONDeTamanho monta {"<campo>":"xxx..."} com exatamente n bytes: o valor JSON só
// fecha no último byte, então o decoder precisa ler tudo.
func corpoJSONDeTamanho(t *testing.T, campo string, n int) string {
	t.Helper()
	moldura := `{"` + campo + `":""}`
	if n < len(moldura) {
		t.Fatalf("tamanho %d menor que a moldura %q", n, moldura)
	}
	return `{"` + campo + `":"` + strings.Repeat("x", n-len(moldura)) + `"}`
}

func TestCorpoAcimaDoLimiteRecebe413(t *testing.T) {
	s := &servidor{cat: &okf.Catalogo{Root: t.TempDir()}, log: log.New(io.Discard, "", 0)}
	rotas := []struct {
		nome     string
		metodo   string
		caminho  string
		campo    string
		limite   int
		handler  http.HandlerFunc
		erroNoOK string // o erro do próprio handler quando o corpo passa inteiro
	}{
		{nome: "arquivo", metodo: http.MethodPut, caminho: "/api/arquivo", campo: "conteudo", limite: limiteCorpoArquivo, handler: s.salvarArquivo, erroNoOK: "caminho inválido"},
		{nome: "orquestração", metodo: http.MethodPut, caminho: "/api/admin/orquestracao", campo: "modo", limite: limiteCorpoJSON, handler: s.adminSalvarOrquestracao, erroNoOK: "modo deve ser"},
	}
	for _, rota := range rotas {
		casos := []struct {
			nome       string
			tamanho    int
			querStatus int
		}{
			{nome: "no limite passa", tamanho: rota.limite, querStatus: http.StatusBadRequest},
			{nome: "limite+1 recusa", tamanho: rota.limite + 1, querStatus: http.StatusRequestEntityTooLarge},
		}
		for _, c := range casos {
			t.Run(rota.nome+"/"+c.nome, func(t *testing.T) {
				corpo := corpoJSONDeTamanho(t, rota.campo, c.tamanho)
				w := httptest.NewRecorder()
				rota.handler(w, httptest.NewRequest(rota.metodo, rota.caminho, strings.NewReader(corpo)))

				var resposta map[string]string
				if err := json.Unmarshal(w.Body.Bytes(), &resposta); err != nil {
					t.Fatalf("resposta não é JSON: %v (%q)", err, w.Body.String())
				}
				if w.Code != c.querStatus {
					t.Fatalf("status %d, quer %d: %v", w.Code, c.querStatus, resposta)
				}
				if c.querStatus == http.StatusBadRequest && !strings.Contains(resposta["error"], rota.erroNoOK) {
					t.Fatalf("o corpo não chegou inteiro ao handler: %v", resposta)
				}
			})
		}
	}
}

func TestEspiarCorpoRecusaCorpoAcimaDoLimite(t *testing.T) {
	casos := []struct {
		nome       string
		tamanho    int
		querStatus int // 0: espiou sem erro
	}{
		{nome: "no limite passa", tamanho: limiteCorpoJSON},
		{nome: "limite+1 recusa", tamanho: limiteCorpoJSON + 1, querStatus: http.StatusRequestEntityTooLarge},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPut, "/api/admin/habilitacao", strings.NewReader(corpoJSONDeTamanho(t, "condominio", c.tamanho)))
			var corpo struct {
				Condominio string `json:"condominio"`
			}
			err := espiarCorpo(r, &corpo)
			if c.querStatus == 0 {
				if err != nil {
					t.Fatalf("espiar: %v", err)
				}
				return
			}
			e, ok := errors.AsType[erroDeAlvo](err)
			if !ok || e.status != c.querStatus {
				t.Fatalf("erro %v, quer status %d", err, c.querStatus)
			}
		})
	}
}
