package atende

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"

	"go-concierge/internal/entrada"
	"go-concierge/internal/memoria"
	"go-concierge/internal/pedidos"
)

func TestMensagemAnteriorAoAtendimentoHumanoNaoReativaBot(t *testing.T) {
	svc := &Servico{Sessoes: NewSessoes(), Sender: &Outbox{}}
	msg := entrada.Mensagem{ID: "evt-1", ConversaID: "protocolo", Contato: entrada.Contato{Phone: "5511999999999"}, Texto: "mensagem antiga"}
	antiga, sess := svc.PrepararMensagem(msg)
	humana := msg
	humana.ID, humana.Autor, humana.Texto = "evt-2", "atendente", "Vou atender"
	if !svc.RegistrarHumano(humana) {
		t.Fatal("evento humano não reconheceu a sessão")
	}
	if _, err := svc.Processar(context.Background(), antiga); err != nil {
		t.Fatal(err)
	}
	if !sess.AtendimentoHumano || len(sess.Turnos) != 2 || len(svc.Sender.(*Outbox).Itens(sess.ID)) != 0 {
		t.Fatalf("mensagem anterior reativou o bot: %+v", sess.Turnos)
	}
	nova, _ := svc.PrepararMensagem(msg)
	if sess.AtendimentoHumano || nova.Revisao == antiga.Revisao {
		t.Fatal("mensagem posterior deve permitir a retomada do bot")
	}
}

func TestAutorDaMensagemNoAtendimentoHumano(t *testing.T) {
	svc := &Servico{Sessoes: NewSessoes()}
	msg := entrada.Mensagem{ConversaID: "protocolo", Contato: entrada.Contato{Phone: "5511999999999"}}
	sess := svc.Sessoes.Abrir(msg)
	msg.Texto, msg.Autor = "sou morador", "morador"
	if !svc.RegistrarHumano(msg) {
		t.Fatal("sessão não encontrada")
	}
	h := sess.historicoSnapshot()
	if sess.Turnos[0].Quem != "morador" || len(h) != 1 || h[0].Role != schema.User {
		t.Fatalf("autoria alterada: turno=%+v histórico=%+v", sess.Turnos[0], h)
	}
}

func TestResponderVazioNaoBloqueiaSessao(t *testing.T) {
	svc := &Servico{Sessoes: NewSessoes()}
	msg := entrada.Mensagem{ConversaID: "protocolo", Origem: entrada.Origem{Conector: "atende-emulado", ConexaoID: "console"}}
	sess := svc.Sessoes.Abrir(msg)
	if _, err := svc.Assumir(sess.ID, "atendente", "teste"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Responder(context.Background(), sess.ID, "atendente", " "); err == nil {
		t.Fatal("resposta vazia deveria falhar")
	}
	terminou := make(chan struct{})
	go func() { svc.Sessoes.Lista(); close(terminou) }()
	select {
	case <-terminou:
	case <-time.After(time.Second):
		t.Fatal("a sessão permaneceu bloqueada")
	}
}

func TestEntregaInterrompidaEntrePartesAposTransicaoHumana(t *testing.T) {
	var mu sync.Mutex
	permitido := true
	chamadas := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		chamadas++
		permitido = false
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	ctx := context.WithValue(context.Background(), chaveControleEntrega{}, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return permitido
	})
	err := (&ATENDESender{BaseURL: server.URL, APIKey: "teste"}).Enviar(ctx, entrada.Destino{Telefone: "5511999999999"}, strings.Repeat("x", 5000))
	if !errors.Is(err, errEntregaSuprimida) || chamadas != 1 {
		t.Fatalf("esperava apenas primeira parte e interrupção: chamadas=%d erro=%v", chamadas, err)
	}
}

type extratorBloqueado struct {
	iniciado chan struct{}
	liberar  chan struct{}
}

func (e extratorBloqueado) Extrair(_ context.Context, _, _ string, _ memoria.Memoria) ([]memoria.Fato, error) {
	close(e.iniciado)
	<-e.liberar
	return []memoria.Fato{{Chave: "nome", Texto: "texto antigo"}}, nil
}

func TestEsquecimentoNaoRessuscitaExtracaoEmAndamento(t *testing.T) {
	ctx := context.Background()
	store := memoria.NewArquivos(t.TempDir())
	contato := contatoDeTeste(t, "adm-a")
	m := memoria.Memoria{Fatos: []memoria.Fato{{Chave: "nome", Texto: "texto antigo"}}}
	if err := store.Gravar(ctx, contato, m); err != nil {
		t.Fatal(err)
	}
	ex := extratorBloqueado{iniciado: make(chan struct{}), liberar: make(chan struct{})}
	svc := &Servico{Memorias: store, Extrator: ex}
	sess := &Sessao{Protocolo: "p", Contato: entrada.Contato{Phone: contato.Telefone}}
	sess.fixarContato(contato, "5982")
	revisao := svc.revisaoMemoria(contato)
	fim := make(chan struct{})
	go func() { svc.lembrar(ctx, &m, sess, "oi", "oi", revisao); close(fim) }()
	<-ex.iniciado
	if _, err := svc.EsquecerMemoria(ctx, contato, "nome"); err != nil {
		t.Fatal(err)
	}
	close(ex.liberar)
	<-fim
	atual, err := store.Ler(ctx, contato)
	if err != nil || len(atual.Fatos) != 0 {
		t.Fatalf("fato esquecido reapareceu: %+v erro=%v", atual.Fatos, err)
	}
}

