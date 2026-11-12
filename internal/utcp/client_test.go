package utcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type tokenContextKey struct{}

type contextualToken struct{}

func (contextualToken) Token(ctx context.Context) (string, error) {
	token, _ := ctx.Value(tokenContextKey{}).(string)
	if token == "" {
		return "", errors.New("missing token")
	}
	return token, nil
}

func TestFacadeSDKViaBridgeChamaOperacoesHTTPComQueryETokenPorChamada(t *testing.T) {
	fixture := manualFixture(t)
	var manualCalls atomic.Int32
	var businessCalls atomic.Int32
	var mu sync.Mutex
	tokens := map[string]bool{}
	paths := map[string]bool{}
	tokenByPath := map[string]string{}
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/utcp" {
			manualCalls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(fixture)
			return
		}
		businessCalls.Add(1)
		mu.Lock()
		tokens[r.Header.Get("Authorization")] = true
		paths[r.URL.Path] = true
		tokenByPath[r.URL.Path] = r.Header.Get("Authorization")
		mu.Unlock()
		if r.URL.Query().Get("filter[mobile-number]") != "5519981621916" || r.URL.Query().Get("filter[status]") != "waiting" {
			http.Error(w, "query incorreta", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()
	fixture = replaceManualBase(t, fixture, server.URL)
	client := newClientForTest(t, server.URL+"/utcp", contextualToken{})
	if _, err := client.Descrever(context.Background(), PackagesTool); err != nil {
		t.Fatal(err)
	}
	definition, err := client.Descrever(context.Background(), PackagesTool)
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{"additionalProperties", "pattern", "enum", "filter[status]"} {
		if !strings.Contains(string(definition.Schema), part) {
			t.Errorf("schema bruto perdeu %q: %s", part, definition.Schema)
		}
	}
	if !strings.Contains(string(definition.Schema), "$defs") {
		t.Fatal("schema da fixture não cobre $defs")
	}

	var wg sync.WaitGroup
	for _, call := range []struct{ token, unit string }{{"first-token", "101"}, {"second-token", "202"}} {
		call := call
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := context.WithValue(context.Background(), tokenContextKey{}, call.token)
			_, err := client.Executar(ctx, PackagesTool, map[string]any{"companyId": "5982", "unitId": call.unit, "filter[mobile-number]": "5519981621916", "filter[status]": "waiting"})
			if err != nil {
				t.Errorf("executar: %v", err)
			}
		}()
	}
	wg.Wait()
	mu.Lock()
	defer mu.Unlock()
	if businessCalls.Load() != 2 {
		t.Fatalf("chamadas de negócio = %d, esperado 2", businessCalls.Load())
	}
	if manualCalls.Load() != 1 {
		t.Fatalf("manual consultado %d vezes, esperado uma", manualCalls.Load())
	}
	if !tokens["Bearer first-token"] || !tokens["Bearer second-token"] {
		t.Fatalf("token não foi resolvido por chamada: %v", tokens)
	}
	if !paths["/building-operation/companies/5982/unit/101/packages"] || !paths["/building-operation/companies/5982/unit/202/packages"] {
		t.Fatalf("não chamou URLs individuais: %v", paths)
	}
	if paths["/utcp"] {
		t.Fatal("o SDK chamou o manual como endpoint de negócio")
	}
	if tokenByPath["/building-operation/companies/5982/unit/101/packages"] != "Bearer first-token" || tokenByPath["/building-operation/companies/5982/unit/202/packages"] != "Bearer second-token" {
		t.Fatalf("token cruzou entre chamadas concorrentes: %v", tokenByPath)
	}
}

func TestBridgeValidaSchemaAntesDeHTTPEToleraFalhasDeDescobertaComRetry(t *testing.T) {
	fixture := manualFixture(t)
	var manualCalls atomic.Int32
	var businessCalls atomic.Int32
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/utcp" {
			businessCalls.Add(1)
			http.Error(w, `{"token":"secret"}`, http.StatusUnauthorized)
			return
		}
		if manualCalls.Add(1) == 1 {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(removeTool(t, fixture, PackagesTool))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(replaceManualBase(t, fixture, serverURLFrom(r)))
	}))
	defer server.Close()
	client := newClientForTest(t, server.URL+"/utcp", contextualToken{})
	if _, err := client.Descrever(context.Background(), CompanyTool); err == nil {
		t.Fatal("registro parcial devia falhar")
	}
	if _, err := client.Descrever(context.Background(), CompanyTool); err != nil {
		t.Fatalf("retry após falha parcial: %v", err)
	}
	ctx := context.WithValue(context.Background(), tokenContextKey{}, "test-token")
	if _, err := client.Executar(ctx, PackagesTool, map[string]any{"companyId": "5982", "unitId": "1", "filter[mobile-number]": "119981621916", "filter[status]": "waiting"}); err == nil {
		t.Fatal("schema devia rejeitar telefone inválido")
	}
	if businessCalls.Load() != 0 {
		t.Fatalf("argumentos inválidos chegaram ao HTTP: %d", businessCalls.Load())
	}
	if manualCalls.Load() != 2 {
		t.Fatalf("tentativas de descoberta=%d, esperado 2", manualCalls.Load())
	}
}

