package atende

import (
	"context"
	"errors"
	"strings"
	"testing"

	"go-concierge/internal/entrada"
	"go-concierge/internal/escopo"
	"go-concierge/internal/pedidos"
)

// pedidosQueNaoGravam lê como o Fake e falha toda gravação (banco fora).
type pedidosQueNaoGravam struct{ *pedidos.Fake }

var errBancoFora = errors.New("banco fora")

func (pedidosQueNaoGravam) Gravar(context.Context, escopo.Contato, pedidos.Carteira) error {
	return errBancoFora
}

func mensagemDeTexto(t *testing.T, texto string) entrada.Mensagem {
	t.Helper()
	ev, err := Parse([]byte(strings.Replace(eventoExemplo, "posso fazer barulho depois das 22h?", texto, 1)))
	if err != nil {
		t.Fatal(err)
	}
	msg := ev.Normalizar()
	msg.Origem = entrada.Origem{Conector: "atende-emulado", Meio: "whatsapp", ConexaoID: "console"}
	msg.Destino = entrada.Destino{Conector: "atende-emulado", Meio: "whatsapp", ConexaoID: "console", Protocolo: msg.ConversaID, Telefone: msg.Contato.Phone, Canal: msg.Canal}
	return msg
}

// O pedido aberto no turno já está no banco quando a resposta sai.
func TestPedidoAbertoEstaGravadoQuandoARespostaSai(t *testing.T) {
	svc, _, st := servicoComPedidos(t)
	contato := contatoDeTeste(t, "adm-a")
	var abertosNoEnvio int
	svc.Sender = senderFunc(func(context.Context, entrada.Destino, string) error {
		cart, err := st.Ler(context.Background(), contato)
		if err != nil {
			return err
		}
		abertosNoEnvio = len(cart.Abertos())
		return nil
	})
	if _, err := svc.Processar(context.Background(), mensagemDeTexto(t, textoTurno)); err != nil {
		t.Fatal(err)
	}
	if abertosNoEnvio != 1 {
		t.Fatalf("pedidos abertos no banco no Enviar = %d, want 1", abertosNoEnvio)
	}
}

// Se o pedido aberto não grava, a resposta que o cita não sai: o turno falha fechado e o
// morador recebe só a frase de falha, como antes.
func TestPedidoNaoGravadoNaoSaiNaResposta(t *testing.T) {
	svc, out, _ := servicoComPedidos(t)
	svc.Pedidos = pedidosQueNaoGravam{pedidos.NovoFake()}
	_, err := svc.Processar(context.Background(), mensagemDeTexto(t, textoTurno))
	if !errors.Is(err, errBancoFora) {
		t.Fatalf("Processar = %v, want o erro da gravação", err)
	}
	enviadas := out.Itens("")
	if len(enviadas) != 1 || (enviadas[0].Texto != FraseFalha && enviadas[0].Texto != FraseInstabilidade) {
		t.Fatalf("enviado %+v, want só a frase de falha", enviadas)
	}
}