type senderFunc func(context.Context, entrada.Destino, string) error

func (f senderFunc) Enviar(ctx context.Context, destino entrada.Destino, texto string) error {
	return f(ctx, destino, texto)
}

func TestProcessarRegistraParteEntregueAposMudancaHumana(t *testing.T) {
	svc, _, _ := servicoComPedidos(t)
	ev, err := Parse([]byte(eventoExemplo))
	if err != nil {
		t.Fatal(err)
	}
	msg, sess := svc.PrepararMensagem(ev.Normalizar())
	svc.Sender = senderFunc(func(_ context.Context, _ entrada.Destino, _ string) error {
		humana := msg
		humana.Autor, humana.Texto = "atendente", "Assumi o atendimento"
		svc.RegistrarHumano(humana)
		return &entregaParcial{partes: []string{"parte entregue"}, erro: errEntregaSuprimida}
	})
	if _, err := svc.Processar(context.Background(), msg); err != nil {
		t.Fatal(err)
	}
	historico := sess.historicoSnapshot()
	var entregas, humanas int
	for _, item := range historico {
		if item.Role == schema.Assistant && item.Content == "parte entregue" {
			entregas++
		}
		if item.Role == schema.Assistant && item.Content == "[atendente humano] Assumi o atendimento" {
			humanas++
		}
	}
	if entregas != 1 || humanas != 1 {
		t.Fatalf("histórico deve preservar parte entregue e fala humana uma vez: %+v", historico)
	}
}

func TestProcessarNaoDescartaEntregaConcluidaAntesDaMudancaHumana(t *testing.T) {
	svc, _, _ := servicoComPedidos(t)
	ev, err := Parse([]byte(eventoExemplo))
	if err != nil {
		t.Fatal(err)
	}
	msg, sess := svc.PrepararMensagem(ev.Normalizar())
	svc.Sender = senderFunc(func(_ context.Context, _ entrada.Destino, _ string) error {
		humana := msg
		humana.Autor, humana.Texto = "atendente", "Assumi o atendimento"
		svc.RegistrarHumano(humana)
		return nil
	})
	if _, err := svc.Processar(context.Background(), msg); err != nil {
		t.Fatal(err)
	}
	for _, turno := range sess.Turnos {
		if turno.Quem == "concierge" {
			return
		}
	}
	t.Fatal("resposta já entregue foi descartada do histórico")
}

func TestPreencherPedidoRegistraSomenteParteEntregue(t *testing.T) {
	pedidos.Agora = func() time.Time { return time.Date(2026, 10, 28, 12, 0, 0, 0, time.Local) }
	defer func() { pedidos.Agora = time.Now }()
	svc, _, _ := servicoComPedidos(t)
	sess := falar(t, svc, "parcial-1", "quero reservar o salão")
	protocolo := ultimoConcierge(sess).Pedidos[0].Protocolo
	sess = falar(t, svc, "parcial-2", "quero falar com um atendente")
	if _, _, _, err := svc.PreencherPedido(context.Background(), sess.ID, protocolo, "Ana", map[string]any{"area": "salao", "data": "2026-12-20"}, false); err != nil {
		t.Fatal(err)
	}
	svc.Sender = senderFunc(func(context.Context, entrada.Destino, string) error {
		return &entregaParcial{partes: []string{"parte confirmada"}, erro: errors.New("falha de envio")}
	})
	if _, _, _, err := svc.PreencherPedido(context.Background(), sess.ID, protocolo, "Ana", nil, true); err == nil {
		t.Fatal("falha parcial deve ser propagada")
	}
	historico := sess.historicoSnapshot()
	if len(historico) == 0 || historico[len(historico)-1].Content != "parte confirmada" {
		t.Fatalf("histórico deve conter apenas a parte entregue: %+v", historico)
	}
}

func TestProcessarNaoRegistraRespostaSemConfirmacaoDeEntrega(t *testing.T) {
	svc, _, _ := servicoComPedidos(t)
	ev, err := Parse([]byte(eventoExemplo))
	if err != nil {
		t.Fatal(err)
	}
	msg, sess := svc.PrepararMensagem(ev.Normalizar())
	svc.Sender = senderFunc(func(context.Context, entrada.Destino, string) error {
		return errors.New("falha de envio")
	})
	if _, err := svc.Processar(context.Background(), msg); err == nil {
		t.Fatal("falha de envio deveria ser propagada")
	}
	for _, turno := range sess.Turnos {
		if turno.Quem == "concierge" {
			t.Fatal("resposta não confirmada foi registrada como enviada")
		}
	}
	for _, item := range sess.historicoSnapshot() {
		if item.Role == schema.Assistant {
			t.Fatal("histórico contém resposta sem confirmação de entrega")
		}
	}
}
