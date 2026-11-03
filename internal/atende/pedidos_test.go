package atende

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go-concierge/internal/entrada"
	"go-concierge/internal/gruvi"
	"go-concierge/internal/harness"
	"go-concierge/internal/okf"
	"go-concierge/internal/pedidos"
)

func servicoComPedidos(t *testing.T) (*Servico, *Outbox, *pedidos.Arquivos) {
	t.Helper()
	cat, err := okf.NewCatalogo(filepath.Join(raizDoRepo(t), "okf"))
	if err != nil {
		t.Fatal(err)
	}
	fix, _ := okf.LerRecursosFixture(filepath.Join(cat.Root, "nexus", "cond-001.yaml"))
	cat.Recursos = func() []okf.Recurso { return fix }
	adicionarToolsMCP(cat)
	out := &Outbox{}
	st := pedidos.NewArquivos(t.TempDir())
	mcpFalso := &mcpServicoFalso{}
	svc := &Servico{
		Cat: cat, Agente: "agentes/concierge", Account: "cond-001", Sessoes: NewSessoes(), Sender: out,
		Config:      harness.Config{Mock: true, Env: func(string) string { return "" }, MCP: mcpFalso},
		TelefoneB2C: "5519981621916", Gruvi: &gruvi.Client{CompanyID: "5982"},
		Pedidos: st,
	}
	return svc, out, st
}

func falar(t *testing.T, svc *Servico, id, texto string) *Sessao {
	t.Helper()
	ev, _ := Parse([]byte(strings.Replace(eventoExemplo, "posso fazer barulho depois das 22h?", texto, 1)))
	ev.ID = id
	msg := ev.Normalizar()
	msg.Origem = entrada.Origem{Conector: "atende-emulado", Meio: "whatsapp", ConexaoID: "console"}
	msg.Destino = entrada.Destino{Conector: "atende-emulado", Meio: "whatsapp", ConexaoID: "console", Protocolo: msg.ConversaID, Telefone: msg.Contato.Phone, Canal: msg.Canal}
	sess, err := svc.Processar(context.Background(), msg)
	if err != nil {
		t.Fatalf("%s: %v", texto, err)
	}
	return sess
}

func ultimoConcierge(sess *Sessao) Turno {
	for i := len(sess.Turnos) - 1; i >= 0; i-- {
		if sess.Turnos[i].Quem == "concierge" {
			return sess.Turnos[i]
		}
	}
	return Turno{}
}

// Pedido do início ao fim pelo registro: detectado por palavra-chave, unidade sozinha,
// área perguntada com opções, data validada, confirmação, escrita com o sim, protocolo
// no resultado; o estado vem dos parâmetros e fica gravado por telefone.
func TestPedidoDeReservaPeloRegistro(t *testing.T) {
	pedidos.Agora = func() time.Time { return time.Date(2026, 10, 28, 12, 0, 0, 0, time.Local) }
	defer func() { pedidos.Agora = time.Now }()
	svc, out, st := servicoComPedidos(t)
	ctx := context.Background()

	sess := falar(t, svc, "e1", "quero reservar uma área comum")
	r := ultimoConcierge(sess)
	if !strings.Contains(r.Texto, "Qual área você quer reservar?") || !strings.Contains(r.Texto, "1) Churrasqueira") {
		t.Fatalf("devia perguntar a área com opções: %q", r.Texto)
	}
	if len(r.Pedidos) == 0 || r.Pedidos[0].Evento != "aberto" || !strings.HasPrefix(r.Pedidos[0].Protocolo, "P-") {
		t.Fatalf("turno devia registrar o pedido aberto: %+v", r.Pedidos)
	}
	prot := r.Pedidos[0].Protocolo
	cart, _ := st.Ler(ctx, "cond-001", "5519981621916")
	p := cart.Por(prot)
	if p == nil || p.Estado != pedidos.Coletando || p.Valores["unidade"] != "109" || p.Pendencia == nil || p.Pendencia.Slot != "area" {
		t.Fatalf("pedido gravado com a unidade da identidade e a área pendente: %+v", p)
	}

	sess = falar(t, svc, "e2", "salão")
	if r = ultimoConcierge(sess); !strings.Contains(r.Texto, "Para qual data?") {
		t.Fatalf("depois da área, a data: %q", r.Texto)
	}
	sess = falar(t, svc, "e3", "30/02")
	if r = ultimoConcierge(sess); !strings.Contains(r.Texto, "Não consegui usar a resposta") || !strings.Contains(r.Texto, "Para qual data?") {
		t.Fatalf("data inválida pede de novo com a correção: %q", r.Texto)
	}
	sess = falar(t, svc, "e4", "15/11")
	if r = ultimoConcierge(sess); !strings.Contains(r.Texto, "Confirma a reserva de Salão de festas em 15 de novembro de 2026 para A - 101? (sim/não)") {
		t.Fatalf("confirmação renderizada do caso.yaml: %q", r.Texto)
	}
	cart, _ = st.Ler(ctx, "cond-001", "5519981621916")
	if p = cart.Por(prot); p.Estado != pedidos.Confirmando {
		t.Fatalf("estado devia ser aguardando_confirmacao: %s", p.Estado)
	}
	sess = falar(t, svc, "e5", "sim")
	r = ultimoConcierge(sess)
	if !strings.Contains(r.Texto, "Reserva confirmada: Salão de festas em 15 de novembro de 2026 (A - 101). Protocolo: MOCK-") {
		t.Fatalf("output do caso com o protocolo do recurso: %q", r.Texto)
	}
	cart, _ = st.Ler(ctx, "cond-001", "5519981621916")
	p = cart.Por(prot)
	if p.Estado != pedidos.Concluido || p.Resultado == nil || !strings.HasPrefix(p.Resultado.Protocolo, "MOCK-") || p.Confirmacao.Por != pedidos.OrigemMorador {
		t.Fatalf("concluído com protocolo do recurso: %+v %+v", p.Estado, p.Resultado)
	}
	if n := len(out.Itens("344200011995")); n != 5 {
		t.Fatalf("cinco respostas enviadas: %d", n)
	}
	// o que não é pedido segue pelo orquestrador (router), sem abrir pedido
	sess = falar(t, svc, "e6", "posso fazer barulho depois das 22h?")
	if r = ultimoConcierge(sess); r.Agente != "agentes/concierge" || len(r.Pedidos) != 0 || r.Texto == "" {
		t.Fatalf("dúvida não vira pedido: %+v", r)
	}
	temRota := false
	for _, p := range r.Rota {
		if p.Tipo == "rota" {
			temRota = true
		}
	}
	if !temRota {
		t.Fatalf("o restante devia passar pelo roteador: %+v", r.Rota)
	}
}

