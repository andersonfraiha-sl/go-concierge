package mcp

import (
	"encoding/json"
	"testing"
)

func TestConverterResultado(t *testing.T) {
	if ConverterResultado(nil) != nil {
		t.Fatal("resposta ausente deve permanecer ausente")
	}
	r := ConverterResultado(&CallResult{StructuredContent: json.RawMessage(`{"id":42}`), Content: []Content{{Type: "text", Text: "recusado"}}, IsError: true})
	valor, ok := r.Valor.(map[string]any)
	if !ok || valor["id"] != float64(42) || r.Texto != "recusado" || !r.Falhou {
		t.Fatalf("resultado inesperado: %#v", r)
	}
}
