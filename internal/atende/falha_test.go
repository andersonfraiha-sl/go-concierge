package atende

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"go-concierge/internal/entrada"
	"go-concierge/internal/pedidos"
)

// mensagemExemplo: o evento de exemplo já normalizado para o canal emulado.
func mensagemExemplo(t *testing.T) entrada.Mensagem {
	t.Helper()
	ev, err := Parse([]byte(eventoExemplo))
	if err != nil {
		t.Fatal(err)
	}
	msg := ev.Normalizar()
	msg.Origem = entrada.Origem{Conector: "atende-emulado", Meio: "whatsapp", ConexaoID: "console"}
	msg.Destino = entrada.Destino{Conector: "atende-emulado", Meio: "whatsapp", ConexaoID: "console", Protocolo: msg.ConversaID, Telefone: msg.Contato.Phone, Canal: msg.Canal}
	return msg
}

// modeloFalho: um modelo que só devolve erro (o provedor fora do ar).
type modeloFalho struct{ err error }

func (m modeloFalho) Generate(context.Context, []*schema.Message, ...model.Option) (*schema.Message, error) {
	return nil, m.err
}
func (m modeloFalho) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, m.err
}
func (m modeloFalho) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) { return m, nil }

type extratorFalho struct{ err error }

func (e extratorFalho) Extrair(context.Context, pedidos.Entrada) (pedidos.Extracao, error) {
	return pedidos.Extracao{}, e.err
}

var err503 = errors.New("failed to create chat completion: error, status code: 503, status: 503 Service Unavailable, message: This model is currently experiencing high demand. Please try again later.")

func TestFalhaTemporaria(t *testing.T) {
	for _, c := range []struct {
		err error
		sim bool
	}{
		{err503, true},
		{errors.New("status code: 429, rate limit exceeded"), true},
		{context.DeadlineExceeded, true},
		{errors.New("roteador: json: cannot unmarshal string"), false},
		{errors.New("caso.yaml inválido: passo sem resource"), false},
		{nil, false},
	} {
		if got := FalhaTemporaria(c.err); got != c.sim {
			t.Errorf("FalhaTemporaria(%v) = %v", c.err, got)
		}
	}
}

// O modelo fora do ar: o morador recebe uma frase amigável (não o erro), o erro técnico
// fica no turno para o Console e no retorno para o log, e a troca entra no histórico.
func TestFalhaDoModeloViraMensagemAmigavel(t *testing.T) {
	svc, out, _ := servicoComPedidos(t)
	svc.Config.Mock = false
	svc.Config.ChatModel = modeloFalho{err503}
	sess, err := svc.Processar(context.Background(), mensagemExemplo(t))
	if err == nil || !strings.Contains(err.Error(), "503") {
		t.Fatalf("o erro técnico continua indo para o log: %v", err)
	}
	r := ultimoConcierge(sess)
	if r.Texto != FraseInstabilidade || !strings.Contains(r.Erro, "503") {
		t.Fatalf("resposta ao morador: %q (erro: %q)", r.Texto, r.Erro)
	}
	for _, tr := range sess.Turnos {
		if tr.Quem == "erro" {
			t.Fatalf("o erro não vira bolha na conversa: %+v", tr)
		}
	}
	if itens := out.Itens(sess.Protocolo); len(itens) != 1 || itens[0].Texto != FraseInstabilidade {
		t.Fatalf("o morador recebe só a frase: %+v", itens)
	}
	if len(sess.historico) != 2 {
		t.Fatalf("a troca entra no histórico: %d", len(sess.historico))
	}
}

// Erro que não é de instabilidade: frase genérica de falha, mesma mecânica.
func TestFalhaGenericaViraMensagemAmigavel(t *testing.T) {
	svc, _, _ := servicoComPedidos(t)
	svc.Config.Mock = false
	svc.Config.ChatModel = modeloFalho{errors.New("json: cannot unmarshal array into Go value")}
	sess, _ := svc.Processar(context.Background(), mensagemExemplo(t))
	if r := ultimoConcierge(sess); r.Texto != FraseFalha || r.Erro == "" {
		t.Fatalf("resposta: %q (erro: %q)", r.Texto, r.Erro)
	}
}

// O extrator de pedidos fora do ar e as regras sem nada: o morador ouve que há
// instabilidade, não "não entendi".
func TestExtratorForaDoArAvisaInstabilidade(t *testing.T) {
	svc, _, _ := servicoComPedidos(t)
	svc.ExtratorPedidos = extratorFalho{err503}
	sess, err := svc.Processar(context.Background(), mensagemExemplo(t))
	if err == nil {
		t.Fatal("o erro vai ao log")
	}
	if r := ultimoConcierge(sess); r.Texto != FraseInstabilidade {
		t.Fatalf("resposta: %q", r.Texto)
	}
}
