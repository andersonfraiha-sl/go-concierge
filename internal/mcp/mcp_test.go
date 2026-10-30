package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go-concierge/internal/okf"
)

// servidorFalso imita o Capability Views da entrega: exige Bearer e Host de loopback,
// inicializa com Mcp-Session-Id, responde tools/list em JSON e tools/call em SSE.
func servidorFalso(t *testing.T, token string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var chamadas atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "método", http.StatusMethodNotAllowed)
			return
		}
		if token != "" && r.Header.Get("Authorization") != "Bearer "+token {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if !strings.HasPrefix(r.Host, "127.0.0.1:") {
			http.Error(w, "Host ou Origin não permitido", http.StatusForbidden)
			return
		}
		body, _ := io.ReadAll(r.Body)
		var req struct {
			ID     *int64         `json:"id"`
			Method string         `json:"method"`
			Params map[string]any `json:"params"`
		}
		_ = json.Unmarshal(body, &req)
		switch req.Method {
		case "initialize":
			w.Header().Set("Mcp-Session-Id", "sess-1")
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":{"protocolVersion":"2025-06-18","serverInfo":{"name":"falso","version":"1"},"capabilities":{"tools":{}}}}`, *req.ID)
		case "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)
		case "tools/list":
			if r.Header.Get("Mcp-Session-Id") != "sess-1" {
				http.Error(w, "sem sessão", http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			if req.Params["cursor"] == nil {
				fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":{"tools":[{"name":"gruvi.bff.building-operation.get-units","description":"Unidades do morador","inputSchema":{"type":"object","properties":{"companyId":{"type":"number","description":"id da company"},"filter":{"type":"object","properties":{"mobile-number":{"type":"string"}},"required":["mobile-number"]}},"required":["companyId"]}}],"nextCursor":"p2"}}`, *req.ID)
				return
			}
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":{"tools":[{"name":"gruvi.bff.building-operation.create-visit","description":"Libera visitante","inputSchema":{"type":"object","properties":{"name":{"type":"string"},"when":{"type":"string","enum":["today","tomorrow"]}}},"annotations":{"readOnlyHint":false}},{"name":"gruvi.bff.mobile.post-something","description":"Só leitura apesar do nome","inputSchema":{"type":"object"},"annotations":{"readOnlyHint":true}}]}}`, *req.ID)
		case "tools/call":
			chamadas.Add(1)
			w.Header().Set("Content-Type", "text/event-stream")
			args, _ := json.Marshal(req.Params["arguments"])
			fmt.Fprintf(w, "event: message\ndata: {\"jsonrpc\":\"2.0\",\"method\":\"notifications/message\",\"params\":{\"level\":\"info\"}}\n\n")
			fmt.Fprintf(w, "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":%d,\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"{\\\"ok\\\":true,\\\"args\\\":%s}\"}],\"isError\":false}}\n\n", *req.ID, strings.ReplaceAll(string(args), `"`, `\"`))
		default:
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"error":{"code":-32601,"message":"método desconhecido"}}`, *req.ID)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &chamadas
}

func TestClienteListaEChamaComSessaoBearerEHostDeLoopback(t *testing.T) {
	srv, chamadas := servidorFalso(t, "b2c-token")
	// URL com o nome "mcp" não existe: o teste usa a URL do servidor, mas força o Host
	// como o Docker faria (nome do serviço → Host 127.0.0.1:porta).
	c := New(srv.URL+"/mcp/gruvi/bff", Static("b2c-token"))
	c.HostHeader = "127.0.0.1:" + srv.URL[strings.LastIndex(srv.URL, ":")+1:]
	tools, err := c.ListTools(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 3 || tools[0].Name != "gruvi.bff.building-operation.get-units" {
		t.Fatalf("tools: %+v", tools)
	}
	res, err := c.CallTool(context.Background(), "gruvi.bff.building-operation.get-units", map[string]any{"companyId": 5982})
	if err != nil {
		t.Fatal(err)
	}
	v, ok := res.Valor().(map[string]any)
	if !ok || v["ok"] != true || chamadas.Load() != 1 {
		t.Fatalf("resultado SSE: %#v (chamadas %d)", res.Valor(), chamadas.Load())
	}
}

func TestClienteRepeteLeituraAposSessaoExpirada(t *testing.T) {
	srv, inicializacoes, chamadas := servidorDeRetry(t, "gruvi.bff.mobile.get-profile", true, func(w http.ResponseWriter, req retryRequest, chamada int32) {
		if chamada == 1 {
			http.Error(w, "sessão expirada", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":{"content":[],"isError":false}}`, *req.ID)
	})
	defer srv.Close()
	c := New(srv.URL, nil)
	c.HostHeader = "127.0.0.1:" + srv.URL[strings.LastIndex(srv.URL, ":")+1:]
	if _, err := c.ListTools(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := c.CallTool(context.Background(), "gruvi.bff.mobile.get-profile", nil); err != nil {
		t.Fatal(err)
	}
	if chamadas.Load() != 2 || inicializacoes.Load() != 2 {
		t.Fatalf("leitura devia ser repetida com nova sessão: chamadas=%d inicializações=%d", chamadas.Load(), inicializacoes.Load())
	}
}

