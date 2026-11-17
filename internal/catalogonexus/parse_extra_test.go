package catalogonexus

import (
	"testing"

	"go-concierge/internal/mcp"
)

func TestPesquisaSemResultadosReconhecidos(t *testing.T) {
	caller := &fakeCaller{result: &mcp.CallResult{Content: []mcp.Content{{Type: "text", Text: "0 operação(ões) para \"nada\" (limite 10)."}}}}
	got, err := New(caller).Search(t.Context(), "nada")
	if err != nil || len(got) != 0 {
		t.Fatalf("resultado vazio inesperado: %#v, %v", got, err)
	}
}

func TestDetalhesComMultiplasExposicoesNaoEscolhemURL(t *testing.T) {
	text := `GET /items · ListItems · Listar itens
Como chamar (2 exposição(ões)):
- URL: https://one.example.test/items | gateway: one
- URL: https://two.example.test/items | gateway: two
Definição OpenAPI da operação ($ref resolvidos):
{"parameters":[{"name":"tenant","in":"header","required":true,"schema":{"type":"integer"}}]}
`
	got, ok := parseDetailText(text)
	if !ok || got.URL != "" {
		t.Fatalf("exposição ambígua selecionada: %#v", got)
	}
	if len(got.Params) != 1 || got.Params[0].Type != "integer" || got.Params[0].Description != "header: (inteiro)" {
		t.Fatalf("parâmetro não preservou tipo/localização: %#v", got.Params)
	}
}
