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
	chamadas    []string
	args        map[string]any
	argsPorNome map[string]map[string]any
	erro        bool
	resultados  map[string]*mcp.CallResult
}

func (m *mcpFalso) CallTool(_ context.Context, name string, args map[string]any) (*mcp.CallResult, error) {
	m.chamadas = append(m.chamadas, name)
	m.args = args
	if m.argsPorNome == nil {
		m.argsPorNome = map[string]map[string]any{}
	}
	m.argsPorNome[name] = args
	if m.erro {
		return &mcp.CallResult{IsError: true, Content: []mcp.Content{{Type: "text", Text: "Unauthorized: token expirado"}}}, nil
	}
	if res, ok := m.resultados[name]; ok {
		return res, nil
	}
	return &mcp.CallResult{Content: []mcp.Content{{Type: "text", Text: `{"unidades":[{"id":"109","label":"Jataí"}]}`}}}, nil
}

func TestToolsObrigatoriasExecutadasPorMCPFalso(t *testing.T) {
	const (
		empresa = "gruvi.bff.building-operation.get-company-by-document-controller-handle"
		pacotes = "gruvi.bff.building-operation.fetch-unit-packages-controller-handle"
		codigo  = "gruvi.bff.building-operation.get-package-delivery-code-controller-handle"
		motivos = "gruvi.bff.building-operation.fetch-visit-reasons-controller-handle"
		visita  = "gruvi.bff.building-operation.create-visit-controller-handle"
	)
	cat, a := catalogoComMCP(t)
	fake := &mcpFalso{resultados: map[string]*mcp.CallResult{
		pacotes: {Content: []mcp.Content{{Type: "text", Text: `{"data":{"packages":[{"id":42,"description":"Caixa"}]}}`}}},
		codigo:  {Content: []mcp.Content{{Type: "text", Text: `{"data":{"delivery_code":"4831"}}`}}},
		motivos: {Content: []mcp.Content{{Type: "text", Text: `{"data":{"reasons":[{"id":5,"reason":"Visita","type_provider":"GUEST"}]}}`}}},
		visita:  {Content: []mcp.Content{{Type: "text", Text: `{"data":{"visit":{"clearance_status":"AUTHORIZED"}}}`}}},
	}}
	telefoneMorador := "5519981621916"
	payloads := map[string]map[string]any{
		empresa: {"companyId": "5982", "filter_mobile_number": telefoneMorador},
		pacotes: {"companyId": "5982", "filter_mobile_number": telefoneMorador},
		codigo:  {"companyId": "5982", "filter_mobile_number": telefoneMorador},
		motivos: {"companyId": "5982"},
		visita:  {"companyId": "5982", "resident": map[string]any{"mobile_number": telefoneMorador}},
	}
	ag, err := Build(context.Background(), cat, a, Config{Mock: true, Env: semEnv, MCP: fake, Payloads: payloads, Identidade: Identidade{Authenticated: true, AccountID: "cond-001", Role: "morador"}})
	if err != nil {
		t.Fatal(err)
	}

	casos := []struct {
		agente, nome string
		params       map[string]any
		confirmacao  string
	}{
		{"encomendas", empresa, map[string]any{"companyId": "9999", "filter_mobile_number": "000", "filter": map[string]any{"mobile-number": "000"}}, ""},
		{"encomendas", pacotes, map[string]any{"companyId": "9999", "unitId": 101, "filter_mobile_number": "000", "filter_status": "waiting"}, ""},
		{"codigo-retirada", codigo, map[string]any{"companyId": "9999", "filter_mobile_number": "000", "unitId": 101, "packageId": 42}, ""},
		{"liberacao-visitante", motivos, map[string]any{"companyId": "9999", "filter_visitor_type": "GUEST"}, ""},
	}
	for _, caso := range casos {
		sub := ag.Subagentes[caso.agente]
		out, err := sub.InvocarRecurso(context.Background(), caso.nome, caso.params, caso.confirmacao)
		if err != nil || out["error"] != nil || out["origem"] != "mcp" {
			t.Fatalf("%s via MCP: resultado=%v erro=%v", caso.nome, out, err)
		}
	}
	quer := []string{empresa, pacotes, codigo, motivos}
	if strings.Join(fake.chamadas, ",") != strings.Join(quer, ",") {
		t.Fatalf("tools MCP chamadas: %v", fake.chamadas)
	}
	if fake.argsPorNome[empresa]["companyId"] != "5982" || fake.argsPorNome[pacotes]["companyId"] != "5982" || fake.argsPorNome[codigo]["companyId"] != "5982" || fake.argsPorNome[motivos]["companyId"] != "5982" || fake.argsPorNome[pacotes]["unitId"] != 101 || fake.argsPorNome[codigo]["packageId"] != 42 || fake.argsPorNome[motivos]["filter_visitor_type"] != "GUEST" {
		t.Fatalf("parâmetros MCP: %+v", fake.argsPorNome)
	}
	for _, nome := range []string{empresa, pacotes, codigo} {
		if fake.argsPorNome[nome]["filter_mobile_number"] != telefoneMorador {
			t.Fatalf("telefone do morador foi sobrescrito em %s: %+v", nome, fake.argsPorNome[nome])
		}
	}
	filtro := fake.argsPorNome[empresa]["filter"].(map[string]any)
	if filtro["mobile-number"] != telefoneMorador {
		t.Fatalf("telefone no filtro aninhado foi sobrescrito: %+v", filtro)
	}
	resultadoVisita, errOut := ag.Subagentes["liberacao-visitante"].InvocarRecurso(context.Background(), visita, map[string]any{"unitId": 101, "reason": 5, "resident": map[string]any{"mobile_number": "000", "name": "Morador"}, "visitors": []any{map[string]any{"name": "Ana"}}}, "sim")
	if errOut != nil || !strings.Contains(erroDe(resultadoVisita), "motor determinístico") {
		t.Fatalf("escrita de visita por chamada direta deve ser bloqueada: %+v erro=%v", resultadoVisita, errOut)
	}
	if fake.argsPorNome["gruvi.bff.building-operation.create-visit-controller-handle"] != nil {
		t.Fatal("a chamada direta não deveria chegar ao MCP")
	}
}