func TestClienteNaoRepeteEscritaAposSessaoExpirada(t *testing.T) {
	srv, inicializacoes, chamadas := servidorDeRetry(t, "gruvi.bff.mobile.create-visit", false, func(w http.ResponseWriter, _ retryRequest, _ int32) {
		http.Error(w, "sessão expirada", http.StatusNotFound)
	})
	defer srv.Close()
	c := New(srv.URL, nil)
	c.HostHeader = "127.0.0.1:" + srv.URL[strings.LastIndex(srv.URL, ":")+1:]
	if _, err := c.ListTools(context.Background()); err != nil {
		t.Fatal(err)
	}
	_, err := c.CallTool(context.Background(), "gruvi.bff.mobile.create-visit", nil)
	var he *httpError
	if !errors.As(err, &he) || he.status != http.StatusNotFound {
		t.Fatalf("esperava HTTP 404 sem replay, veio %v", err)
	}
	if chamadas.Load() != 1 || inicializacoes.Load() != 1 {
		t.Fatalf("escrita incerta não devia ser repetida: chamadas=%d inicializações=%d", chamadas.Load(), inicializacoes.Load())
	}
}

func TestClienteNaoRepeteEscritaComErroFuncionalMCP(t *testing.T) {
	srv, inicializacoes, chamadas := servidorDeRetry(t, "gruvi.bff.mobile.create-visit", false, func(w http.ResponseWriter, req retryRequest, _ int32) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":{"content":[{"type":"text","text":"falha funcional"}],"isError":true}}`, *req.ID)
	})
	defer srv.Close()
	c := New(srv.URL, nil)
	c.HostHeader = "127.0.0.1:" + srv.URL[strings.LastIndex(srv.URL, ":")+1:]
	if _, err := c.ListTools(context.Background()); err != nil {
		t.Fatal(err)
	}
	res, err := c.CallTool(context.Background(), "gruvi.bff.mobile.create-visit", nil)
	if err != nil || res == nil || !res.IsError {
		t.Fatalf("erro funcional MCP devia voltar sem replay: resultado=%+v erro=%v", res, err)
	}
	if chamadas.Load() != 1 || inicializacoes.Load() != 1 {
		t.Fatalf("erro funcional após POST não devia disparar retry: chamadas=%d inicializações=%d", chamadas.Load(), inicializacoes.Load())
	}
}

type retryRequest struct {
	ID     *int64         `json:"id"`
	Method string         `json:"method"`
	Params map[string]any `json:"params"`
}

