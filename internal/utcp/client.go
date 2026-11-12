package utcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	sdk "github.com/universal-tool-calling-protocol/go-utcp"
	"github.com/universal-tool-calling-protocol/go-utcp/src/providers/base"
	httpprovider "github.com/universal-tool-calling-protocol/go-utcp/src/providers/http"
	"github.com/universal-tool-calling-protocol/go-utcp/src/repository"
	sdktools "github.com/universal-tool-calling-protocol/go-utcp/src/tools"
	"github.com/universal-tool-calling-protocol/go-utcp/src/transports"
	sdkhttp "github.com/universal-tool-calling-protocol/go-utcp/src/transports/http"

	"go-concierge/internal/okf"
	"go-concierge/internal/recursos"
)

const (
	ProviderName = "agent_utcp"
	CompanyTool  = "agent_utcp.building-operation.GetCompanyByDocumentController_handle"
	PackagesTool = "agent_utcp.building-operation.FetchUnitPackagesController_handle"
)

type TokenProvider interface {
	Token(context.Context) (string, error)
}

type Client struct {
	manualURL string
	token     TokenProvider
	http      *http.Client
	mu        sync.Mutex
	snapshot  *snapshot
}

type snapshot struct {
	client      sdk.UtcpClientInterface
	bridge      *bridge
	definitions map[string]recursos.Definicao
}

type bridge struct {
	manualURL  string
	manualHTTP *http.Client
	transport  *sdkhttp.HttpClientTransport
	token      TokenProvider
	operations map[string]operation
}

type operation struct {
	definition recursos.Definicao
	provider   *httpprovider.HttpProvider
	timeout    time.Duration
}

type manualTool struct {
	Name             string          `json:"name"`
	Description      string          `json:"description"`
	Inputs           json.RawMessage `json:"inputs"`
	Outputs          json.RawMessage `json:"outputs"`
	ToolCallTemplate struct {
		Type    string            `json:"call_template_type"`
		Method  string            `json:"http_method"`
		URL     string            `json:"url"`
		Headers map[string]string `json:"headers"`
		Timeout int               `json:"timeout"`
	} `json:"tool_call_template"`
}

type manualDocument struct {
	Version       string       `json:"version"`
	UTCPVersion   string       `json:"utcp_version"`
	ManualVersion string       `json:"manual_version"`
	Tools         []manualTool `json:"tools"`
}

func New(manualURL string, token TokenProvider) (*Client, error) {
	parsed, err := url.Parse(strings.TrimSpace(manualURL))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || strings.TrimRight(parsed.Path, "/") != "/utcp" {
		return nil, errors.New("UTCP manual URL inválida")
	}
	return &Client{manualURL: parsed.String(), token: token, http: &http.Client{Timeout: 15 * time.Second}}, nil
}

func (c *Client) Descrever(ctx context.Context, name string) (recursos.Definicao, error) {
	s, err := c.ensure(ctx)
	if err != nil {
		return recursos.Definicao{}, err
	}
	definition, ok := s.definitions[name]
	if !ok {
		return recursos.Definicao{}, fmt.Errorf("recurso UTCP ausente: %s", name)
	}
	definition.Schema = append(json.RawMessage(nil), definition.Schema...)
	return definition, nil
}

