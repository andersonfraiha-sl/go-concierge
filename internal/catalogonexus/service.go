package catalogonexus

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"strings"
	"sync"

	"go-concierge/internal/mcp"
	"go-concierge/internal/okf"
)

const (
	ToolSearch = "search_operations"
	ToolDetail = "get_operation"
	DefaultURL = "https://gateway-catalog.idp.nexus.superlogica.net/catalog-mcp/mcp"
)

type Caller interface {
	CallTool(context.Context, string, map[string]any) (*mcp.CallResult, error)
}

type Service struct {
	caller Caller
}

func New(caller Caller) *Service { return &Service{caller: caller} }

func NewMCPCaller(endpoint, token string) (Caller, error) {
	u, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return nil, errors.New("URL do catálogo Nexus inválida")
	}
	if strings.TrimSpace(token) == "" {
		return nil, errors.New("LLM_LUNA_GATEWAY_TOKEN não configurado")
	}
	client := mcp.New(u.String(), mcp.Static(token))
	client.HostHeader = u.Host
	return &serializedCaller{client: client}, nil
}

type serializedCaller struct {
	client *mcp.Client
	mu     sync.Mutex
}

func (c *serializedCaller) CallTool(ctx context.Context, name string, args map[string]any) (*mcp.CallResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.client.CallTool(ctx, name, args)
}

type Operation struct {
	Method      string `json:"method"`
	Path        string `json:"path"`
	OperationID string `json:"operationId"`
	Title       string `json:"title"`
	ServiceName string `json:"serviceName"`
	ServiceID   string `json:"serviceId"`
	Score       string `json:"score,omitempty"`
}

