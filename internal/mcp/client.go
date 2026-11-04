// Package mcp é o cliente do catálogo MCP oficial (Model Context Protocol, transporte
// Streamable HTTP): inicializa a sessão, lista as tools e as chama. Cada tool do
// servidor vira um recurso do Nexus (fonte.go), e o harness a executa com o token
// certo (token.go): um Bearer fixo (o token B2C do Keycloak, como na entrega) ou um
// token obtido no Keycloak por client_credentials.
//
// O servidor da entrega (mcp/, Capability Views) só aceita requisições cujo header
// Host seja 127.0.0.1/localhost/[::1] com a porta dele; por isso o cliente manda esse
// Host mesmo quando chega pelo nome do serviço no Docker (HostHeader).
package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go-concierge/internal/falha"
)

const (
	ProtocolVersion = "2025-06-18"
	maxBody         = 8 << 20
	// prazoChamada limita cada chamada ao MCP (initialize, tools/list, tools/call), mesmo com
	// um ctx sem prazo; o Timeout do http.Client é a segunda trava.
	prazoChamada = 60 * time.Second
)

// Client fala JSON-RPC 2.0 com um endpoint MCP por HTTP.
type Client struct {
	URL   string
	Token TokenProvider // nil = sem Authorization
	// HostHeader é o Host enviado; vazio = o da URL (ou 127.0.0.1:porta fora de loopback).
	HostHeader string
	HTTP       *http.Client

	mu          sync.Mutex
	session     string
	initialized bool
	seq         atomic.Int64
	toolsMu     sync.RWMutex
	toolWrites  map[string]bool
}

// New cria um cliente para a URL (ex.: http://127.0.0.1:3337/mcp/gruvi/bff).
func New(endpoint string, token TokenProvider) *Client {
	return &Client{URL: endpoint, Token: token, HTTP: &http.Client{Timeout: prazoChamada}}
}

func (c *Client) ValidarToken(ctx context.Context) error {
	if c == nil || c.Token == nil {
		return errors.New("mcp: token B2C não configurado")
	}
	token, err := c.Token.Token(ctx)
	if err != nil {
		return fmt.Errorf("mcp: token B2C indisponível: %w", err)
	}
	if strings.TrimSpace(token) == "" {
		return errors.New("mcp: token B2C vazio")
	}
	return nil
}

// Tool é uma tool publicada pelo servidor (tools/list).
type Tool struct {
	Name        string           `json:"name"`
	Title       string           `json:"title,omitempty"`
	Description string           `json:"description,omitempty"`
	InputSchema json.RawMessage  `json:"inputSchema,omitempty"`
	Annotations *ToolAnnotations `json:"annotations,omitempty"`
}

// ToolAnnotations são as dicas do servidor sobre a tool (MCP spec).
type ToolAnnotations struct {
	Title           string `json:"title,omitempty"`
	ReadOnlyHint    *bool  `json:"readOnlyHint,omitempty"`
	DestructiveHint *bool  `json:"destructiveHint,omitempty"`
	IdempotentHint  *bool  `json:"idempotentHint,omitempty"`
	OpenWorldHint   *bool  `json:"openWorldHint,omitempty"`
}

// Content é um bloco do resultado de tools/call.
type Content struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// CallResult é o resultado de tools/call.
type CallResult struct {
	Content           []Content       `json:"content"`
	StructuredContent json.RawMessage `json:"structuredContent,omitempty"`
	IsError           bool            `json:"isError,omitempty"`
}

