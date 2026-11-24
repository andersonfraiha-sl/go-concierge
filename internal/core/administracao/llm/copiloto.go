package administracaollm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"

	"go-concierge/internal/core/administracao/copiloto"
)

// Copiloto do Studio: conversa com quem configura o Concierge e propõe operações tipadas
// (internal/copiloto) que o servidor filtra antes de mostrar. Modelo: MODEL_COPILOTO,
// senão o padrão do provedor (MODEL_PLANNER, senão o do provedor).

// prazoDoModelo é o teto da chamada HTTP ao provedor; o servidor ainda impõe o seu.
const prazoDoModelo = 55 * time.Second

// MensagemDoCopiloto é uma fala da conversa; Papel é "pessoa" ou "copiloto".
type MensagemDoCopiloto struct {
	Papel string `json:"papel"`
	Texto string `json:"texto"`
}

const (
	PapelPessoa   = "pessoa"
	PapelCopiloto = "copiloto"
)

// PedidoAoCopiloto: Contexto é montado pelo servidor; Operacoes são as que a tela aceita.
type PedidoAoCopiloto struct {
	Mensagem  string
	Historico []MensagemDoCopiloto
	Contexto  string
	Operacoes []copiloto.TipoDeOperacao
}

type RespostaDoModelo struct {
	Mensagem  string              `json:"mensagem"`
	Operacoes []copiloto.Operacao `json:"operacoes"`
}

type Copiloto func(ctx context.Context, p PedidoAoCopiloto) (RespostaDoModelo, error)

var errSemMensagem = errors.New("resposta do modelo sem mensagem")

// NovoCopiloto monta o Copiloto com o modelo do provedor; false quando não há provedor
// (mock ou sem chave).
func NovoCopiloto(cfg Config) (Copiloto, bool) {
	if cfg.Mock || cfg.Modelos == nil {
		return nil, false
	}
	m, _, provedor, disponivel, err := cfg.Modelos.Criar(context.Background(), "copiloto", "", prazoDoModelo)
	if err != nil {
		// sem cliente não há Copiloto: o painel mostra "Indisponível" (503), e o motivo fica no log
		log.Printf("copiloto: montar modelo %s: %v", provedor, err)
		return nil, false
	}
	if !disponivel {
		return nil, false
	}
	return func(ctx context.Context, p PedidoAoCopiloto) (RespostaDoModelo, error) {
		resp, err := m.Generate(ctx, mensagensAoModelo(p))
		if err != nil {
			return RespostaDoModelo{}, fmt.Errorf("copiloto: chamar modelo: %w", err)
		}
		return LerRespostaDoModelo(resp.Content)
	}, true
}

// LerRespostaDoModelo aceita o JSON puro ou dentro de uma cerca ```json. O erro não traz o
// texto do modelo, que pode repetir o que a pessoa escreveu.
func LerRespostaDoModelo(raw string) (RespostaDoModelo, error) {
	var out RespostaDoModelo
	if err := json.Unmarshal([]byte(semCerca(raw)), &out); err != nil {
		return RespostaDoModelo{}, fmt.Errorf("resposta do modelo não é JSON: %w", err)
	}
	if strings.TrimSpace(out.Mensagem) == "" {
		return RespostaDoModelo{}, errSemMensagem
	}
	return out, nil
}

func semCerca(raw string) string {
	s := strings.TrimSpace(raw)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	_, corpo, _ := strings.Cut(s, "\n") // descarta a linha da cerca, com ou sem linguagem
	return strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(corpo), "```"))
}

func mensagensAoModelo(p PedidoAoCopiloto) []*schema.Message {
	msgs := []*schema.Message{schema.SystemMessage(promptDoCopiloto(p))}
	for _, h := range p.Historico {
		if h.Papel == PapelCopiloto {
			msgs = append(msgs, schema.AssistantMessage(h.Texto, nil))
			continue
		}
		msgs = append(msgs, schema.UserMessage(h.Texto))
	}
	return append(msgs, schema.UserMessage(p.Mensagem))
}

const promptBaseDoCopiloto = `Você é o Copiloto do Studio da Superlógica. Quem fala com você configura o Concierge,
o atendente virtual que conversa com os moradores dos condomínios pelo WhatsApp. Você ajuda a
entender e a mudar o que está na tela: skills (o que o Concierge sabe fazer), o fluxo de cada
skill (blocos em ordem), os condomínios (orientação e ajustes) e o
jeito de falar do Concierge.

Fale em português do Brasil, simples e curto, como um colega. Use só estas palavras para as
coisas: Concierge, skill, fluxo, bloco, condomínio, ferramenta, orientação, ajuste. Nunca diga
Assistente, Backdoor, Admin, caso de uso, subagente, operador, orquestrador, roteador, projeção,
orquestração, Nexus, MCP, YAML nem nomes de arquivo, exceto na tela Avançado.

Você não aplica nada: você propõe, e a pessoa decide. Proponha só o que ela pediu, usando
apenas as operações listadas abaixo e os nomes (de skill, condomínio, bloco) que aparecem no
contexto. Se não houver operação para o pedido, explique e não proponha nada. Num fluxo, um
bloco que grava no sistema precisa de um Confirmar logo antes.

Responda SÓ com JSON, sem texto fora dele:
{"mensagem": "o que você diz à pessoa", "operacoes": [ ... ]}`

// formaDaOperacao mostra ao modelo o JSON de cada operação (função: tabela fixa, sem estado global).
func formaDaOperacao(t copiloto.TipoDeOperacao) string {
	switch t {
	case copiloto.PreencherTextos:
		return `{"tipo":"preencher_textos","skill":"<nome>","textos":{"titulo":"...","resumo":"...","quandoUsar":"...","palavrasChave":["..."],"frasesDeExemplo":["..."]}} (mande só os textos que mudam)`
	case copiloto.TrocarFluxo:
		return `{"tipo":"trocar_fluxo","skill":"<nome>","fluxo":{...o fluxo inteiro, no mesmo formato do contexto...}}`
	case copiloto.CriarSkill:
		return `{"tipo":"criar_skill","titulo":"..."}`
	case copiloto.ExcluirSkill:
		return `{"tipo":"excluir_skill","skill":"<nome>"}`
	case copiloto.Orientar:
		return `{"tipo":"orientar","condominio":"<id>","skill":"<nome, ou vazio para a orientação geral>","texto":"..."}`
	case copiloto.Ajustar:
		return `{"tipo":"ajustar","condominio":"<id>","recurso":"<ferramenta>","campos":{"<campo>":"<valor>"}}`
	case copiloto.EditarPersona:
		return `{"tipo":"editar_persona","agente":"<caminho>","texto":"..."}`
	case copiloto.EditarArquivo:
		return `{"tipo":"editar_arquivo","agente":"<caminho>","caminho":"<arquivo>","conteudo":"<arquivo inteiro>"}`
	}
	return fmt.Sprintf(`{"tipo":%q}`, t)
}

func promptDoCopiloto(p PedidoAoCopiloto) string {
	var b strings.Builder
	b.WriteString(promptBaseDoCopiloto)
	b.WriteString("\n\nOperações permitidas agora:\n")
	if len(p.Operacoes) == 0 {
		b.WriteString("- nenhuma: aqui você só explica; mande \"operacoes\": [].\n")
	}
	for _, t := range p.Operacoes {
		if copiloto.PermissaoDe(t) == "" {
			continue
		}
		fmt.Fprintf(&b, "- %s\n", formaDaOperacao(t))
	}
	b.WriteString("\nO que está aberto na tela:\n")
	b.WriteString(p.Contexto)
	return b.String()
}
