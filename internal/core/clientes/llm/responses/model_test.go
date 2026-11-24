package responses

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestResponsesToolRoundTrip(t *testing.T) {
	var requests []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" || r.Header.Get("Authorization") != "Bearer teste" || r.Header.Get("x-company-id") != "internal" || r.Header.Get("x-origem") != "workspace" {
			t.Error("requisição ou headers incorretos")
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		requests = append(requests, body)
		w.Header().Set("Content-Type", "application/json")
		if len(requests) == 1 {
			_, _ = w.Write([]byte(`{"status":"completed","output":[{"type":"reasoning","id":"rs_1","encrypted_content":"enc","summary":[]},{"type":"function_call","call_id":"call_1","name":"consultar","arguments":"{\"id\":\"1\"}"}],"usage":{"input_tokens":10,"output_tokens":4,"total_tokens":14}}`))
		} else {
			_, _ = w.Write([]byte(`{"status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"Concluído"}]}],"usage":{"input_tokens":20,"output_tokens":2,"total_tokens":22}}`))
		}
	}))
	defer server.Close()
	m := &Model{URL: server.URL + "/v1", Token: "teste", Name: "luna"}
	bound, err := m.WithTools([]*schema.ToolInfo{{Name: "consultar", Desc: "consulta", ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{"id": {Type: schema.String, Required: true}})}})
	if err != nil {
		t.Fatal(err)
	}
	history := []*schema.Message{schema.SystemMessage("instruções"), schema.UserMessage("consulte")}
	stream, err := bound.Stream(context.Background(), history)
	if err != nil {
		t.Fatal(err)
	}
	first, err := schema.ConcatMessageStream(stream)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.ToolCalls) != 1 || first.ToolCalls[0].ID != "call_1" || first.ResponseMeta.Usage.TotalTokens != 14 {
		t.Fatalf("resposta inesperada: %+v", first)
	}
	history = append(history, first, schema.ToolMessage("resultado", "call_1"))
	second, err := bound.Generate(context.Background(), history)
	if err != nil || second.Content != "Concluído" {
		t.Fatalf("continuação: %+v, %v", second, err)
	}
	firstReq := requests[0]
	if firstReq["model"] != "luna" || firstReq["store"] != false || len(firstReq["tools"].([]any)) != 1 {
		t.Fatal("configuração da chamada incorreta")
	}
	input := requests[1]["input"].([]any)
	if len(input) != 5 || input[2].(map[string]any)["type"] != "reasoning" || input[3].(map[string]any)["call_id"] != "call_1" || input[4].(map[string]any)["type"] != "function_call_output" {
		t.Fatalf("histórico incompleto: %+v", input)
	}
}

func TestResponsesErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		code int
		body string
	}{
		{"http", 401, `segredo`},
		{"incompleta", 200, `{"status":"incomplete","output":[]}`},
		{"vazia", 200, `{"status":"completed","output":[]}`},
		{"recusa", 200, `{"status":"completed","output":[{"type":"message","content":[{"type":"refusal"}]}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.code); _, _ = w.Write([]byte(tc.body)) }))
			defer server.Close()
			_, err := (&Model{URL: server.URL, Token: "segredo", Name: "luna"}).Generate(context.Background(), []*schema.Message{schema.UserMessage("oi")})
			if err == nil || strings.Contains(err.Error(), "segredo") {
				t.Fatalf("erro inesperado: %v", err)
			}
		})
	}
}
