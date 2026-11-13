package harness

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"

	"go-concierge/internal/okf"
)

// Sugestões com o modelo barato: o Backdoor e o Admin pedem ao modelo para preencher
// formulários a partir de uma descrição em linguagem natural (hoje, o payload de um
// recurso por condomínio). Mesma via do extrator de memória: MODEL_EXTRACT, senão
// MODEL_CHEAP, senão o modelo do provedor; sem chave (mock) não há sugestão.

// modeloBarato monta o chat model barato, ou ok=false quando não há provedor.
func modeloBarato(ctx context.Context, cfg Config) (func(ctx context.Context, msgs []*schema.Message) (*schema.Message, error), bool) {
	prov := ResolverProvedor(cfg.env)
	chave := cfg.APIKey
	if chave == "" {
		chave = prov.Chave
	}
	if cfg.Mock || prov.semAutenticacao(chave) || prov.Nome == "mock" {
		return nil, false
	}
	modelo := cfg.env("MODEL_EXTRACT")
	if modelo == "" {
		modelo = cfg.env("MODEL_CHEAP")
	}
	if modelo == "" {
		modelo = prov.Modelo
	}
	base := cfg.BaseURL
	if base == "" {
		base = prov.BaseURL
	}
	m, err := novoModelo(ctx, prov, chave, base, modelo, 60*time.Second)
	if err != nil {
		return nil, false
	}
	return func(ctx context.Context, msgs []*schema.Message) (*schema.Message, error) {
		return m.Generate(ctx, msgs)
	}, true
}

// Sugestao é o que o modelo devolve: o payload proposto e uma nota curta sobre o que
// não coube nos campos.
type Sugestao struct {
	Payload map[string]any `json:"payload"`
	Nota    string         `json:"nota,omitempty"`
}

const promptSugestao = `Você ajuda a administradora de um condomínio a preencher as definições de um recurso
do concierge. Recebe o recurso (o que faz, parâmetros, campos de ajuste com rótulo, tipo
e padrão), as definições já gravadas e um texto livre da administradora. Devolva as
definições como um objeto JSON plano, usando EXATAMENTE as chaves dos campos de ajuste
quando o texto falar deles (converta "dois dias" em 2, "sim/não" em true/false, respeite
as opções de um campo select). Só crie uma chave fora dos campos de ajuste se o texto
pedir claramente um parâmetro padrão do recurso (use o nome do parâmetro). Não invente
valores que o texto não dá; mantenha os já gravados que o texto não contradiz. Responda
SÓ com JSON: {"payload": {...}, "nota": "o que não coube ou ficou em dúvida, em uma frase"}.`

// SugerirPayload pede ao modelo as definições de um recurso a partir do texto da
// administradora.
func SugerirPayload(ctx context.Context, cfg Config, r okf.Recurso, atual map[string]any, texto string) (Sugestao, error) {
	gerar, ok := modeloBarato(ctx, cfg)
	if !ok {
		return Sugestao{}, errors.New("sem modelo configurado: preencha à mão")
	}
	texto = strings.TrimSpace(texto)
	if texto == "" {
		return Sugestao{}, errors.New("descreva em uma frase o que vale neste condomínio")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Recurso: %s (%s)\nO que faz: %s\n", r.Name, r.Titulo, strings.TrimSpace(r.Description))
	if len(r.Params) > 0 {
		b.WriteString("Parâmetros:\n")
		for _, p := range r.Params {
			fmt.Fprintf(&b, "- %s (%s)%s %s\n", p.Name, p.Type, map[bool]string{true: " obrigatório", false: ""}[p.Required], p.Description)
		}
	}
	if len(r.Definicoes) > 0 {
		b.WriteString("Campos de ajuste:\n")
		for _, d := range r.Definicoes {
			fmt.Fprintf(&b, "- %s: %q (%s)", d.Name, d.Label, d.Type)
			if len(d.Options) > 0 {
				fmt.Fprintf(&b, " opções: %s", strings.Join(d.Options, " | "))
			}
			if d.Default != nil {
				fmt.Fprintf(&b, " padrão: %v", d.Default)
			}
			if d.Description != "" {
				fmt.Fprintf(&b, " — %s", d.Description)
			}
			b.WriteString("\n")
		}
	}
	if len(atual) > 0 {
		raw, _ := json.Marshal(atual)
		fmt.Fprintf(&b, "Definições já gravadas: %s\n", raw)
	}
	fmt.Fprintf(&b, "\nTexto da administradora:\n%s", texto)
	m, err := gerar(ctx, []*schema.Message{schema.SystemMessage(promptSugestao), schema.UserMessage(b.String())})
	if err != nil {
		return Sugestao{}, err
	}
	saida := strings.TrimSpace(m.Content)
	saida = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(saida, "```json"), "```"), "```"))
	var out Sugestao
	if err := json.Unmarshal([]byte(saida), &out); err != nil {
		return Sugestao{}, fmt.Errorf("o modelo não devolveu JSON: %s", corta(saida, 200))
	}
	if out.Payload == nil {
		out.Payload = map[string]any{}
	}
	return out, nil
}
