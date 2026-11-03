package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"

	"go-concierge/internal/pedidos"
)

// ExtratorPedidosLLM mapeia a mensagem do morador nos pedidos (casos de uso com os
// parâmetros do caso.yaml) com o modelo barato. Só extrai: quem decide estado, valida e
// executa é o motor de pedidos, em código.
type ExtratorPedidosLLM struct {
	gerar func(ctx context.Context, msgs []*schema.Message) (*schema.Message, error)
}

// NovoExtratorPedidos: Regras sem modelo (mock), LLM com o modelo barato quando há chave.
func NovoExtratorPedidos(ctx context.Context, cfg Config) pedidos.Extrator {
	prov := ResolverProvedor(cfg.env)
	chave := cfg.APIKey
	if chave == "" {
		chave = prov.Chave
	}
	if cfg.Mock || chave == "" || prov.Nome == "mock" {
		return pedidos.Regras{}
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
		return pedidos.Regras{}
	}
	return &ExtratorPedidosLLM{gerar: func(ctx context.Context, msgs []*schema.Message) (*schema.Message, error) {
		return m.Generate(ctx, msgs)
	}}
}

const promptPedidos = `Você é o detector de PEDIDOS do concierge de um condomínio (WhatsApp). Dada a mensagem do
morador, os casos de uso disponíveis (com os parâmetros de cada um) e os pedidos já abertos
deste morador, diga SÓ o que a mensagem contém. Responda SÓ com JSON:
{"pedidos":[{"protocolo":"P-XXXXX se for um pedido aberto, senão vazio","caso":"id do caso","valores":{"slot":"valor"},"trecho":"as palavras do morador sobre ESTE pedido"}],
 "confirmacoes":[{"protocolo":"P-XXXXX","resposta":"sim|nao"}],
 "cancelar":["P-XXXXX"],
 "atendente":false,
 "restante":"o que NÃO é pedido (dúvida, saudação, conversa), em uma frase, ou vazio"}
Regras:
- pedido é uma AÇÃO que o morador quer que seja feita (reservar, liberar, comunicar, agendar,
  consultar). PERGUNTA sobre regra, permissão ou horário ("posso?", "pode?", "é permitido?",
  "até que horas?") NÃO é pedido: vai em restante, com as palavras do morador;
- um pedido por intenção; vários pedidos numa mensagem viram vários itens, na ordem;
- "quero fazer tudo isso", "todos", "os dois", "esses aí": o morador se refere à lista de
  serviços que o concierge acabou de enumerar (últimos turnos). Abra um pedido para cada
  CASO de ação daquela lista (reservar, liberar, agendar, comunicar, atualizar…), na ordem;
  dúvidas (regimento, horários) não viram pedido. Sem lista anterior, vai em restante;
- trecho: copie as palavras do morador que falam DESTE pedido (numa mensagem com dois pedidos,
  cada item leva só a sua parte; nunca a mensagem inteira em todos);
- só abra um pedido quando um caso de fato cobre o que o morador quer. Serviço que nenhum caso
  oferece (ex.: cadastrar um novo morador, se não há caso para isso) NÃO vira pedido do caso
  "parecido": vai em restante, com as palavras do morador, para o concierge explicar;
- se existe um pedido ABERTO do mesmo caso, use o protocolo dele (não abra outro) e ponha em
  valores só o que a mensagem traz; a resposta a uma pergunta em aberto vai no slot da pergunta;
- valores: use o nome EXATO do slot; datas em AAAA-MM-DD (hoje é %s, amanhã é %s); opções pelo valor ou rótulo;
  listas como array; sim/não como true/false; NUNCA invente valor que o morador não disse nesta
  mensagem (nem a partir do histórico, nem "o mais provável"); na dúvida, deixe de fora;
- pedidos recorrentes de liberação de visitante não cabem neste caso: não abra pedido e deixe a
  mensagem em restante para encaminhamento ao app Gruvi;
- confirmacoes: só quando um pedido espera confirmação e o morador respondeu sim ou não;
- cancelar: só se o morador desistiu de um pedido aberto;
- atendente: true se o morador pede humano/atendente/pessoa;
- restante: SÓ as palavras do morador que não são pedido (dúvida, saudação, conversa), nunca
  uma resposta sua, nunca uma pergunta ao morador; vazio se tudo era pedido.

## Casos de uso
%s
## Pedidos abertos deste morador
%s`