func servidorDeRetry(t *testing.T, nome string, readOnly bool, responder func(http.ResponseWriter, retryRequest, int32)) (*httptest.Server, *atomic.Int32, *atomic.Int32) {
	t.Helper()
	var inicializacoes, chamadas atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req retryRequest
		_ = json.Unmarshal(body, &req)
		switch req.Method {
		case "initialize":
			n := inicializacoes.Add(1)
			w.Header().Set("Mcp-Session-Id", fmt.Sprintf("sess-%d", n))
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":{"protocolVersion":"2025-06-18","serverInfo":{"name":"falso","version":"1"}}}`, *req.ID)
		case "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)
		case "tools/list":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":{"tools":[{"name":%q,"annotations":{"readOnlyHint":%t}}]}}`, *req.ID, nome, readOnly)
		case "tools/call":
			responder(w, req, chamadas.Add(1))
		default:
			http.Error(w, "método desconhecido", http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &inicializacoes, &chamadas
}

func TestClienteSemTokenRecebeNaoAutorizado(t *testing.T) {
	srv, _ := servidorFalso(t, "b2c-token")
	c := New(srv.URL, nil)
	if _, err := c.ListTools(context.Background()); err != ErrNaoAutorizado {
		t.Fatalf("esperava ErrNaoAutorizado, veio %v", err)
	}
}

func TestHostHeaderPadrao(t *testing.T) {
	if h := (&Client{URL: "http://mcp:3337/mcp/gruvi/bff"}).hostHeader(); h != "127.0.0.1:3337" {
		t.Fatalf("fora de loopback devia virar 127.0.0.1:3337, veio %q", h)
	}
	if h := (&Client{URL: "http://127.0.0.1:3337/mcp"}).hostHeader(); h != "" {
		t.Fatalf("loopback não mexe no Host, veio %q", h)
	}
	if h := (&Client{URL: "http://mcp:3337/mcp", HostHeader: "x:1"}).hostHeader(); h != "x:1" {
		t.Fatal("HostHeader configurado vence")
	}
}

func TestRecursosConvertemToolsEmRecursosDoNexus(t *testing.T) {
	srv, _ := servidorFalso(t, "")
	c := New(srv.URL, nil)
	c.HostHeader = "127.0.0.1:1"
	tools, err := c.ListTools(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	rs := Recursos(tools)
	por := map[string]okf.Recurso{}
	for _, r := range rs {
		por[r.Name] = r
	}
	u := por["gruvi.bff.building-operation.get-units"]
	if u.Kind != okf.KindAPI || u.Visibility != "private" || u.Write || u.Origem != okf.OrigemMCP {
		t.Fatalf("get-units: %+v", u)
	}
	if len(u.Params) != 2 || u.Params[0].Name != "companyId" || !u.Params[0].Required || u.Params[0].Type != "number" || u.Params[1].Name != "filter" || !strings.Contains(u.Params[1].Description, "mobile-number*: string") {
		t.Fatalf("params: %+v", u.Params)
	}
	if !por["gruvi.bff.building-operation.create-visit"].Write {
		t.Fatal("create-visit com readOnlyHint=false é escrita")
	}
	if !strings.Contains(por["gruvi.bff.building-operation.create-visit"].Params[1].Description, "today | tomorrow") {
		t.Fatalf("enum devia aparecer: %+v", por["gruvi.bff.building-operation.create-visit"].Params)
	}
	if por["gruvi.bff.mobile.post-something"].Write {
		t.Fatal("readOnlyHint=true vence o verbo do nome")
	}
	if !escrita(Tool{Name: "gruvi.bff.mobile.update-vehicle"}) || escrita(Tool{Name: "gruvi.bff.mobile.list-vehicles"}) {
		t.Fatal("heurística pelo verbo")
	}
}

func TestFonteCacheECombinar(t *testing.T) {
	srv, _ := servidorFalso(t, "")
	c := New(srv.URL, nil)
	c.HostHeader = "127.0.0.1:1"
	f := Fonte(c, time.Minute, nil)
	a := f()
	b := f()
	if len(a) != 3 || len(b) != 3 {
		t.Fatalf("fonte: %d %d", len(a), len(b))
	}
	local := func() []okf.Recurso {
		return []okf.Recurso{{Name: "gruvi.bff.mobile.post-something", Kind: okf.KindAPI, Description: "do instantâneo"}, {Name: "faq.buscar", Kind: okf.KindKnowledge}}
	}
	todos := Combinar(f, local)()
	if len(todos) != 4 {
		t.Fatalf("combinado: %d", len(todos))
	}
	for _, r := range todos {
		if r.Name == "gruvi.bff.mobile.post-something" && r.Origem != okf.OrigemMCP {
			t.Fatal("pelo nome, a primeira fonte (MCP) vence")
		}
	}
	if Combinar(nil, nil) != nil {
		t.Fatal("sem fontes devolve nil")
	}
	// servidor fora do ar: lista vazia, sem derrubar
	srv.Close()
	f2 := Fonte(New("http://127.0.0.1:1/mcp", nil), time.Minute, nil)
	if got := f2(); got == nil || len(got) != 0 {
		t.Fatalf("fora do ar devia dar lista vazia, veio %v", got)
	}
}

func TestKeycloakClientCredentialsComCache(t *testing.T) {
	var pedidos atomic.Int32
	kc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pedidos.Add(1)
		_ = r.ParseForm()
		if r.Form.Get("grant_type") != "client_credentials" || r.Form.Get("client_id") != "concierge" || r.Form.Get("client_secret") != "s3gredo" {
			http.Error(w, "bad", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"tok-1","expires_in":300}`)
	}))
	defer kc.Close()
	k := &Keycloak{TokenURL: kc.URL, ClientID: "concierge", ClientSecret: "s3gredo"}
	for i := 0; i < 3; i++ {
		tok, err := k.Token(context.Background())
		if err != nil || tok != "tok-1" {
			t.Fatalf("token: %q %v", tok, err)
		}
	}
	if pedidos.Load() != 1 {
		t.Fatalf("o token devia ficar em cache: %d pedidos", pedidos.Load())
	}
}

func TestFromEnv(t *testing.T) {
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	if c, _, err := FromEnv(env(map[string]string{})); c != nil || err != nil {
		t.Fatal("sem MCP_URL: nil")
	}
	c, modo, err := FromEnv(env(map[string]string{"MCP_URL": "http://mcp:3337/mcp", "MCP_BEARER_TOKEN": "abc"}))
	if err != nil || modo != "bearer" || c.Token.(Static) != "abc" {
		t.Fatalf("bearer: %v %s", err, modo)
	}
	c, modo, err = FromEnv(env(map[string]string{"MCP_URL": "http://mcp:3337/mcp", "MCP_KEYCLOAK_URL": "http://kc/token", "MCP_KEYCLOAK_CLIENT_ID": "x"}))
	if err != nil || modo != "keycloak" {
		t.Fatalf("keycloak: %v %s", err, modo)
	}
	if _, _, err := FromEnv(env(map[string]string{"MCP_URL": "http://mcp:3337/mcp", "MCP_AUTH": "bearer"})); err == nil {
		t.Fatal("bearer sem token devia falhar")
	}
	if _, _, err := FromEnv(env(map[string]string{"MCP_URL": "http://mcp:3337/mcp", "MCP_AUTH": "xyz"})); err == nil {
		t.Fatal("modo desconhecido devia falhar")
	}
	if c, modo, _ := FromEnv(env(map[string]string{"MCP_URL": "http://mcp:3337/mcp"})); modo != "none" || c.Token != nil {
		t.Fatal("sem credencial: none")
	}
}
