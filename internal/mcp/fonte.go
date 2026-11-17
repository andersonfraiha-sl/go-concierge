package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"go-concierge/internal/okf"
)

// Recursos converte as tools do servidor em recursos do Nexus (classe api). Tudo é
// privado: as operações do BFF são de um morador identificado. Escrita vem da
// anotação readOnlyHint; sem anotação, pelo verbo do nome da operação.
func Recursos(tools []Tool) []okf.Recurso {
	out := make([]okf.Recurso, 0, len(tools))
	for _, t := range tools {
		r := okf.Recurso{
			Name:        t.Name,
			Titulo:      titulo(t),
			Kind:        okf.KindAPI,
			Visibility:  "private",
			Write:       escrita(t),
			Description: strings.TrimSpace(t.Description),
			Params:      params(t.InputSchema),
			Output:      "resultado da operação (JSON do BFF)",
			Origem:      okf.OrigemMCP,
		}
		if len(t.InputSchema) > 0 {
			var s any
			if json.Unmarshal(t.InputSchema, &s) == nil {
				r.Schema = s
			}
		}
		if r.Description == "" {
			r.Description = "Operação " + t.Name + " do catálogo MCP."
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func titulo(t Tool) string {
	if t.Title != "" {
		return t.Title
	}
	if t.Annotations != nil && t.Annotations.Title != "" {
		return t.Annotations.Title
	}
	return ""
}

// escrita: readOnlyHint decide; sem ela, o nome da operação diz.
func escrita(t Tool) bool {
	if t.Annotations != nil && t.Annotations.ReadOnlyHint != nil {
		return !*t.Annotations.ReadOnlyHint
	}
	return okf.EscritaPeloNome(t.Name)
}

// params achata o nível de cima do inputSchema em ParamMeta; objetos e listas levam
// um resumo da estrutura na descrição, para o manual da projeção.
func params(schema json.RawMessage) []okf.ParamMeta {
	out := []okf.ParamMeta{}
	if len(schema) == 0 {
		return out
	}
	var s struct {
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
	}
	if json.Unmarshal(schema, &s) != nil {
		return out
	}
	req := map[string]bool{}
	for _, r := range s.Required {
		req[r] = true
	}
	names := make([]string, 0, len(s.Properties))
	for n := range s.Properties {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		tipo, desc := resumo(s.Properties[n], 0)
		out = append(out, okf.ParamMeta{Name: n, Type: tipo, Required: req[n], Description: desc})
	}
	return out
}

// resumo devolve o tipo e uma descrição com a estrutura (até 2 níveis).
func resumo(raw json.RawMessage, nivel int) (string, string) {
	var p struct {
		Type        any                        `json:"type"`
		Description string                     `json:"description"`
		Enum        []any                      `json:"enum"`
		Items       json.RawMessage            `json:"items"`
		Properties  map[string]json.RawMessage `json:"properties"`
		Required    []string                   `json:"required"`
		Format      string                     `json:"format"`
	}
	if json.Unmarshal(raw, &p) != nil {
		return "string", ""
	}
	tipo := tipoDe(p.Type)
	desc := strings.TrimSpace(p.Description)
	extra := ""
	switch {
	case len(p.Enum) > 0:
		vals := make([]string, 0, len(p.Enum))
		for _, e := range p.Enum {
			vals = append(vals, fmt.Sprint(e))
		}
		extra = "um de: " + strings.Join(vals, " | ")
	case tipo == "object" && len(p.Properties) > 0 && nivel < 2:
		req := map[string]bool{}
		for _, r := range p.Required {
			req[r] = true
		}
		names := make([]string, 0, len(p.Properties))
		for n := range p.Properties {
			names = append(names, n)
		}
		sort.Strings(names)
		campos := make([]string, 0, len(names))
		for _, n := range names {
			t, _ := resumo(p.Properties[n], nivel+1)
			mark := ""
			if req[n] {
				mark = "*"
			}
			campos = append(campos, n+mark+": "+t)
		}
		extra = "objeto {" + strings.Join(campos, ", ") + "}"
	case tipo == "array" && len(p.Items) > 0 && nivel < 2:
		t, d := resumo(p.Items, nivel+1)
		extra = "lista de " + t
		if d != "" && strings.HasPrefix(d, "objeto") {
			extra = "lista de " + d
		}
	case p.Format != "":
		extra = "formato " + p.Format
	}
	switch {
	case desc != "" && extra != "":
		desc = desc + " (" + extra + ")"
	case desc == "":
		desc = extra
	}
	return tipo, desc
}

func tipoDe(t any) string {
	switch v := t.(type) {
	case string:
		if v == "integer" {
			return "number"
		}
		return v
	case []any:
		for _, x := range v {
			if s, ok := x.(string); ok && s != "null" {
				return tipoDe(s)
			}
		}
	}
	return "string"
}

// Fonte devolve a função de recursos que o catálogo usa: lista as tools com cache
// (ttl) e, numa falha, mantém a última lista boa (ou vazia, com o erro no log).
func Fonte(c *Client, ttl time.Duration, logf func(string, ...any)) func() []okf.Recurso {
	if c == nil {
		return nil
	}
	if ttl <= 0 {
		ttl = time.Minute
	}
	var (
		mu     sync.Mutex
		cache  []okf.Recurso
		quando time.Time
		falha  string
	)
	return func() []okf.Recurso {
		mu.Lock()
		defer mu.Unlock()
		if cache != nil && time.Since(quando) < ttl {
			return cache
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		tools, err := c.ListTools(ctx)
		if err != nil {
			if msg := err.Error(); msg != falha && logf != nil {
				logf("mcp: catálogo indisponível (%s): %v", c.URL, err)
			}
			falha = err.Error()
			if cache == nil {
				cache = []okf.Recurso{}
			}
			quando = time.Now().Add(-ttl + 10*time.Second) // tenta de novo em 10 s
			return cache
		}
		if falha != "" && logf != nil {
			logf("mcp: catálogo de volta (%d tools)", len(tools))
		}
		falha = ""
		cache, quando = Recursos(tools), time.Now()
		return cache
	}
}

// Combinar junta fontes de recursos; pelo nome, a primeira vence. Fontes nil são
// ignoradas; sem nenhuma, devolve nil (o catálogo sabe que não há como conferir).
func Combinar(fontes ...func() []okf.Recurso) func() []okf.Recurso {
	var ativas []func() []okf.Recurso
	for _, f := range fontes {
		if f != nil {
			ativas = append(ativas, f)
		}
	}
	switch len(ativas) {
	case 0:
		return nil
	case 1:
		return ativas[0]
	}
	return func() []okf.Recurso {
		vistos := map[string]bool{}
		var out []okf.Recurso
		for _, f := range ativas {
			for _, r := range f() {
				if vistos[r.Name] {
					continue
				}
				vistos[r.Name] = true
				out = append(out, r)
			}
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
		return out
	}
}
