package agentes

import (
	"errors"
	"testing"

	"go-concierge/internal/recurso"
)

func TestValorMCPSeparaRespostaIncertaRecusaESucesso(t *testing.T) {
	const nome = "gruvi.bff.building-operation.create-visit-controller-handle"
	_, err := valorMCP(nome, &recurso.Resultado{Valor: map[string]any{"visit": map[string]any{"clearance_status": "PENDING"}}})
	var incerto *resultadoMCPIncerto
	if !errors.As(err, &incerto) {
		t.Fatalf("status sem confirmação de autorização devia ser incerto, veio %v", err)
	}

	_, err = valorMCP(nome, &recurso.Resultado{Falhou: true, Texto: "recusada por regra funcional"})
	if err == nil || errors.As(err, &incerto) {
		t.Fatalf("erro funcional explícito deve continuar distinto da incerteza, veio %v", err)
	}

	resultado, err := valorMCP(nome, &recurso.Resultado{Valor: map[string]any{"visit": map[string]any{"clearance_status": "AUTHORIZED"}}})
	if err != nil || resultado == nil {
		t.Fatalf("autorização confirmada não deve ser tratada como falha: resultado=%v erro=%v", resultado, err)
	}
}
