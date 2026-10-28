package harness

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"go-concierge/internal/mcp"
	"go-concierge/internal/okf"
)

// mcpFalso imita o cliente MCP: grava a chamada e devolve um resultado em texto JSON.
type mcpFalso struct {
	chamadas []string
	args     map[string]any
	erro     bool
}

func (m *mcpFalso) CallTool(_ context.Context, name string, args map[string]any) (*mcp.CallResult, error) {
	m.chamadas = append(m.chamadas, name)
	m.args = args
	if m.erro {
		return &mcp.CallResult{IsError: true, Content: []mcp.Content{{Type: "text", Text: "Unauthorized: token expirado"}}}, nil
	}
	return &mcp.CallResult{Content: []mcp.Content{{Type: "text", Text: `{"unidades":[{"id":"109","label":"Jataí"}]}`}}}, nil
}

// Um recurso que veio do catálogo MCP (Origem mcp) é executado por tools/call, com o
// payload do condomínio como parâmetro padrão; erro do servidor vira {"error"}; sem
// cliente MCP configurado, o recurso fica indisponível em vez de cair no Nexus.
func TestChamarRecursoDoCatalogoMCP(t *testing.T) {
	cat, a := catalogoExemplo(t)
	orig := cat.Recursos
	cat.Recursos = func() []okf.Recurso {
		var out []okf.Recurso
		for _, r := range orig() {
			if r.Name == "unidades.listar" {
				r.Origem = okf.OrigemMCP // como se tivesse vindo do tools/list
			}
			out = append(out, r)
		}
		return out
	}
	fake := &mcpFalso{}
	ident := Identidade{Authenticated: true, AccountID: "cond-001", Role: "morador"}
	ag, err := Build(context.Background(), cat, a, Config{Mock: true, Env: semEnv, MCP: fake, Identidade: ident,
		Payloads: map[string]map[string]any{"unidades.listar": {"companyId": 5982}}})
	if err != nil {
		t.Fatal(err)
	}
	out := chama(t, ag, "chamar_recurso", map[string]any{"nome": "unidades.listar", "params": map[string]any{"filter": "x"}})
	if erroDe(out) != "" || out["origem"] != "mcp" {
		t.Fatalf("esperava execução pelo MCP; veio %v", out)
	}
	if len(fake.chamadas) != 1 || fake.chamadas[0] != "unidades.listar" || fmt.Sprint(fake.args["companyId"]) != "5982" || fake.args["filter"] != "x" {
		t.Fatalf("tools/call com os params + payload do condomínio; veio %v %v", fake.chamadas, fake.args)
	}
	res, _ := out["resultado"].(map[string]any)
	if _, ok := res["unidades"]; !ok {
		t.Fatalf("o texto JSON do resultado devia virar dado: %v", out["resultado"])
	}

	fake.erro = true
	out = chama(t, ag, "chamar_recurso", map[string]any{"nome": "unidades.listar"})
	if !strings.Contains(erroDe(out), "recusou") || !strings.Contains(erroDe(out), "token expirado") {
		t.Fatalf("isError do MCP devia virar erro de uso; veio %v", out)
	}

	ag2, err := Build(context.Background(), cat, a, Config{Mock: true, Env: semEnv, Identidade: ident}) // sem MCP
	if err != nil {
		t.Fatal(err)
	}
	out = chama(t, ag2, "chamar_recurso", map[string]any{"nome": "unidades.listar"})
	if !strings.Contains(erroDe(out), "MCP indisponível") {
		t.Fatalf("sem cliente MCP o recurso do catálogo MCP fica indisponível; veio %v", out)
	}
}