// Texto junta os blocos de texto.
func (r *CallResult) Texto() string {
	var parts []string
	for _, c := range r.Content {
		if c.Type == "text" && strings.TrimSpace(c.Text) != "" {
			parts = append(parts, c.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// Valor devolve o resultado como dado: structuredContent, senão o texto (como JSON
// quando for JSON válido, senão a string).
func (r *CallResult) Valor() any {
	if len(r.StructuredContent) > 0 {
		var v any
		if json.Unmarshal(r.StructuredContent, &v) == nil {
			return v
		}
	}
	t := r.Texto()
	var v any
	if json.Unmarshal([]byte(t), &v) == nil && (strings.HasPrefix(strings.TrimSpace(t), "{") || strings.HasPrefix(strings.TrimSpace(t), "[")) {
		return v
	}
	return t
}

// Caller é o que o harness precisa do cliente (facilita testes).
type Caller interface {
	CallTool(ctx context.Context, name string, args map[string]any) (*CallResult, error)
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      *int64 `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError é o erro JSON-RPC devolvido pelo servidor.
type RPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *RPCError) Error() string { return fmt.Sprintf("mcp: %s (código %d)", e.Message, e.Code) }

// ErrNaoAutorizado: o servidor recusou o token (401).
var ErrNaoAutorizado = errors.New("mcp: não autorizado (token ausente, inválido ou expirado)")

// Initialize abre a sessão (initialize + notifications/initialized). Idempotente.
func (c *Client) Initialize(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.initLocked(ctx)
}

func (c *Client) initLocked(ctx context.Context) error {
	if c.initialized {
		return nil
	}
	c.session = ""
	var res struct {
		ProtocolVersion string `json:"protocolVersion"`
		ServerInfo      struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"serverInfo"`
	}
	if err := c.call(ctx, "initialize", map[string]any{
		"protocolVersion": ProtocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "go-concierge", "version": "1"},
	}, &res); err != nil {
		return err
	}
	if err := c.notify(ctx, "notifications/initialized"); err != nil {
		return err
	}
	c.initialized = true
	return nil
}

// ListTools lista todas as tools (seguindo a paginação por cursor).
func (c *Client) ListTools(ctx context.Context) ([]Tool, error) {
	if err := c.Initialize(ctx); err != nil {
		return nil, err
	}
	var out []Tool
	cursor := ""
	for {
		params := map[string]any{}
		if cursor != "" {
			params["cursor"] = cursor
		}
		var res struct {
			Tools      []Tool `json:"tools"`
			NextCursor string `json:"nextCursor"`
		}
		if err := c.callRetry(ctx, "tools/list", params, &res, true); err != nil {
			return nil, err
		}
		out = append(out, res.Tools...)
		if res.NextCursor == "" || len(res.Tools) == 0 {
			break
		}
		cursor = res.NextCursor
	}
	c.cacheToolWrites(out)
	return out, nil
}

// CallTool chama uma tool com os argumentos.
func (c *Client) CallTool(ctx context.Context, name string, args map[string]any) (*CallResult, error) {
	if err := c.Initialize(ctx); err != nil {
		return nil, err
	}
	if args == nil {
		args = map[string]any{}
	}
	var res CallResult
	if err := c.callRetry(ctx, "tools/call", map[string]any{"name": name, "arguments": args}, &res, c.toolReadOnly(name)); err != nil {
		return nil, err
	}
	return &res, nil
}

// callRetry repete uma vez quando a sessão expirou (404), se a chamada puder ser
// repetida com segurança. Escritas nunca são repetidas após resultado incerto.
func (c *Client) callRetry(ctx context.Context, method string, params, out any, retryOnNotFound bool) error {
	err := c.call(ctx, method, params, out)
	var he *httpError
	if retryOnNotFound && errors.As(err, &he) && he.status == http.StatusNotFound {
		c.mu.Lock()
		c.initialized = false
		ierr := c.initLocked(ctx)
		c.mu.Unlock()
		if ierr != nil {
			return ierr
		}
		return c.call(ctx, method, params, out)
	}
	return err
}

func (c *Client) cacheToolWrites(tools []Tool) {
	writes := make(map[string]bool, len(tools))
	for _, tool := range tools {
		writes[tool.Name] = escrita(tool)
	}
	c.toolsMu.Lock()
	c.toolWrites = writes
	c.toolsMu.Unlock()
}

func (c *Client) toolReadOnly(name string) bool {
	c.toolsMu.RLock()
	writes, found := c.toolWrites[name]
	c.toolsMu.RUnlock()
	if found {
		return !writes
	}
	return false
}

type httpError struct {
	status int
	corpo  string
}

func (e *httpError) Error() string {
	return fmt.Sprintf("mcp: HTTP %d: %s", e.status, strings.TrimSpace(e.corpo))
}

func (c *Client) notify(ctx context.Context, method string) error {
	_, err := c.post(ctx, rpcRequest{JSONRPC: "2.0", Method: method}, nil)
	return err
}

// call manda uma requisição com id e espera a resposta correspondente (JSON ou SSE).
func (c *Client) call(ctx context.Context, method string, params, out any) error {
	id := c.seq.Add(1)
	raw, err := c.post(ctx, rpcRequest{JSONRPC: "2.0", ID: &id, Method: method, Params: params}, &id)
	if err != nil {
		return err
	}
	if raw == nil {
		return fmt.Errorf("mcp: %s sem resposta", method)
	}
	if raw.Error != nil {
		return raw.Error
	}
	if out != nil && len(raw.Result) > 0 {
		if err := json.Unmarshal(raw.Result, out); err != nil {
			return fmt.Errorf("mcp: resultado de %s inválido: %w", method, err)
		}
	}
	return nil
}

// post envia o JSON-RPC e lê a resposta. wantID != nil: procura a resposta com esse
// id (num corpo JSON ou num stream SSE). Devolve nil para notificações (202).
func (c *Client) post(ctx context.Context, body rpcRequest, wantID *int64) (*rpcResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, prazoChamada)
	defer cancel() // a resposta (JSON ou SSE) é lida inteira antes de voltar
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", ProtocolVersion)
	if c.session != "" {
		req.Header.Set("Mcp-Session-Id", c.session)
	}
	if c.Token != nil {
		tok, err := c.Token.Token(ctx)
		if err != nil {
			return nil, err
		}
		if tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}
	}
	if h := c.hostHeader(); h != "" {
		req.Host = h
	}
	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: prazoChamada} // Client montado à mão, sem New
	}
	res, err := hc.Do(req)
	if err != nil {
		return nil, falha.ComPilha(fmt.Errorf("mcp: %w", err))
	}
	defer res.Body.Close()
	if sid := res.Header.Get("Mcp-Session-Id"); sid != "" {
		c.session = sid
	}
	switch {
	case res.StatusCode == http.StatusUnauthorized:
		return nil, ErrNaoAutorizado
	case res.StatusCode == http.StatusAccepted || res.StatusCode == http.StatusNoContent:
		return nil, nil
	case res.StatusCode < 200 || res.StatusCode >= 300:
		corpo, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
		return nil, falha.ComPilha(&httpError{status: res.StatusCode, corpo: string(corpo)})
	}
	if wantID == nil {
		return nil, nil
	}
	ct := strings.ToLower(res.Header.Get("Content-Type"))
	if strings.HasPrefix(ct, "text/event-stream") {
		return readSSE(res.Body, *wantID)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, maxBody))
	if err != nil {
		return nil, err
	}
	return pickResponse(data, *wantID)
}

