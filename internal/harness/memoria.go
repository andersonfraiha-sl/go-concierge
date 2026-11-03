package harness

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"

	"go-concierge/internal/memoria"
)

// Memória relacional no harness: o Contexto da Config leva os fatos conhecidos do
// contato ao prompt de toda a família, e o ExtratorLLM captura fatos novos a cada
// troca, com o modelo barato (MODEL_EXTRACT, senão MODEL_CHEAP, senão o do provedor).

// SecaoMemoria é o título da seção que entra na instrução de cada agente.
const SecaoMemoria = "# Memória sobre o morador (fatos que ele mesmo informou em conversas anteriores; use sem perguntar de novo)"

// ContextoDeMemoria formata a memória para a Config.Contexto.
func ContextoDeMemoria(m memoria.Memoria) string {
	t := m.Texto()
	if t == "" {
		return ""
	}
	return SecaoMemoria + "\n\n" + t
}

// ExtratorLLM pede ao modelo os fatos duráveis de uma troca.
type ExtratorLLM struct {
	gerar func(ctx context.Context, msgs []*schema.Message) (*schema.Message, error)
}

// NovoExtrator devolve o extrator conforme o provedor: Regras sem modelo (mock),
// LLM com o modelo barato quando há chave.
func NovoExtrator(ctx context.Context, cfg Config) memoria.Extrator {
	prov := ResolverProvedor(cfg.env)
	chave := cfg.APIKey
	if chave == "" {
		chave = prov.Chave
	}
	if cfg.Mock || chave == "" || prov.Nome == "mock" {
		return memoria.Regras{}
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
		return memoria.Regras{}
	}
	return &ExtratorLLM{gerar: func(ctx context.Context, msgs []*schema.Message) (*schema.Message, error) {
		return m.Generate(ctx, msgs)
	}}
}

const promptExtrator = `Você mantém a memória do concierge de um condomínio sobre um morador. Dada uma troca
(mensagem do morador e resposta do concierge), extraia só FATOS DURÁVEIS que o próprio
morador afirmou sobre si: quantas unidades tem e quais, como prefere ser chamado, pets,
veículos, pessoas da casa, horários e preferências estáveis, restrições. NÃO extraia:
pedidos pontuais, perguntas, o assunto da conversa, nada dito pelo concierge, nada incerto.
Chaves estáveis em snake_case (unidades, nome_preferido, pets, veiculo, horario_preferido,
familia, restricoes...). Se já existe um fato com a mesma chave e o morador o corrigiu, repita
a chave com o texto novo. Responda SÓ com JSON: {"fatos":[{"chave":"...","texto":"...","confianca":0.0}]}
ou {"fatos":[]} quando não houver nada durável.`

func (e *ExtratorLLM) Extrair(ctx context.Context, mensagem, resposta string, atual memoria.Memoria) ([]memoria.Fato, error) {
	var b strings.Builder
	if t := atual.Texto(); t != "" {
		b.WriteString("Memória atual:\n" + t + "\n\n")
	}
	b.WriteString("Mensagem do morador:\n" + mensagem + "\n\nResposta do concierge:\n" + resposta)
	m, err := e.gerar(ctx, []*schema.Message{schema.SystemMessage(promptExtrator), schema.UserMessage(b.String())})
	if err != nil {
		return nil, err
	}
	texto := strings.TrimSpace(m.Content)
	texto = strings.TrimPrefix(strings.TrimSuffix(strings.TrimPrefix(texto, "```json"), "```"), "```")
	var out struct {
		Fatos []memoria.Fato `json:"fatos"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(texto)), &out); err != nil {
		return nil, nil // resposta fora do formato: nada a guardar
	}
	var fatos []memoria.Fato
	for _, f := range out.Fatos {
		if f.Confianca == 0 || f.Confianca >= 0.6 {
			fatos = append(fatos, f)
		}
	}
	return fatos, nil
}
