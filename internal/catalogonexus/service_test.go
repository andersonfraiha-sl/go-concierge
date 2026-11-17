package catalogonexus

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"go-concierge/internal/mcp"
)

type fakeCaller struct {
	name   string
	args   map[string]any
	result *mcp.CallResult
	err    error
}

func (f *fakeCaller) CallTool(_ context.Context, name string, args map[string]any) (*mcp.CallResult, error) {
	f.name, f.args = name, args
	return f.result, f.err
}

func fixture(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestSearchAdaptaFormatoRealSanitizado(t *testing.T) {
	caller := &fakeCaller{result: &mcp.CallResult{Content: []mcp.Content{{Type: "text", Text: fixture(t, "search_operations.txt")}}}}
	got, err := New(caller).Search(context.Background(), "listar encomendas")
	if err != nil {
		t.Fatal(err)
	}
	if caller.name != ToolSearch || !reflect.DeepEqual(caller.args, map[string]any{"q": "listar encomendas"}) {
		t.Fatalf("chamada MCP inesperada: %s %#v", caller.name, caller.args)
	}
	if len(got) != 2 || got[0].ServiceID != "11111111-1111-4111-8111-111111111111" || got[0].OperationID != "FetchUnitPackages" || got[0].Method != "GET" {
		t.Fatalf("resultados inesperados: %#v", got)
	}
}

func TestGetOperationAdaptaFormatoRealSanitizado(t *testing.T) {
	caller := &fakeCaller{result: &mcp.CallResult{Content: []mcp.Content{{Type: "text", Text: fixture(t, "get_operation.txt")}}}}
	got, err := New(caller).GetOperation(context.Background(), "service-1", "FetchUnitPackages")
	if err != nil {
		t.Fatal(err)
	}
	if caller.name != ToolDetail || !reflect.DeepEqual(caller.args, map[string]any{"serviceId": "service-1", "operationId": "FetchUnitPackages"}) {
		t.Fatalf("chamada MCP inesperada: %s %#v", caller.name, caller.args)
	}
	if got.Method != "GET" || got.Path != "/v2/units/{unitId}/packages" || got.URL != "https://api.example.test/v2/units/{unitId}/packages" || len(got.Params) != 2 {
		t.Fatalf("detalhes inesperados: %#v", got)
	}
	if got.Params[1].Type != "integer" || !got.Params[0].Required || got.Definition == "" || !strings.Contains(got.Evidence, "Auth: não informado") {
		t.Fatalf("parâmetros/definição incompletos: %#v", got)
	}
}

func TestSearchAceitaStructuredContentIdentificavel(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{"operations": []any{map[string]any{
		"method": "GET", "path": "/v1/items", "operationId": "ListItems", "title": "Listar itens", "serviceId": "svc-1",
	}}})
	caller := &fakeCaller{result: &mcp.CallResult{StructuredContent: raw}}
	got, err := New(caller).Search(context.Background(), "listar itens")
	if err != nil || len(got) != 1 || got[0].ServiceID != "svc-1" {
		t.Fatalf("resposta estruturada inesperada: %#v, %v", got, err)
	}
}

func TestFormatoDesconhecidoViraErroLegivel(t *testing.T) {
	caller := &fakeCaller{result: &mcp.CallResult{Content: []mcp.Content{{Type: "text", Text: "resposta sem formato compreensível"}}}}
	_, err := New(caller).Search(context.Background(), "itens")
	if err == nil {
		t.Fatal("esperava erro de formato")
	}
	caller.result = &mcp.CallResult{Content: []mcp.Content{{Type: "text", Text: "detalhe sem definição"}}}
	_, err = New(caller).GetOperation(context.Background(), "svc", "op")
	if err == nil {
		t.Fatal("esperava erro de detalhes")
	}
}

func TestErrosMCPNaoDevolvemMensagensCruas(t *testing.T) {
	caller := &fakeCaller{err: errors.New("Authorization Bearer segredo")}
	_, err := New(caller).Search(context.Background(), "itens")
	if err == nil || err.Error() == caller.err.Error() {
		t.Fatalf("erro externo deveria ser traduzido: %v", err)
	}
}

func TestSearchNaoPermiteConsultaVazia(t *testing.T) {
	_, err := New(&fakeCaller{}).Search(context.Background(), " ")
	if err == nil {
		t.Fatal("esperava erro de validação")
	}
}

func TestNewMCPCallerUsaHostRealDaURL(t *testing.T) {
	caller, err := NewMCPCaller("https://catalog.example.test/mcp", "token-de-teste")
	if err != nil {
		t.Fatal(err)
	}
	client := caller.(*serializedCaller).client
	if client.HostHeader != "catalog.example.test" || client.URL != "https://catalog.example.test/mcp" {
		t.Fatalf("host do MCP não acompanha o destino: host=%q url=%q", client.HostHeader, client.URL)
	}
	if _, err := NewMCPCaller(DefaultURL, " "); err == nil {
		t.Fatal("token vazio foi aceito")
	}
}
