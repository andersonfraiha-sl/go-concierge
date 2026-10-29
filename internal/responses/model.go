package responses

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type Model struct {
	URL    string
	Token  string
	Name   string
	Client *http.Client
	tools  []*schema.ToolInfo
}

var _ model.ToolCallingChatModel = (*Model)(nil)

func (m *Model) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	clone := *m
	clone.tools = append([]*schema.ToolInfo(nil), tools...)
	return &clone, nil
}

func (m *Model) Stream(ctx context.Context, messages []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	message, err := m.Generate(ctx, messages, opts...)
	if err != nil {
		return nil, err
	}
	return schema.StreamReaderFromArray([]*schema.Message{message}), nil
}

func (m *Model) Generate(ctx context.Context, messages []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	options := model.GetCommonOptions(&model.Options{}, opts...)
	modelName := m.Name
	if options.Model != nil {
		modelName = *options.Model
	}
	input := make([]any, 0, len(messages))
	for _, message := range messages {
		if message == nil {
			continue
		}
		if len(message.MultiContent) > 0 || len(message.UserInputMultiContent) > 0 || len(message.AssistantGenMultiContent) > 0 {
			return nil, fmt.Errorf("responses: conteúdo multimodal não suportado")
		}
		switch message.Role {
		case schema.System, schema.User:
			input = append(input, map[string]any{"role": string(message.Role), "content": message.Content})
		case schema.Assistant:
			if items, ok := message.Extra["responses_output"].([]json.RawMessage); ok {
				for _, item := range items {
					input = append(input, item)
				}
				continue
			}
			if message.Content != "" {
				input = append(input, map[string]any{"role": "assistant", "content": message.Content})
			}
			for _, call := range message.ToolCalls {
				input = append(input, map[string]any{"type": "function_call", "call_id": call.ID, "name": call.Function.Name, "arguments": call.Function.Arguments})
			}
		case schema.Tool:
			input = append(input, map[string]any{"type": "function_call_output", "call_id": message.ToolCallID, "output": message.Content})
		default:
			return nil, fmt.Errorf("responses: papel não suportado: %s", message.Role)
		}
	}
	body := map[string]any{"model": modelName, "input": input, "store": false, "reasoning": map[string]any{"effort": "medium"}, "max_output_tokens": 32000, "include": []string{"reasoning.encrypted_content"}}
	if options.MaxTokens != nil {
		body["max_output_tokens"] = *options.MaxTokens
	}
	if options.Temperature != nil {
		body["temperature"] = *options.Temperature
	}
	if options.TopP != nil {
		body["top_p"] = *options.TopP
	}
	if len(options.Stop) > 0 {
		return nil, fmt.Errorf("responses: opção stop não suportada")
	}
	tools := m.tools
	if options.Tools != nil {
		tools = options.Tools
	}
	if len(tools) > 0 {
		defs := make([]any, 0, len(tools))
		for _, tool := range tools {
			if tool == nil {
				continue
			}
			params, err := tool.ParamsOneOf.ToJSONSchema()
			if err != nil {
				return nil, fmt.Errorf("responses: schema da ferramenta %s: %w", tool.Name, err)
			}
			var p any = params
			if params == nil {
				p = map[string]any{"type": "object", "properties": map[string]any{}}
			}
			defs = append(defs, map[string]any{"type": "function", "name": tool.Name, "description": tool.Desc, "parameters": p, "strict": false})
		}
		body["tools"] = defs
	}
	if options.ToolChoice != nil {
		switch *options.ToolChoice {
		case schema.ToolChoiceForbidden:
			body["tool_choice"] = "none"
		case schema.ToolChoiceAllowed:
			body["tool_choice"] = "auto"
		case schema.ToolChoiceForced:
			body["tool_choice"] = "required"
		default:
			return nil, fmt.Errorf("responses: escolha de ferramenta não suportada")
		}
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("responses: codificar requisição: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(m.URL, "/")+"/responses", bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("responses: criar requisição: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+m.Token)
	req.Header.Set("x-company-id", "internal")
	req.Header.Set("x-origem", "workspace")
	client := m.Client
	if client == nil {
		client = &http.Client{Timeout: 90 * time.Second}
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("responses: requisição: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("responses: gateway retornou HTTP %d", res.StatusCode)
	}
	var response struct {
		Status string            `json:"status"`
		Output []json.RawMessage `json:"output"`
		Usage  struct {
			Input  int `json:"input_tokens"`
			Output int `json:"output_tokens"`
			Total  int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 16<<20)).Decode(&response); err != nil {
		return nil, fmt.Errorf("responses: resposta inválida: %w", err)
	}
	if response.Status != "completed" {
		return nil, fmt.Errorf("responses: resposta não concluída: %s", response.Status)
	}
	var text strings.Builder
	var calls []schema.ToolCall
	for _, rawItem := range response.Output {
		var item struct {
			Type      string `json:"type"`
			CallID    string `json:"call_id"`
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
			Content   []struct {
				Type    string `json:"type"`
				Text    string `json:"text"`
				Refusal string `json:"refusal"`
			} `json:"content"`
		}
		if err := json.Unmarshal(rawItem, &item); err != nil {
			return nil, fmt.Errorf("responses: item inválido: %w", err)
		}
		switch item.Type {
		case "function_call":
			if item.CallID == "" || item.Name == "" {
				return nil, fmt.Errorf("responses: chamada de ferramenta incompleta")
			}
			calls = append(calls, schema.ToolCall{ID: item.CallID, Type: "function", Function: schema.FunctionCall{Name: item.Name, Arguments: item.Arguments}})
		case "message":
			for _, part := range item.Content {
				if part.Type == "output_text" {
					text.WriteString(part.Text)
				}
				if part.Type == "refusal" {
					return nil, fmt.Errorf("responses: modelo recusou a solicitação")
				}
			}
		}
	}
	if text.Len() == 0 && len(calls) == 0 {
		return nil, fmt.Errorf("responses: resposta sem texto nem chamada de ferramenta")
	}
	msg := schema.AssistantMessage(text.String(), calls)
	msg.Extra = map[string]any{"responses_output": response.Output}
	msg.ResponseMeta = &schema.ResponseMeta{FinishReason: "stop", Usage: &schema.TokenUsage{PromptTokens: response.Usage.Input, CompletionTokens: response.Usage.Output, TotalTokens: response.Usage.Total}}
	return msg, nil
}
