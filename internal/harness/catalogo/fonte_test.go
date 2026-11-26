package catalogo_test

import (
	"context"
	"testing"

	"go-concierge/internal/harness/catalogo"
	"go-concierge/internal/harness/provedores/mcp"
)

type dadosDaFonteTeste struct{ retrato catalogo.Retrato }

func (d dadosDaFonteTeste) CatalogoDoEscopo(context.Context, string, *int) (catalogo.Retrato, error) {
	return d.retrato, nil
}

type mcpDaFonteTeste struct{}

func (mcpDaFonteTeste) ListTools(context.Context) ([]mcp.Tool, error) {
	return []mcp.Tool{{Name: "operacao", Description: "via MCP"}}, nil
}

func TestFonteCompoeFerramentasMCPComRecursosDaConta(t *testing.T) {
	retrato := catalogo.Retrato{Agente: "agentes/raiz", Conta: "conta", Arquivos: map[string][]byte{
		"agentes/raiz/agent.yaml": []byte("nome: Raiz\ntipo: operador\n"),
		"nexus/conta.yaml":        []byte("recursos:\n  - name: operacao\n    kind: api\n    description: via fixture\n  - name: outra\n    kind: api\n"),
	}}
	fonte := &catalogo.Fonte{Dados: dadosDaFonteTeste{retrato}, MCP: mcpDaFonteTeste{}}
	cat, err := fonte.Carregar(t.Context(), "empresa", nil)
	if err != nil {
		t.Fatal(err)
	}
	recursos := cat.RecursosNexus()
	if len(recursos) != 2 || recursos[0].Name != "operacao" || recursos[0].Description != "via MCP" || recursos[1].Name != "outra" {
		t.Fatalf("recursos não compostos: %+v", recursos)
	}
}