func (e *ExtratorPedidosLLM) Extrair(ctx context.Context, in pedidos.Entrada) (pedidos.Extracao, error) {
	agora := in.Agora
	if agora.IsZero() {
		agora = time.Now()
	}
	var casos strings.Builder
	for _, c := range in.Casos {
		fmt.Fprintf(&casos, "- %s (%s): %s", c.ID, c.Nome, strings.TrimSpace(c.Descricao))
		if len(c.Exemplos) > 0 {
			fmt.Fprintf(&casos, " Ex.: %s.", strings.Join(c.Exemplos, "; "))
		}
		casos.WriteString("\n  slots:")
		for _, s := range c.Slots {
			obr := "opcional"
			if s.Obrigatorio {
				obr = "obrigatório"
			}
			fmt.Fprintf(&casos, " %s (%s, %s", s.Nome, s.Tipo, obr)
			if len(s.Opcoes) > 0 {
				var ops []string
				for _, o := range s.Opcoes {
					ops = append(ops, o.Valor)
				}
				fmt.Fprintf(&casos, ", opções: %s", strings.Join(ops, "|"))
			}
			casos.WriteString(");")
		}
		casos.WriteString("\n")
	}
	abertos := "(nenhum)\n"
	if len(in.Abertos) > 0 {
		var b strings.Builder
		for _, p := range in.Abertos {
			vals, _ := json.Marshal(p.Valores)
			fmt.Fprintf(&b, "- %s: caso %s, estado %s, valores %s", p.Protocolo, p.Caso, p.Estado, vals)
			if p.Pendencia != nil {
				fmt.Fprintf(&b, ", pergunta em aberto sobre o slot %q: %q", p.Pendencia.Slot, p.Pendencia.Pergunta)
			}
			if p.Estado == pedidos.Confirmando && p.Confirmacao != nil {
				fmt.Fprintf(&b, ", aguarda confirmação: %q", p.Confirmacao.Texto)
			}
			b.WriteString("\n")
		}
		abertos = b.String()
	}
	sys := fmt.Sprintf(promptPedidos, agora.Format("2006-01-02"), agora.AddDate(0, 0, 1).Format("2006-01-02"), casos.String(), abertos)
	var u strings.Builder
	if len(in.Historico) > 0 {
		u.WriteString("Últimos turnos:\n" + strings.Join(in.Historico, "\n") + "\n\n")
	}
	u.WriteString("Mensagem do morador: " + in.Mensagem)
	m, err := e.gerar(ctx, []*schema.Message{schema.SystemMessage(sys), schema.UserMessage(u.String())})
	if err != nil {
		return pedidos.Extracao{}, err
	}
	var out pedidos.Extracao
	if err := json.Unmarshal([]byte(extrairJSON(m.Content)), &out); err != nil {
		return pedidos.Extracao{}, fmt.Errorf("extrator de pedidos: resposta inválida: %w", err)
	}
	// o sim/não a uma confirmação pendente é decidido em código, por cima do modelo
	for _, p := range in.Abertos {
		if p.Estado != pedidos.Confirmando {
			continue
		}
		ja := false
		for _, c := range out.Confirmacoes {
			if c.Protocolo == p.Protocolo {
				ja = true
			}
		}
		if ja {
			continue
		}
		switch {
		case pedidos.Confirmado(in.Mensagem):
			out.Confirmacoes = append(out.Confirmacoes, pedidos.Confirmada{Protocolo: p.Protocolo, Resposta: "sim"})
		case pedidos.Negado(in.Mensagem):
			out.Confirmacoes = append(out.Confirmacoes, pedidos.Confirmada{Protocolo: p.Protocolo, Resposta: "nao"})
		}
	}
	return out, nil
}
