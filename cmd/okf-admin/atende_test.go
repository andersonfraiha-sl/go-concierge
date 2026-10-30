package main

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"go-concierge/internal/atende"
	"go-concierge/internal/gruvi"
	"go-concierge/internal/harness"
	"go-concierge/internal/mcp"
)

type tokenIdentidadeFake struct{}

func (tokenIdentidadeFake) Token(context.Context) (string, error) { return "token", nil }

type mcpIdentidadeFake struct {
	chamadas int
	args     map[string]any
}

func (f *mcpIdentidadeFake) ValidarToken(context.Context) error { return nil }

func (f *mcpIdentidadeFake) CallTool(_ context.Context, nome string, args map[string]any) (*mcp.CallResult, error) {
	f.chamadas++
	f.args = args
	if nome != "gruvi.bff.building-operation.get-company-by-document-controller-handle" {
		return nil, context.Canceled
	}
	return &mcp.CallResult{StructuredContent: json.RawMessage(`{"data":{"name":"Residencial Exemplo","condominium":{"name":"Cond A"},"units":[{"id":"uuid-109","id_fastpass":"fp-109","unit":"109","block":"Jataí","resident":{"name":"Vanderlei Martins","mobile_number":"5519981621916"}}]}}`)}, nil
}

func TestBotaoIdentidadeDevolveContratoDaConsole(t *testing.T) {
	fake := &mcpIdentidadeFake{}
	s := &servidor{
		gruvi:   gruvi.New("", "5982", "5519981621916", tokenIdentidadeFake{}),
		gruviOK: true,
		atende:  &atende.Servico{Config: harness.Config{MCP: fake}},
	}
	r := httptest.NewRequest("GET", "/api/atende/identidade?phone=19+98162-1916", nil)
	w := httptest.NewRecorder()
	s.atendeIdentidade(w, r)
	if w.Code != 200 {
		t.Fatalf("status HTTP %d: %s", w.Code, w.Body.String())
	}
	var body struct {
		Configurado bool             `json:"configurado"`
		Erro        string           `json:"erro"`
		Identidade  gruvi.Identidade `json:"identidade"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	id := body.Identidade
	if !body.Configurado || body.Erro != "" || !id.Authenticated || id.Phone != "5519981621916" || id.CompanyID != "5982" || len(id.Units) != 1 || len(id.Condominios) != 1 {
		t.Fatalf("resposta para a console: %+v", body)
	}
	if id.Units[0].ID != "fp-109" || id.Units[0].IDInterno != "uuid-109" || id.Units[0].Label != "Jataí - 109" {
		t.Fatalf("unidade deve expor o ID operacional separado do interno: %+v", id.Units[0])
	}
	if fake.chamadas != 1 || fake.args["companyId"] != "5982" || fake.args["filter_mobile_number"] != "5519981621916" {
		t.Fatalf("chamada MCP filtrada pelo telefone B2C: %+v", fake)
	}
}