type Detail struct {
	Method      string          `json:"method"`
	Path        string          `json:"path"`
	URL         string          `json:"url"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Params      []okf.ParamMeta `json:"params"`
	Definition  string          `json:"definition"`
	Evidence    string          `json:"evidence"`
	ServiceID   string          `json:"serviceId"`
	OperationID string          `json:"operationId"`
}

func (s *Service) Search(ctx context.Context, query string) ([]Operation, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, errors.New("informe o que deseja buscar")
	}
	result, err := s.call(ctx, ToolSearch, map[string]any{"q": query})
	if err != nil {
		return nil, err
	}
	if result.IsError {
		return nil, errors.New("o catálogo Nexus recusou a pesquisa")
	}
	if len(result.StructuredContent) > 0 {
		if operations, ok := parseStructuredSearch(result.StructuredContent); ok {
			return operations, nil
		}
	}
	operations, err := parseSearchText(result.Texto())
	if err != nil {
		return nil, err
	}
	return operations, nil
}

func (s *Service) GetOperation(ctx context.Context, serviceID, operationID string) (Detail, error) {
	serviceID, operationID = strings.TrimSpace(serviceID), strings.TrimSpace(operationID)
	if serviceID == "" || operationID == "" {
		return Detail{}, errors.New("selecione um resultado válido da pesquisa")
	}
	result, err := s.call(ctx, ToolDetail, map[string]any{"serviceId": serviceID, "operationId": operationID})
	if err != nil {
		return Detail{}, err
	}
	if result.IsError {
		return Detail{}, errors.New("o catálogo Nexus não encontrou os detalhes da operação")
	}
	detail, ok := parseStructuredDetail(result.StructuredContent)
	if !ok {
		detail, ok = parseDetailText(result.Texto())
	}
	if !ok || detail.Method == "" || detail.Path == "" {
		return Detail{}, errors.New("o catálogo Nexus devolveu detalhes em formato não compreensível")
	}
	detail.ServiceID = serviceID
	detail.OperationID = operationID
	if detail.Params == nil {
		detail.Params = []okf.ParamMeta{}
	}
	return detail, nil
}

func (s *Service) call(ctx context.Context, name string, args map[string]any) (*mcp.CallResult, error) {
	if s == nil || s.caller == nil {
		return nil, errors.New("consulta do catálogo Nexus indisponível: token backend não configurado")
	}
	result, err := s.caller.CallTool(ctx, name, args)
	if err != nil {
		return nil, errors.New("não foi possível consultar o catálogo Nexus; confira a configuração e tente novamente")
	}
	if result == nil {
		return nil, errors.New("o catálogo Nexus não devolveu resposta")
	}
	return result, nil
}

var (
	reSearchRow = regexp.MustCompile(`^- ([A-Z]+) (.*?) · ([^ ·]+) · (.+)$`)
	reServiceID = regexp.MustCompile(`serviceId: ([^ ·]+)`)
	reScore     = regexp.MustCompile(`score ([^ ·]+)`)
	reHTTP      = regexp.MustCompile(`^(GET|POST|PUT|PATCH|DELETE) (\S+)`)
	reURL       = regexp.MustCompile(`- URL: (https?://\S+)`)
	reZero      = regexp.MustCompile(`^0 operação\(ões\)`)
)

func parseSearchText(text string) ([]Operation, error) {
	if strings.TrimSpace(text) == "" {
		return nil, errors.New("o catálogo Nexus devolveu uma pesquisa vazia ou sem formato reconhecido")
	}
	lines := strings.Split(text, "\n")
	out := make([]Operation, 0)
	for i, line := range lines {
		row := reSearchRow.FindStringSubmatch(strings.TrimSpace(line))
		if row == nil {
			continue
		}
		serviceID := ""
		serviceName := ""
		score := ""
		for j := i + 1; j < len(lines) && j <= i+2; j++ {
			serviceIDMatch := reServiceID.FindStringSubmatch(lines[j])
			if serviceIDMatch != nil {
				serviceID = serviceIDMatch[1]
				serviceName = strings.TrimSpace(strings.SplitN(strings.TrimSpace(lines[j]), " · serviceId:", 2)[0])
				serviceName = strings.TrimPrefix(serviceName, "serviço: ")
			}
			if scoreMatch := reScore.FindStringSubmatch(lines[j]); scoreMatch != nil {
				score = scoreMatch[1]
			}
			if strings.HasPrefix(strings.TrimSpace(lines[j]), "- ") {
				break
			}
		}
		if serviceID == "" {
			continue
		}
		out = append(out, Operation{Method: row[1], Path: row[2], OperationID: row[3], Title: strings.TrimSpace(row[4]), ServiceName: serviceName, ServiceID: serviceID, Score: score})
	}
	if len(out) == 0 {
		for _, line := range lines {
			if reZero.MatchString(strings.TrimSpace(line)) {
				return []Operation{}, nil
			}
		}
		return nil, errors.New("o catálogo Nexus devolveu uma pesquisa em formato não reconhecido")
	}
	return out, nil
}

func parseStructuredSearch(raw json.RawMessage) ([]Operation, bool) {
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return nil, false
	}
	var items []any
	switch x := value.(type) {
	case []any:
		items = x
	case map[string]any:
		for _, key := range []string{"operations", "results", "items"} {
			if rows, ok := x[key].([]any); ok {
				items = rows
				break
			}
		}
	}
	if items == nil {
		return nil, false
	}
	out := make([]Operation, 0, len(items))
	for _, item := range items {
		row, ok := item.(map[string]any)
		if !ok {
			return nil, false
		}
		op := Operation{
			Method:      stringValue(row, "method"),
			Path:        stringValue(row, "path", "url"),
			OperationID: stringValue(row, "operationId", "operation_id"),
			Title:       stringValue(row, "title", "summary", "name"),
			ServiceName: stringValue(row, "serviceName", "service", "service_name"),
			ServiceID:   stringValue(row, "serviceId", "service_id"),
		}
		if op.Method == "" || op.Path == "" || op.OperationID == "" || op.ServiceID == "" {
			return nil, false
		}
		if op.Method == "GET" || op.Method == "POST" || op.Method == "PUT" || op.Method == "PATCH" || op.Method == "DELETE" {
			out = append(out, op)
		} else {
			return nil, false
		}
	}
	return out, true
}