func TestBridgePropagaCancelamentoTimeoutEEscondeCorpoDeErro(t *testing.T) {
	fixture := manualFixture(t)
	var businessCalls atomic.Int32
	var responseStatus atomic.Int32
	responseStatus.Store(http.StatusUnauthorized)
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/utcp" {
			manual := replaceManualBase(t, fixture, serverURLFrom(r))
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(manual)
			return
		}
		businessCalls.Add(1)
		if responseStatus.Load() == 0 {
			<-r.Context().Done()
			return
		}
		http.Error(w, `{"token":"secret-value"}`, int(responseStatus.Load()))
	}))
	defer server.Close()
	client := newClientForTest(t, server.URL+"/utcp", contextualToken{})
	if _, err := client.Descrever(context.Background(), CompanyTool); err != nil {
		t.Fatal(err)
	}
	args := map[string]any{"companyId": "5982", "filter[mobile-number]": "5519981621916"}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.Executar(canceled, CompanyTool, args); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelamento perdido: %v", err)
	}
	if businessCalls.Load() != 0 {
		t.Fatalf("chamada cancelada chegou ao servidor: %d", businessCalls.Load())
	}
	ctx := context.WithValue(context.Background(), tokenContextKey{}, "test-token")
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusInternalServerError} {
		responseStatus.Store(int32(status))
		_, err := client.Executar(ctx, CompanyTool, args)
		if err == nil || strings.Contains(err.Error(), "secret-value") {
			t.Fatalf("erro HTTP %d expôs corpo ou foi aceito: %v", status, err)
		}
	}
	responseStatus.Store(0)
	timeCtx, stop := context.WithTimeout(ctx, 30*time.Millisecond)
	defer stop()
	if _, err := client.Executar(timeCtx, CompanyTool, args); err == nil {
		t.Fatal("timeout foi ignorado")
	}
	if businessCalls.Load() < 4 {
		t.Fatalf("chamadas esperadas de falha/timeout, vieram %d", businessCalls.Load())
	}
}

func manualFixture(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/manual.json")
	if err != nil {
		t.Fatal(err)
	}
	var manual map[string]any
	if err := json.Unmarshal(data, &manual); err != nil {
		t.Fatal(err)
	}
	tools, ok := manual["tools"].([]any)
	if !ok {
		t.Fatal("fixture sem tools")
	}
	for _, value := range tools {
		tool := value.(map[string]any)
		inputs := tool["inputs"].(map[string]any)
		properties := inputs["properties"].(map[string]any)
		company := properties["companyId"]
		inputs["$defs"] = map[string]any{"companyId": company}
		properties["companyId"] = map[string]any{"$ref": "#/$defs/companyId"}
	}
	encoded, err := json.Marshal(manual)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func replaceManualBase(t *testing.T, fixture []byte, base string) []byte {
	t.Helper()
	return []byte(strings.ReplaceAll(string(fixture), "http://127.0.0.1:9999", base))
}

func removeTool(t *testing.T, fixture []byte, name string) []byte {
	t.Helper()
	var manual map[string]any
	if err := json.Unmarshal(fixture, &manual); err != nil {
		t.Fatal(err)
	}
	var tools []any
	for _, value := range manual["tools"].([]any) {
		if value.(map[string]any)["name"] != name {
			tools = append(tools, value)
		}
	}
	manual["tools"] = tools
	encoded, err := json.Marshal(manual)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func serverURLFrom(r *http.Request) string { return fmt.Sprintf("http://%s", r.Host) }

func newClientForTest(t *testing.T, endpoint string, token TokenProvider) *Client {
	t.Helper()
	client, err := New(endpoint, token)
	if err != nil {
		t.Fatal(err)
	}
	return client
}