// Pendência com o atendente: o morador pede humano no meio da coleta; o pedido vai ao
// atendente com o resumo; o atendente preenche o que faltava e confirma; o motor executa
// e o resultado vai ao morador pelo mesmo canal.
func TestPedidoPendenteVaiAoAtendente(t *testing.T) {
	pedidos.Agora = func() time.Time { return time.Date(2026, 10, 28, 12, 0, 0, 0, time.Local) }
	defer func() { pedidos.Agora = time.Now }()
	svc, out, st := servicoComPedidos(t)
	ctx := context.Background()

	sess := falar(t, svc, "a1", "quero reservar o salão")
	prot := ultimoConcierge(sess).Pedidos[0].Protocolo
	sess = falar(t, svc, "a2", "quero falar com um atendente")
	if sess.Transbordo == nil || !sess.Transbordo.Ativo || !strings.Contains(sess.Transbordo.Motivo, prot) || !strings.Contains(sess.Transbordo.Motivo, "falta") {
		t.Fatalf("transbordo com a pendência do pedido: %+v", sess.Transbordo)
	}
	cart, _ := st.Ler(ctx, "cond-001", "5519981621916")
	p := cart.Por(prot)
	if p.Estado != pedidos.ComAtendente || !strings.Contains(p.Resumo, "Falta:") || !strings.Contains(p.Resumo, "Pergunta em aberto") {
		t.Fatalf("pedido com o atendente e resumo: %s %q", p.Estado, p.Resumo)
	}
	// o atendente informa a área e a data, sem confirmar: o motor pede a confirmação a ele
	_, p, saida, err := svc.PreencherPedido(ctx, sess.Protocolo, prot, "Ana", map[string]any{"area": "salao", "data": "2026-12-20"}, false)
	if err != nil || saida.Tipo != "confirmar" || p.Estado != pedidos.ComAtendente {
		t.Fatalf("faltava confirmar: %v %+v %s", err, saida, p.Estado)
	}
	antes := len(out.Itens("344200011995"))
	_, p, saida, err = svc.PreencherPedido(ctx, sess.Protocolo, prot, "Ana", nil, true)
	if err != nil || saida.Tipo != "concluido" || p.Estado != pedidos.Concluido || p.Confirmacao.Por != "Ana" {
		t.Fatalf("confirmado pelo atendente executa: %v %+v %s", err, saida, p.Estado)
	}
	itens := out.Itens("344200011995")
	if len(itens) != antes+1 || !strings.Contains(itens[len(itens)-1].Texto, "Reserva confirmada: Salão de festas em 20 de dezembro de 2026") {
		t.Fatalf("resultado devia ir ao morador: %+v", itens[len(itens)-1])
	}
	if ult := sess.Turnos[len(sess.Turnos)-1]; ult.Quem != "concierge" || len(ult.Pedidos) != 1 || ult.Pedidos[0].Evento != "concluido" {
		t.Fatalf("turno do resultado: %+v", ult)
	}
	// rejeição do atendente também é validada
	_, _, _, err = svc.PreencherPedido(ctx, sess.Protocolo, "P-NAO", "Ana", nil, false)
	if err == nil {
		t.Fatal("pedido inexistente devia falhar")
	}
	cart, _ = st.Ler(ctx, "cond-001", "5519981621916")
	p2 := cart.Novo(mustContrato(t, svc), "reserva-area-comum", pedidos.Origem{})
	_ = st.Gravar(ctx, cart)
	if _, _, _, err := svc.PreencherPedido(ctx, sess.Protocolo, p2.Protocolo, "Ana", map[string]any{"data": "ontem"}, false); err == nil || !strings.Contains(err.Error(), "data") {
		t.Fatalf("valor inválido do atendente é rejeitado: %v", err)
	}
	if p, err := svc.CancelarPedido(ctx, "5519981621916", p2.Protocolo, "Ana", "teste"); err != nil || p.Estado != pedidos.Cancelado {
		t.Fatalf("cancelar: %v %+v", err, p)
	}
}