// pickResponse aceita um objeto ou um lote JSON-RPC e devolve a resposta do id.
func pickResponse(data []byte, wantID int64) (*rpcResponse, error) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil, errors.New("mcp: resposta vazia")
	}
	if data[0] == '[' {
		var lote []rpcResponse
		if err := json.Unmarshal(data, &lote); err != nil {
			return nil, fmt.Errorf("mcp: lote inválido: %w", err)
		}
		for i := range lote {
			if idOf(lote[i].ID) == wantID {
				return &lote[i], nil
			}
		}
		return nil, errors.New("mcp: resposta sem o id esperado")
	}
	var r rpcResponse
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("mcp: resposta inválida: %w", err)
	}
	if idOf(r.ID) != wantID {
		return nil, errors.New("mcp: resposta com outro id")
	}
	return &r, nil
}

// readSSE lê o stream de eventos até achar a resposta do id (ignora notificações).
func readSSE(body io.Reader, wantID int64) (*rpcResponse, error) {
	sc := bufio.NewScanner(body)
	sc.Buffer(make([]byte, 64<<10), maxBody)
	var data []string
	flush := func() (*rpcResponse, bool) {
		if len(data) == 0 {
			return nil, false
		}
		payload := strings.Join(data, "\n")
		data = nil
		var r rpcResponse
		if json.Unmarshal([]byte(payload), &r) != nil || len(r.ID) == 0 {
			return nil, false // notificação ou evento que não é resposta
		}
		if idOf(r.ID) != wantID {
			return nil, false
		}
		return &r, true
	}
	for sc.Scan() {
		line := sc.Text()
		switch {
		case line == "":
			if r, ok := flush(); ok {
				return r, nil
			}
		case strings.HasPrefix(line, "data:"):
			data = append(data, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		default: // event:, id:, retry:, comentários
		}
	}
	if r, ok := flush(); ok {
		return r, nil
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("mcp: stream: %w", err)
	}
	return nil, errors.New("mcp: stream terminou sem a resposta")
}

func idOf(raw json.RawMessage) int64 {
	var n int64
	if json.Unmarshal(raw, &n) == nil {
		return n
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		var m int64
		_, _ = fmt.Sscan(s, &m)
		return m
	}
	return -1
}

// hostHeader: o configurado; fora de loopback, 127.0.0.1:porta (o servidor da entrega
// valida o Host contra loopback e a porta em que escuta).
func (c *Client) hostHeader() string {
	if c.HostHeader != "" {
		return c.HostHeader
	}
	u, err := url.Parse(c.URL)
	if err != nil {
		return ""
	}
	host := u.Hostname()
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return ""
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return ""
	}
	port := u.Port()
	if port == "" {
		if u.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	return "127.0.0.1:" + port
}
