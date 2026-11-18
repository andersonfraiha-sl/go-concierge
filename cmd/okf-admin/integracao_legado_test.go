package main

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func chamarIntegracao(amb ambienteRotas, metodo, url, corpo string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(metodo, url, strings.NewReader(corpo))
	r.Header.Set("Authorization", "Bearer "+chaveDeTeste)
	w := httptest.NewRecorder()
	amb.handler.ServeHTTP(w, r)
	return w
}

func TestIntegracaoLegadaDeMapeamentoNaoEstaExpostaNemAlteraDados(t *testing.T) {
	amb := novoAmbienteRotas(t)
	antes, err := amb.repo.Ler(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{"", "?company_id=adm-a", "?administradora=adm-b"} {
		w := chamarIntegracao(amb, http.MethodPut, "/integracao/v1/mapeamento"+query, `{"administradoras":[{"id":"outra"}],"remover":{"condominios":["101"]}}`)
		if w.Code != http.StatusNotFound && w.Code != http.StatusMethodNotAllowed {
			t.Fatalf("mapeamento legado respondeu %d: %s", w.Code, w.Body.String())
		}
	}
	depois, err := amb.repo.Ler(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(antes, depois) {
		t.Fatal("requisição ao endpoint retirado alterou dados")
	}
}