func mustContrato(t *testing.T, svc *Servico) *pedidos.Contrato {
	t.Helper()
	c, err := okf.LoadCaso(filepath.Join(svc.Cat.Root, "agentes", "concierge", "agentes", "reserva-area-comum", "caso.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	ct, err := pedidos.ContratoDe(c)
	if err != nil {
		t.Fatal(err)
	}
	return ct
}

// Pendências não dormem: falar de outra coisa traz de volta a pergunta em aberto; na
// terceira vez a resposta oferece cancelar ou atendente; na quinta o pedido vai ao
// atendente com transbordo, em vez de ficar aberto para sempre.
func TestPendenciasNaoDormem(t *testing.T) {
	svc, _, st := servicoComPedidos(t)
	ctx := context.Background()
	sess := falar(t, svc, "n1", "quero reservar uma área comum")
	prot := ultimoConcierge(sess).Pedidos[0].Protocolo
	sess = falar(t, svc, "n2", "posso fazer barulho depois das 22h?")
	r := ultimoConcierge(sess)
	if !strings.Contains(r.Texto, "Ainda sobre reserva de área comum ("+prot+")") || !strings.Contains(r.Texto, "Qual área você quer reservar?") || strings.Contains(r.Texto, "cancelar "+prot) {
		t.Fatalf("a pergunta em aberto devia voltar junto com a resposta: %q", r.Texto)
	}
	if r.Agente != "agentes/concierge" || len(r.Pedidos) != 1 || r.Pedidos[0].Evento != "lembrete" {
		t.Fatalf("turno devia anotar o lembrete: %+v", r.Pedidos)
	}
	sess = falar(t, svc, "n3", "e até que horas a piscina abre?")
	if r = ultimoConcierge(sess); !strings.Contains(r.Texto, "cancelar "+prot) || !strings.Contains(r.Texto, "atendente") {
		t.Fatalf("na terceira vez oferece cancelar ou atendente: %q", r.Texto)
	}
	falar(t, svc, "n4", "obrigado pela informação")
	sess = falar(t, svc, "n5", "vou pensar sobre isso")
	if sess.Transbordo == nil || !sess.Transbordo.Ativo || !strings.Contains(sess.Transbordo.Motivo, prot) {
		t.Fatalf("na quinta vez vai ao atendente: %+v", sess.Transbordo)
	}
	cart, _ := st.Ler(ctx, "cond-001", "5519981621916")
	if p := cart.Por(prot); p == nil || p.Estado != pedidos.ComAtendente || p.Pendencia == nil || p.Pendencia.Vezes != 5 {
		t.Fatalf("pedido com o atendente depois de 5 perguntas: %+v", cart.Por(prot))
	}
}

// "cancelar P-XXXXX" vale sempre, em código, por cima do extrator.
func TestCancelarPorProtocolo(t *testing.T) {
	svc, _, st := servicoComPedidos(t)
	sess := falar(t, svc, "c1", "quero reservar uma área comum")
	prot := ultimoConcierge(sess).Pedidos[0].Protocolo
	sess = falar(t, svc, "c2", "cancelar "+strings.ToLower(prot))
	r := ultimoConcierge(sess)
	if !strings.Contains(r.Texto, "cancelado") || len(r.Pedidos) != 1 || r.Pedidos[0].Evento != "cancelado" {
		t.Fatalf("cancelamento pelo protocolo: %q %+v", r.Texto, r.Pedidos)
	}
	cart, _ := st.Ler(context.Background(), "cond-001", "5519981621916")
	if p := cart.Por(prot); p == nil || p.Estado != pedidos.Cancelado {
		t.Fatalf("estado: %+v", cart.Por(prot))
	}
}

// Remover só tira pedidos encerrados; um aberto precisa ser cancelado antes.
func TestRemoverPedidoEncerrado(t *testing.T) {
	svc, _, st := servicoComPedidos(t)
	ctx := context.Background()
	sess := falar(t, svc, "r1", "quero reservar uma área comum")
	prot := ultimoConcierge(sess).Pedidos[0].Protocolo
	if err := svc.RemoverPedido(ctx, "5519981621916", prot); err == nil {
		t.Fatalf("pedido aberto não pode ser removido")
	}
	if _, err := svc.CancelarPedido(ctx, "5519981621916", prot, "Atendente", "teste"); err != nil {
		t.Fatal(err)
	}
	if err := svc.RemoverPedido(ctx, "5519981621916", strings.ToLower(prot)); err != nil {
		t.Fatal(err)
	}
	cart, _ := st.Ler(ctx, "cond-001", "5519981621916")
	if cart.Por(prot) != nil || len(cart.Pedidos) != 0 {
		t.Fatalf("pedido devia ter saído da carteira: %+v", cart.Pedidos)
	}
	if err := svc.RemoverPedido(ctx, "5519981621916", prot); err == nil {
		t.Fatalf("remover de novo devia falhar: não encontrado")
	}
}

// extratorFila: no primeiro turno abre dois pedidos de uma vez; depois, Regras.
type extratorFila struct{ vez int }

func (e *extratorFila) Extrair(ctx context.Context, in pedidos.Entrada) (pedidos.Extracao, error) {
	e.vez++
	if e.vez == 1 {
		return pedidos.Extracao{Pedidos: []pedidos.PedidoExtraido{
			{Caso: "reserva_area_comum", Valores: map[string]any{}, Trecho: "reservar a churrasqueira"},
			{Caso: "agendar_mudanca", Valores: map[string]any{}, Trecho: "agendar minha mudança"},
		}}, nil
	}
	return pedidos.Regras{}.Extrair(ctx, in)
}

// Fila: dois pedidos na mesma mensagem → só o primeiro pergunta; o segundo espera (sem
// lembrete) e é puxado quando o primeiro termina.
func TestPedidosUmPorVez(t *testing.T) {
	pedidos.Agora = func() time.Time { return time.Date(2026, 11, 3, 12, 0, 0, 0, time.Local) }
	defer func() { pedidos.Agora = time.Now }()
	svc, _, st := servicoComPedidos(t)
	svc.ExtratorPedidos = &extratorFila{}
	ctx := context.Background()

	sess := falar(t, svc, "f1", "quero reservar a churrasqueira e agendar minha mudança")
	r := ultimoConcierge(sess)
	if !strings.Contains(r.Texto, "Qual área você quer reservar?") || strings.Contains(r.Texto, "data você gostaria de agendar a mudança") {
		t.Fatalf("só o primeiro pedido pergunta: %q", r.Texto)
	}
	if !strings.Contains(r.Texto, "um pedido por vez") || !strings.Contains(r.Texto, "agendamento de mudança") {
		t.Fatalf("o morador sabe a ordem: %q", r.Texto)
	}
	cart, _ := st.Ler(ctx, "cond-001", "5519981621916")
	if len(cart.Abertos()) != 2 {
		t.Fatalf("dois pedidos abertos: %+v", cart.Abertos())
	}
	// turno sobre outra coisa: só o ativo lembra; o da fila fica quieto
	sess = falar(t, svc, "f2", "posso fazer barulho depois das 22h?")
	r = ultimoConcierge(sess)
	if !strings.Contains(r.Texto, "Qual área você quer reservar?") || strings.Contains(r.Texto, "mudança") {
		t.Fatalf("lembrete só do ativo: %q", r.Texto)
	}
	// cancela o ativo: o próximo é puxado no mesmo turno
	prot := cart.Abertos()[0].Protocolo
	sess = falar(t, svc, "f3", "cancelar "+prot)
	r = ultimoConcierge(sess)
	if !strings.Contains(r.Texto, "cancelado") || !strings.Contains(r.Texto, "data você gostaria de agendar a mudança") {
		t.Fatalf("o próximo da fila é puxado quando o ativo termina: %q", r.Texto)
	}
}