func catalogoComMCP(t *testing.T) (*okf.Catalogo, *okf.Agent) {
	t.Helper()
	cat, a := catalogoExemplo(t)
	original := cat.Recursos
	cat.Recursos = func() []okf.Recurso {
		out := original()
		for _, nome := range []string{
			"gruvi.bff.building-operation.get-company-by-document-controller-handle",
			"gruvi.bff.building-operation.fetch-unit-packages-controller-handle",
			"gruvi.bff.building-operation.get-package-delivery-code-controller-handle",
			"gruvi.bff.building-operation.fetch-visit-reasons-controller-handle",
			"gruvi.bff.building-operation.create-visit-controller-handle",
		} {
			out = append(out, okf.Recurso{Name: nome, Kind: okf.KindAPI, Visibility: "private", Origem: okf.OrigemMCP})
		}
		for i := range out {
			if strings.HasSuffix(out[i].Name, "create-visit-controller-handle") {
				out[i].Write = true
			}
		}
		return out
	}
	return cat, a
}

// Um recurso que veio do catálogo MCP (Origem mcp) é executado por tools/call, com o
// payload do condomínio como parâmetro padrão; erro do servidor vira {"error"}; sem
// cliente MCP configurado, o recurso fica indisponível em vez de cair no Nexus.
func TestChamarRecursoDoCatalogoMCP(t *testing.T) {
	cat, a := catalogoComMCP(t)
	fake := &mcpFalso{}
	ident := Identidade{Authenticated: true, AccountID: "cond-001", Role: "morador"}
	ag, err := Build(context.Background(), cat, a, Config{Mock: true, Env: semEnv, MCP: fake, Identidade: ident,
		Payloads: map[string]map[string]any{"unidades.listar": {"companyId": 5982, "filter_mobile_number": "5519981621916"}}})
	if err != nil {
		t.Fatal(err)
	}
	reservas := ag.Subagentes["reserva-area-comum"]
	out := chama(t, reservas, "chamar_recurso", map[string]any{"nome": "unidades.listar", "params": map[string]any{"filter": "x"}})
	if erroDe(out) != "" || out["origem"] != "mcp" {
		t.Fatalf("esperava execução pelo MCP; veio %v", out)
	}
	if len(fake.chamadas) != 1 || fake.chamadas[0] != "gruvi.bff.building-operation.get-company-by-document-controller-handle" || fmt.Sprint(fake.args["companyId"]) != "5982" || fake.args["filter"] != "x" {
		t.Fatalf("tools/call com os params + payload do condomínio; veio %v %v", fake.chamadas, fake.args)
	}
	res, _ := out["resultado"].(map[string]any)
	if _, ok := res["unidades"]; !ok {
		t.Fatalf("o texto JSON do resultado devia virar dado: %v", out["resultado"])
	}

	fake.erro = true
	out = chama(t, reservas, "chamar_recurso", map[string]any{"nome": "unidades.listar"})
	if !strings.Contains(erroDe(out), "recusou") || !strings.Contains(erroDe(out), "token expirado") {
		t.Fatalf("isError do MCP devia virar erro de uso; veio %v", out)
	}

	ag2, err := Build(context.Background(), cat, a, Config{Mock: true, Env: semEnv, Identidade: ident}) // sem MCP
	if err != nil {
		t.Fatal(err)
	}
	out = chama(t, ag2.Subagentes["reserva-area-comum"], "chamar_recurso", map[string]any{"nome": "unidades.listar"})
	if !strings.Contains(erroDe(out), "indisponível") {
		t.Fatalf("sem cliente MCP o recurso do catálogo MCP fica indisponível; veio %v", out)
	}
}