func (c *Client) Executar(ctx context.Context, name string, args map[string]any) (any, error) {
	s, err := c.ensure(ctx)
	if err != nil {
		return nil, err
	}
	op, ok := s.bridge.operations[name]
	if !ok {
		return nil, fmt.Errorf("recurso UTCP ausente: %s", name)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	callCtx, cancel := context.WithTimeout(ctx, op.timeout)
	defer cancel()
	return s.client.CallTool(callCtx, name, args)
}

func (c *Client) ensure(ctx context.Context) (*snapshot, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.snapshot != nil {
		return c.snapshot, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	repo := repository.NewInMemoryToolRepository()
	candidateBridge := &bridge{manualURL: c.manualURL, manualHTTP: c.http, transport: sdkhttp.NewHttpClientTransport(nil), token: c.token, operations: map[string]operation{}}
	candidateClient, err := sdk.NewUTCPClient(ctx, nil, repo, nil)
	if err != nil {
		return nil, errors.New("inicialização do cliente UTCP falhou")
	}
	transportsMap := candidateClient.GetTransports()
	if transportsMap == nil || transportsMap["http"] == nil {
		return nil, errors.New("transporte HTTP UTCP indisponível")
	}
	transportsMap["http"] = candidateBridge
	provider := &httpprovider.HttpProvider{
		BaseProvider: base.BaseProvider{Name: ProviderName, ProviderType: base.ProviderHTTP},
		HTTPMethod:   http.MethodGet,
		URL:          c.manualURL,
	}
	registered, err := candidateClient.RegisterToolProvider(ctx, provider)
	if err != nil {
		return nil, errors.New("descoberta do manual UTCP falhou")
	}
	found := map[string]bool{}
	for _, tool := range registered {
		found[tool.Name] = true
	}
	definitions := candidateBridge.definitions()
	for _, name := range []string{CompanyTool, PackagesTool} {
		if !found[name] {
			return nil, fmt.Errorf("manual UTCP sem operação necessária: %s", name)
		}
		if _, ok := definitions[name]; !ok {
			return nil, fmt.Errorf("manual UTCP sem contrato de entrada: %s", name)
		}
	}
	result := &snapshot{client: candidateClient, bridge: candidateBridge, definitions: definitions}
	c.snapshot = result
	return result, nil
}

func (b *bridge) RegisterToolProvider(ctx context.Context, provider base.Provider) ([]sdktools.Tool, error) {
	manualProvider, ok := provider.(*httpprovider.HttpProvider)
	if !ok || manualProvider.URL != b.manualURL {
		return nil, errors.New("provider manual UTCP inválido")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.manualURL, nil)
	if err != nil {
		return nil, errors.New("requisição de descoberta UTCP inválida")
	}
	req.Header.Set("Accept", "application/json")
	response, err := b.manualHTTP.Do(req)
	if err != nil {
		return nil, errors.New("manual UTCP indisponível")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("manual UTCP respondeu HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return nil, errors.New("leitura do manual UTCP falhou")
	}
	var manual manualDocument
	if err := json.Unmarshal(body, &manual); err != nil {
		return nil, errors.New("manual UTCP inválido")
	}
	if manual.Version == "" && manual.UTCPVersion == "" && manual.ManualVersion == "" || len(manual.Tools) == 0 {
		return nil, errors.New("manual UTCP vazio")
	}
	for _, t := range manual.Tools {
		if t.Name != CompanyTool && t.Name != PackagesTool {
			continue
		}
		if _, duplicate := b.operations[t.Name]; duplicate {
			return nil, fmt.Errorf("operação UTCP duplicada: %s", t.Name)
		}
		template := t.ToolCallTemplate
		if template.Type != "http" || strings.ToUpper(template.Method) != http.MethodGet || template.URL == "" || template.Timeout <= 0 {
			return nil, fmt.Errorf("template HTTP inválido para %s", t.Name)
		}
		parsed, err := url.Parse(template.URL)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil {
			return nil, fmt.Errorf("URL HTTP inválida para %s", t.Name)
		}
		schema, err := rawInputSchema(t.Inputs)
		if err != nil {
			return nil, fmt.Errorf("schema inválido para %s", t.Name)
		}
		provider := &httpprovider.HttpProvider{
			BaseProvider: base.BaseProvider{Name: ProviderName + "_" + strings.ReplaceAll(strings.TrimPrefix(t.Name, ProviderName+"."), ".", "_"), ProviderType: base.ProviderHTTP},
			HTTPMethod:   http.MethodGet,
			URL:          template.URL,
			Headers:      cloneHeaders(template.Headers),
		}
		b.operations[t.Name] = operation{definition: recursos.Definicao{Nome: t.Name, Descricao: t.Description, Schema: schema}, provider: provider, timeout: time.Duration(template.Timeout) * time.Millisecond}
	}
	tools := make([]sdktools.Tool, 0, len(b.operations))
	for _, t := range manual.Tools {
		op, ok := b.operations[t.Name]
		if !ok {
			continue
		}
		inputs := decodeToolSchema(op.definition.Schema)
		tools = append(tools, sdktools.Tool{Name: t.Name, Description: t.Description, Inputs: inputs, Outputs: decodeToolSchema(t.Outputs), Provider: provider})
	}
	return tools, nil
}

func rawInputSchema(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 {
		return nil, errors.New("inputs ausente")
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		return nil, err
	}
	if schema["type"] != "object" {
		return nil, errors.New("inputs precisa ser objeto")
	}
	return append(json.RawMessage(nil), raw...), nil
}

func decodeToolSchema(raw json.RawMessage) sdktools.ToolInputOutputSchema {
	var s struct {
		Type        string                 `json:"type"`
		Properties  map[string]interface{} `json:"properties"`
		Required    []string               `json:"required"`
		Description string                 `json:"description"`
		Title       string                 `json:"title"`
		Items       map[string]interface{} `json:"items"`
		Enum        []interface{}          `json:"enum"`
	}
	_ = json.Unmarshal(raw, &s)
	return sdktools.ToolInputOutputSchema{Type: s.Type, Properties: s.Properties, Required: s.Required, Description: s.Description, Title: s.Title, Items: s.Items, Enum: s.Enum}
}

func cloneHeaders(headers map[string]string) map[string]string {
	copy := make(map[string]string, len(headers))
	for key, value := range headers {
		copy[key] = value
	}
	return copy
}

func (b *bridge) DeregisterToolProvider(context.Context, base.Provider) error { return nil }

func (b *bridge) CallTool(ctx context.Context, name string, args map[string]any, _ base.Provider, _ *string) (any, error) {
	op, ok := b.operations[name]
	if !ok {
		return nil, fmt.Errorf("operação UTCP desconhecida: %s", name)
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal(op.definition.Schema, &schema); err != nil {
		return nil, errors.New("schema de entrada UTCP inválido")
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		return nil, errors.New("schema de entrada UTCP inválido")
	}
	if err := resolved.Validate(args); err != nil {
		return nil, fmt.Errorf("argumentos UTCP inválidos: %w", err)
	}
	provider := *op.provider
	provider.Headers = cloneHeaders(op.provider.Headers)
	for key, value := range provider.Headers {
		if strings.Contains(value, "${B2C_TOKEN}") {
			if b.token == nil {
				return nil, errors.New("token B2C não configurado")
			}
			token, err := b.token.Token(ctx)
			if err != nil {
				return nil, errors.New("token B2C indisponível")
			}
			if strings.TrimSpace(token) == "" {
				return nil, errors.New("token B2C vazio")
			}
			provider.Headers[key] = strings.ReplaceAll(value, "${B2C_TOKEN}", token)
		}
	}
	return b.transport.CallTool(ctx, name, args, &provider, nil)
}

func (b *bridge) CallToolStream(context.Context, string, map[string]any, base.Provider) (transports.StreamResult, error) {
	return nil, errors.New("streaming UTCP indisponível")
}

func (b *bridge) definitions() map[string]recursos.Definicao {
	definitions := make(map[string]recursos.Definicao, len(b.operations))
	for name, op := range b.operations {
		definition := op.definition
		definition.Schema = append(json.RawMessage(nil), definition.Schema...)
		definitions[name] = definition
	}
	return definitions
}

func Fonte(client *Client) func() []okf.Recurso {
	if client == nil {
		return nil
	}
	return func() []okf.Recurso {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, discoveryErr := client.ensure(ctx)
		out := []okf.Recurso{
			{Name: CompanyTool, Kind: okf.KindAPI, Visibility: "private", Description: "Consulta os dados e unidades vinculados ao telefone do morador.", Output: "dados do condomínio e unidades vinculadas", Origem: okf.OrigemUTCP},
			{Name: PackagesTool, Kind: okf.KindAPI, Visibility: "private", Description: "Consulta encomendas da unidade autorizada pelo canal.", Output: "lista de encomendas", Origem: okf.OrigemUTCP},
		}
		for i := range out {
			if discoveryErr == nil {
				definition, err := client.Descrever(ctx, out[i].Name)
				if err != nil {
					continue
				}
				out[i].Description = definition.Descricao
				var schema any
				if json.Unmarshal(definition.Schema, &schema) == nil {
					out[i].Schema = schema
				}
				out[i].Params = params(definition.Schema)
			} else {
				out[i].Description += " O manual UTCP está indisponível no momento."
			}
		}
		return out
	}
}

func params(raw json.RawMessage) []okf.ParamMeta {
	var schema struct {
		Properties map[string]struct {
			Type        string `json:"type"`
			Description string `json:"description"`
		} `json:"properties"`
		Required []string `json:"required"`
	}
	if json.Unmarshal(raw, &schema) != nil {
		return nil
	}
	required := map[string]bool{}
	for _, name := range schema.Required {
		required[name] = true
	}
	out := make([]okf.ParamMeta, 0, len(schema.Properties))
	for name, property := range schema.Properties {
		out = append(out, okf.ParamMeta{Name: name, Type: property.Type, Required: required[name], Description: property.Description})
	}
	return out
}
