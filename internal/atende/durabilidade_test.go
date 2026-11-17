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

// pedidosQueRenomeiam grava como o Fake, mas dá ao pedido novo outro protocolo, como faz a
// gravação no Postgres quando o sorteado já é de outro contato.
type pedidosQueRenomeiam struct{ *pedidos.Fake }

const protocoloRenomeado = "P-RENOMEAD"

func (st pedidosQueRenomeiam) Gravar(ctx context.Context, c escopo.Contato, cart pedidos.Carteira) error {
	gravada, err := st.Ler(ctx, c)
	if err != nil {
		return err
	}
	for _, p := range cart.Pedidos {
		if gravada.Por(p.Protocolo) == nil {
			p.Protocolo = protocoloRenomeado
		}
	}
	return st.Fake.Gravar(ctx, c, cart)
}

// O protocolo que a gravação trocou é o que sai do turno: no pedido ao atendente (o texto
// que vai ao ATENDE com os pedidos pendentes) e nos pedidos do turno. As respostas ao morador
// deste fluxo não citam o protocolo do pedido novo; o renomeador cobre o texto também.
func TestTurnoLevaOProtocoloRenomeado(t *testing.T) {
	svc, out, _ := servicoComPedidos(t)
	svc.Pedidos = pedidosQueRenomeiam{pedidos.NovoFake()}
	sess, err := svc.Processar(context.Background(), mensagemDeTexto(t, textoTurno+", quero falar com um atendente"))
	if err != nil {
		t.Fatal(err)
	}
	var transbordo string
	for _, s := range out.Itens("") {
		if s.Tipo == "transbordo" {
			transbordo = s.Texto
		}
	}
	if !strings.Contains(transbordo, protocoloRenomeado) {
		t.Fatalf("pedido ao atendente %q, want com o protocolo %s", transbordo, protocoloRenomeado)
	}
	turno := ultimoConcierge(sess)
	if len(turno.Pedidos) == 0 || turno.Pedidos[0].Protocolo != protocoloRenomeado {
		t.Fatalf("pedidos do turno %+v, want %s", turno.Pedidos, protocoloRenomeado)
	}
}

func TestRenomeador(t *testing.T) {
	tests := []struct {
		nome          string
		antes, depois []string
		texto, want   string
	}{
		{"nada mudou", []string{"P-AAAAAAAA"}, []string{"P-AAAAAAAA"}, "", ""},
		{"um renomeado", []string{"P-AAAAAAAA", "P-BBBBBBBB"}, []string{"P-AAAAAAAA", "P-CCCCCCCC"},
			"P-AAAAAAAA e P-BBBBBBBB", "P-AAAAAAAA e P-CCCCCCCC"},
		{"prefixo de um protocolo que não mudou", []string{"P-AB", "P-ABC"}, []string{"P-XY", "P-ABC"},
			"P-AB e P-ABC", "P-XY e P-ABC"},
		{"troca entre dois", []string{"P-A", "P-B"}, []string{"P-B", "P-A"}, "P-A, P-B", "P-B, P-A"},
	}
	for _, tt := range tests {
		t.Run(tt.nome, func(t *testing.T) {
			r := renomeador(tt.antes, tt.depois)
			if tt.want == "" {
				if r != nil {
					t.Fatal("renomeador sem mudança != nil")
				}
				return
			}
			if got := r.Replace(tt.texto); got != tt.want {
				t.Fatalf("Replace(%q) = %q, want %q", tt.texto, got, tt.want)
			}
		})
	}
}

// Um turno deixou a carteira adiada (mudança menor, gravada depois da resposta) e o
// atendente remove um pedido antes disso: o pedido removido não volta quando a carteira
// adiada é gravada.
func TestRemoverPedidoComCarteiraAdiada(t *testing.T) {
	svc, _, st := servicoComPedidos(t)
	sess := falar(t, svc, "evt-1", textoTurno)
	contato := sess.contato()
	cart, err := st.Ler(context.Background(), contato)
	if err != nil || len(cart.Pedidos) != 1 {
		t.Fatalf("carteira %+v (%v), want um pedido", cart.Pedidos, err)
	}
	prot := cart.Pedidos[0].Protocolo
	pedidos.Cancelar(cart.Pedidos[0], "Ana", "teste") // removível: encerrado
	if err := st.Gravar(context.Background(), contato, cart); err != nil {
		t.Fatal(err)
	}
	cart.Pedidos[0].Valores["observacao"] = "mudança menor do turno"
	svc.adiarCarteira(contato, cart)

	if err := svc.RemoverPedido(context.Background(), contato, prot); err != nil {
		t.Fatalf("RemoverPedido: %v", err)
	}
	if err := svc.gravarDepoisDaResposta(context.Background(), sess, nil); err != nil {
		t.Fatalf("gravarDepoisDaResposta: %v", err)
	}
	if final, _ := st.Ler(context.Background(), contato); final.Por(prot) != nil {
		t.Fatalf("pedido %s removido voltou: %+v", prot, final.Pedidos)
	}
}
