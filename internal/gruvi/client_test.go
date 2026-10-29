package gruvi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type tokenTeste struct{ chamadas int }

func (t *tokenTeste) Token(context.Context) (string, error) {
	t.chamadas++
	return "tok-1", nil
}

func TestTokenEIdentidade(t *testing.T) {
	token := &tokenTeste{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/bff/building-operation/companies/5982"):
			if r.Header.Get("Authorization") != "Bearer tok-1" {
				http.Error(w, "no token", 401)
				return
			}
			if r.URL.Query().Get("filter[mobile-number]") != "5519981621916" {
				http.Error(w, "phone", 400)
				return
			}
			w.Write([]byte(`{"data":{"name":"Residencial Exemplo","units":[{"id_fastpass":"fp-1","unit":"101","block":"A","resident":{"name":"Vanderlei Martins"}},{"unit":"sem fastpass"}]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := New(srv.URL+"/bff", "5982", "5519981621916", token)
	id, err := c.Identidade(context.Background(), "19 98162-1916")
	if err != nil {
		t.Fatal(err)
	}
	if !id.Authenticated || id.Name != "Vanderlei Martins" || id.CompanyName != "Residencial Exemplo" || len(id.Units) != 1 || id.Units[0].ID != "fp-1" || id.Units[0].Label != "A - 101" {
		t.Fatalf("identidade: %+v", id)
	}
	if _, err := c.Identidade(context.Background(), "5519981621916"); err != nil {
		t.Fatal(err)
	}
	if token.chamadas != 2 {
		t.Fatalf("deveria solicitar o token compartilhado a cada requisição, pediu %d vezes", token.chamadas)
	}
	if _, err := c.Identidade(context.Background(), "5511999999999"); err == nil || token.chamadas != 2 {
		t.Fatal("outro telefone deve ser recusado antes de obter o token")
	}
	if New("", "", "", nil).Configurado() {
		t.Fatal("sem credencial não está configurado")
	}
	if NormalizarTelefone("(19) 8162-1916") != "5519981621916" {
		t.Fatal(NormalizarTelefone("(19) 8162-1916"))
	}
}

func TestIdentidadeComListaDeCondominios(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":[
		  {"condominium":{"name":"Cond A"},"units":[{"id":"uuid-1","id_external_gruvi":"op"},{"id":"uuid-2","id_fastpass":109,"block":"Jataí","unit":"Unidade Operacao Predial","resident":{"name":"Vanderlei","documents":[{"type":"MOBILE_NUMBER","value":"5519981621916"}]}}],
		   "legal_responsibles":[{"name":"Vand","position":{"name":"Síndico"},"documents":[{"type":"MOBILE_NUMBER","value":"5519981621916"}]}]},
		  {"condominium":{"name":"Cond B"},"units":[{"id_fastpass":"77","unit":"12","block":"T1"}]}
		]}`))
	}))
	defer srv.Close()
	c := New(srv.URL, "5982", "5519981621916", &tokenTeste{})
	id, err := c.Identidade(context.Background(), "5519981621916")
	if err != nil {
		t.Fatal(err)
	}
	if !id.Authenticated || id.Name != "Vanderlei" || id.Role != "sindico" {
		t.Fatalf("identidade: %+v", id)
	}
	if len(id.Condominios) != 2 || id.Condominios[0].Nome != "Cond A" || len(id.Condominios[0].Unidades) != 2 || len(id.Condominios[1].Unidades) != 1 {
		t.Fatalf("condomínios: %+v", id.Condominios)
	}
	if id.Units[0].ID != "uuid-1" || id.Units[0].Label != "unidade op" || id.Units[1].ID != "109" || id.Units[1].Condominio != "Cond A" || id.Units[2].Condominio != "Cond B" {
		t.Fatalf("unidades: %+v", id.Units)
	}
}
