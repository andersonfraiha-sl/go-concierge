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
	if id.Units[0].ID != "uuid-1" || id.Units[0].Label != "unidade op" || id.Units[1].ID != "109" || id.Units[1].IDInterno != "uuid-2" || id.Units[1].Condominio != "Cond A" || id.Units[2].Condominio != "Cond B" {
		t.Fatalf("unidades: %+v", id.Units)
	}
}

func TestIdentidadeDoResultadoMCPPreservaContratoEValidaTelefone(t *testing.T) {
	phone := "19 98162-1916"
	data := map[string]any{
		"name":        "Residencial Exemplo",
		"condominium": map[string]any{"name": "Cond A"},
		"units": []any{
			map[string]any{"id": "uuid-2", "id_fastpass": float64(109), "unit": "101", "block": "A", "resident": map[string]any{"name": "Vanderlei Martins", "mobile_number": phone}},
			map[string]any{"id": "uuid-3", "id_fastpass": "fp-outro", "unit": "202", "block": "B", "resident": map[string]any{"name": "Outra pessoa", "mobile_number": "5511999999999"}},
		},
	}
	id := IdentidadeDoResultadoMCP(phone, "5982", data)
	if !id.Authenticated || id.Phone != "5519981621916" || id.CompanyID != "5982" || id.CompanyName != "Cond A" || id.Name != "Vanderlei Martins" || len(id.Units) != 1 || len(id.Condominios) != 1 {
		t.Fatalf("identidade MCP: %+v", id)
	}
	unit := id.Units[0]
	if unit.ID != "109" || unit.IDInterno != "uuid-2" || unit.Label != "A - 101" || id.Condominios[0].Unidades[0].ID != "109" {
		t.Fatalf("IDs e contrato da console: %+v", unit)
	}
}

func TestIdentidadeMCPSemVinculoTelefoneFicaPublica(t *testing.T) {
	id := IdentidadeDoResultadoMCP("5519981621916", "5982", map[string]any{
		"units": []any{map[string]any{"id": "uuid-2", "id_fastpass": "fp-1", "unit": "101"}},
	})
	if id.Authenticated || len(id.Units) != 0 || len(id.Condominios) != 0 {
		t.Fatalf("unidade sem vínculo não autentica nem aparece: %+v", id)
	}
}

func TestCacheVinculadoPreservaUnidadesDeResponsavelLegalSemDadosDeContato(t *testing.T) {
	id, cache := IdentidadeComDadosVinculadosMCP("5519981621916", "5982", map[string]any{
		"name": "Residencial Exemplo",
		"legal_responsibles": []any{map[string]any{
			"name":      "Vanderlei Martins",
			"position":  map[string]any{"name": "Síndico"},
			"documents": []any{map[string]any{"type": "MOBILE_NUMBER", "value": "5519981621916"}},
		}},
		"units": []any{
			map[string]any{"id": "u1", "id_fastpass": "101", "unit": "A - 101", "resident": map[string]any{"name": "Morador A", "mobile_number": "5511999999999"}},
			map[string]any{"id": "u2", "id_fastpass": "202", "unit": "B - 202", "resident": map[string]any{"name": "Morador B", "documents": []any{map[string]any{"type": "MOBILE_NUMBER", "value": "5511999999998"}}}},
		},
	})
	if !id.Authenticated || id.Role != "sindico" || len(id.Units) != 2 {
		t.Fatalf("responsável legal verificado deve manter acesso às unidades do condomínio: %+v", id)
	}
	units, ok := cache["units"].([]any)
	if !ok || len(units) != 2 {
		t.Fatalf("cache deve manter as unidades do responsável legal verificado: %+v", cache)
	}
	for _, unit := range units {
		resident := unit.(map[string]any)["resident"].(map[string]any)
		if _, exists := resident["mobile_number"]; exists {
			t.Fatalf("cache não deve guardar telefone residencial: %+v", resident)
		}
		if _, exists := resident["documents"]; exists {
			t.Fatalf("cache não deve guardar documentos: %+v", resident)
		}
	}
}