func parseDetailText(text string) (Detail, bool) {
	if strings.TrimSpace(text) == "" {
		return Detail{}, false
	}
	var out Detail
	out.Evidence = text
	lines := strings.Split(text, "\n")
	urls := map[string]bool{}
	summary := ""
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if out.Method == "" {
			if m := reHTTP.FindStringSubmatch(line); m != nil {
				out.Method, out.Path = m[1], strings.TrimSuffix(m[2], " ·")
				rest := strings.TrimSpace(strings.TrimPrefix(line, m[0]))
				rest = strings.TrimPrefix(rest, "· ")
				if parts := strings.SplitN(rest, " · ", 2); len(parts) > 0 {
					out.OperationID = strings.TrimSpace(parts[0])
					if len(parts) > 1 {
						out.Title = strings.TrimSpace(parts[1])
					}
				}
			}
		}
		if strings.HasPrefix(line, "Resumo:") {
			summary = strings.TrimSpace(strings.TrimPrefix(line, "Resumo:"))
		}
		if strings.HasPrefix(line, "Descrição:") {
			out.Description = strings.TrimSpace(strings.TrimPrefix(line, "Descrição:"))
		}
		if strings.HasPrefix(line, "- URL:") {
			if m := reURL.FindStringSubmatch(line); m != nil {
				urls[strings.TrimRight(m[1], ".,)")] = true
			}
			if out.Method == "" {
				if m := reHTTP.FindStringSubmatch(strings.TrimPrefix(line, "- ")); m != nil {
					out.Method, out.Path = m[1], m[2]
				}
			}
		}
	}
	const heading = "Definição OpenAPI da operação ($ref resolvidos):"
	if index := strings.Index(text, heading); index >= 0 {
		raw := strings.TrimSpace(text[index+len(heading):])
		var definition map[string]any
		decoder := json.NewDecoder(strings.NewReader(raw))
		if decoder.Decode(&definition) == nil {
			out.Params = paramsFromDefinition(definition)
			if title := stringValue(definition, "summary"); title != "" {
				out.Title = title
			}
			if description := stringValue(definition, "description"); description != "" {
				out.Description = description
			}
			pretty, err := json.MarshalIndent(definition, "", "  ")
			if err == nil {
				out.Definition = string(pretty)
			}
		}
	}
	if summary != "" {
		out.Title = summary
	}
	if len(urls) == 1 {
		for raw := range urls {
			out.URL = raw
		}
	}
	if out.Method == "" || out.Path == "" {
		return Detail{}, false
	}
	return out, true
}

func parseStructuredDetail(raw json.RawMessage) (Detail, bool) {
	if len(raw) == 0 {
		return Detail{}, false
	}
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return Detail{}, false
	}
	row, ok := value.(map[string]any)
	if !ok {
		return Detail{}, false
	}
	detail := Detail{
		Evidence:    string(raw),
		Method:      strings.ToUpper(stringValue(row, "method")),
		Path:        stringValue(row, "path"),
		URL:         stringValue(row, "url"),
		Title:       stringValue(row, "title", "summary"),
		Description: stringValue(row, "description"),
	}
	definition, hasDefinition := row["definition"]
	if !hasDefinition {
		definition, hasDefinition = row["openapi"]
	}
	if hasDefinition {
		if data, err := json.MarshalIndent(definition, "", "  "); err == nil {
			detail.Definition = string(data)
		}
		if object, ok := definition.(map[string]any); ok {
			detail.Params = paramsFromDefinition(object)
		}
	}
	if detail.Method == "" || detail.Path == "" {
		return Detail{}, false
	}
	return detail, true
}

func paramsFromDefinition(definition map[string]any) []okf.ParamMeta {
	raw, ok := definition["parameters"].([]any)
	if !ok {
		return []okf.ParamMeta{}
	}
	out := make([]okf.ParamMeta, 0, len(raw))
	for _, item := range raw {
		param, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name := stringValue(param, "name")
		if name == "" {
			continue
		}
		schema, _ := param["schema"].(map[string]any)
		typeName := stringValue(schema, "type")
		description := stringValue(param, "description")
		if typeName == "integer" && !strings.Contains(strings.ToLower(description), "inteiro") {
			if description != "" {
				description += " "
			}
			description += "(inteiro)"
		}
		if location := stringValue(param, "in"); location != "" {
			if description != "" {
				description = strings.ToLower(location) + ": " + description
			} else {
				description = strings.ToLower(location)
			}
		}
		out = append(out, okf.ParamMeta{Name: name, Type: typeName, Required: boolValue(param["required"]), Description: description})
	}
	return out
}

func stringValue(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := values[key].(string); ok {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func boolValue(value any) bool {
	result, _ := value.(bool)
	return result
}
