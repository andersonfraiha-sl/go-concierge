package harness

import (
	"encoding/json"
	"errors"
	"testing"

	"go-concierge/internal/mcp"
)

func TestValorMCPSeparaRespostaIncertaRecusaESucesso(t *testing.T) {
	const nome = "gruvi.bff.building-operation.create-visit-controller-handle"
	_, err := valorMCP(nome, &mcp.CallResult{StructuredContent: json.RawMessage(`{"visit":{"clearance_status":"PENDING"}}`)})
	var incerto *resultadoMCPIncerto
	if !errors.As(err, &incerto) {
		t.Fatalf("status sem confirmação de autorização devia ser incerto, veio %v", err)
	}

	_, err = valorMCP(nome, &mcp.CallResult{IsError: true, Content: []mcp.Content{{Type: "text", Text: "recusada por regra funcional"}}})
	if err == nil || errors.As(err, &incerto) {
		t.Fatalf("erro funcional explícito deve continuar distinto da incerteza, veio %v", err)
	}

	resultado, err := valorMCP(nome, &mcp.CallResult{StructuredContent: json.RawMessage(`{"visit":{"clearance_status":"AUTHORIZED"}}`)})
	if err != nil || resultado == nil {
		t.Fatalf("autorização confirmada não deve ser tratada como falha: resultado=%v erro=%v", resultado, err)
	}
}
