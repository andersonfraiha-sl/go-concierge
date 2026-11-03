package atende

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"go-concierge/internal/entrada"
	"go-concierge/internal/escopo"
	"go-concierge/internal/mcp"
	"go-concierge/internal/pedidos"
)

func TestMudancaDeAtendimentoAntesDaEscritaInterrompeVisita(t *testing.T) {
	for _, caso := range []string{"humano", "fechamento"} {
		t.Run(caso, func(t *testing.T) {
			fixarAgoraDoFluxo(t)
			svc, fake := servicoComMCPFluxo(t)
			svc.ExtratorPedidos = extratorVisitaMCP()
			sess := falar(t, svc, "interrupcao-1", "quero liberar Ana Souza hoje")
			if !strings.Contains(ultimoConcierge(sess).Texto, "Confirma a liberação") {
				t.Fatal("a visita deveria aguardar confirmação")
			}
			entrou, liberar := make(chan struct{}), make(chan struct{})
			svc.ExtratorPedidos = extratorPedidosFunc(func(ctx context.Context, in pedidos.Entrada) (pedidos.Extracao, error) {
				close(entrou)
				<-liberar
				return (pedidos.Regras{}).Extrair(ctx, in)
			})
			ev, _ := Parse([]byte(eventoExemplo))
			msg := ev.Normalizar()
			msg.ID, msg.Texto = "interrupcao-2", "sim"
			msg.Origem, msg.Destino = sess.Origem, sess.Destino
			msg, _ = svc.PrepararMensagem(msg)
			fim := make(chan error, 1)
			go func() { _, err := svc.Processar(context.Background(), msg); fim <- err }()
			select {
			case <-entrou:
			case <-time.After(5 * time.Second):
				t.Fatal("extrator não iniciou")
			}
			if caso == "humano" {
				humana := msg
				humana.Autor, humana.Texto = "atendente", "atendimento assumido"
				if !svc.RegistrarHumano(humana) {
					t.Fatal("sessão não encontrada")
				}
			} else if !svc.Encerrar(msg) {
				t.Fatal("sessão não encontrada")
			}
			close(liberar)
			select {
			case <-fim:
			case <-time.After(5 * time.Second):
				t.Fatal("processamento não encerrou")
			}
			if fake.tentativasVisita != 0 {
				t.Fatalf("a escrita começou depois da mudança de atendimento: %d", fake.tentativasVisita)
			}
		})
	}
}

func TestMotivoInformadoNaoIgnoraListaMCPVazia(t *testing.T) {
	fixarAgoraDoFluxo(t)
	svc, fake := servicoComMCPFluxo(t)
	fake.motivosVisita = []any{}
	svc.ExtratorPedidos = extratorPedidosFunc(func(_ context.Context, in pedidos.Entrada) (pedidos.Extracao, error) {
		return pedidos.Extracao{Pedidos: []pedidos.PedidoExtraido{{Caso: "liberacao_visitante", Valores: map[string]any{
			"visitante_nome": "Ana Souza", "visitante_telefone": "5511999999999", "data_visita": "2026-10-28", "categoria_motivo": "GUEST", "motivo": 5,
		}}}}, nil
	})
	sess := falar(t, svc, "motivo-inexistente", "quero liberar Ana Souza hoje")
	if texto := ultimoConcierge(sess).Texto; !strings.Contains(texto, "Não há motivos disponíveis") || fake.tentativasVisita != 0 {
		t.Fatalf("lista vazia deve impedir confirmação e escrita: resposta=%q chamadas=%d", texto, fake.tentativasVisita)
	}
}

type storePedidosFalho struct {
	pedidos.Store
	falhar bool
}

func (s *storePedidosFalho) Gravar(ctx context.Context, contato escopo.Contato, c pedidos.Carteira) error {
	if s.falhar {
		return errors.New("gravação indisponível")
	}
	return s.Store.Gravar(ctx, contato, c)
}

func TestFalhaAoSalvarVisitaNaoPermiteRepetirEscrita(t *testing.T) {
	fixarAgoraDoFluxo(t)
	svc, fake := servicoComMCPFluxo(t)
	store := &storePedidosFalho{Store: svc.Pedidos}
	svc.Pedidos = store
	svc.ExtratorPedidos = extratorVisitaMCP()
	sess := falar(t, svc, "persistencia-1", "quero liberar Ana Souza hoje")
	if !strings.Contains(ultimoConcierge(sess).Texto, "Confirma a liberação") {
		t.Fatal("visita não aguardou confirmação")
	}
	store.falhar = true
	ev, _ := Parse([]byte(eventoExemplo))
	msg := ev.Normalizar()
	msg.ID, msg.Texto = "persistencia-2", "sim"
	msg.Origem, msg.Destino = sess.Origem, sess.Destino
	if _, err := svc.Processar(context.Background(), msg); err == nil {
		t.Error("falha de persistência deve ser propagada")
	}
	if fake.tentativasVisita != 1 {
		t.Fatalf("esperava uma tentativa: %d", fake.tentativasVisita)
	}
	msg.ID = "persistencia-3"
	if _, err := svc.Processar(context.Background(), msg); err == nil {
		t.Error("persistência indisponível deve impedir nova tentativa")
	}
	if fake.tentativasVisita != 1 {
		t.Fatalf("visita repetida após falha: %d tentativas", fake.tentativasVisita)
	}
	store.falhar = false
	msg.ID = "persistencia-4"
	if _, err := svc.Processar(context.Background(), msg); err != nil {
		t.Fatalf("não recuperou o estado pendente: %v", err)
	}
	if fake.tentativasVisita != 1 {
		t.Fatalf("visita repetida após recuperação: %d tentativas", fake.tentativasVisita)
	}
}

type mcpVisitaBloqueado struct {
	*mcpFluxoFalso
	iniciou chan struct{}
}

func (m *mcpVisitaBloqueado) CallTool(ctx context.Context, nome string, args map[string]any) (*mcp.CallResult, error) {
	if nome == "gruvi.bff.building-operation.create-visit-controller-handle" {
		m.tentativasVisita++
		close(m.iniciou)
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return m.mcpFluxoFalso.CallTool(ctx, nome, args)
}

func TestMudancaDuranteEscritaPreservaEstadoIncerto(t *testing.T) {
	fixarAgoraDoFluxo(t)
	svc, fake := servicoComMCPFluxo(t)
	svc.ExtratorPedidos = extratorVisitaMCP()
	sess := falar(t, svc, "escrita-1", "quero liberar Ana Souza hoje")
	prot := ultimoConcierge(sess).Pedidos[0].Protocolo
	bloqueado := &mcpVisitaBloqueado{mcpFluxoFalso: fake, iniciou: make(chan struct{})}
	svc.Config.MCP = bloqueado
	ev, _ := Parse([]byte(eventoExemplo))
	msg := ev.Normalizar()
	msg.ID, msg.Texto = "escrita-2", "sim"
	msg.Origem, msg.Destino = sess.Origem, sess.Destino
	msg, _ = svc.PrepararMensagem(msg)
	fim := make(chan error, 1)
	go func() { _, err := svc.Processar(context.Background(), msg); fim <- err }()
	select {
	case <-bloqueado.iniciou:
	case <-time.After(5 * time.Second):
		t.Fatal("escrita não iniciou")
	}
	humana := msg
	humana.Autor, humana.Texto = "atendente", "atendimento assumido"
	if !svc.RegistrarHumano(humana) {
		t.Fatal("sessão não encontrada")
	}
	select {
	case <-fim:
	case <-time.After(5 * time.Second):
		t.Fatal("escrita em andamento não foi interrompida")
	}
	cart, err := svc.Pedidos.Ler(context.Background(), contatoDeTeste(t, "adm-a"))
	if err != nil {
		t.Fatal(err)
	}
	p := cart.Por(prot)
	if p == nil || p.Estado != pedidos.Falhou || !strings.Contains(p.Erro, "incerto") {
		t.Fatalf("resultado incerto não foi preservado: %+v", p)
	}
	if fake.tentativasVisita != 1 {
		t.Fatalf("esperava apenas uma tentativa: %d", fake.tentativasVisita)
	}
	svc.Config.MCP = fake
	svc.ExtratorPedidos = pedidos.Regras{}
	msg.ID, msg.Revisao = "escrita-3", 0
	if _, err := svc.Processar(context.Background(), msg); err != nil {
		t.Fatalf("retomada da conversa falhou: %v", err)
	}
	if fake.tentativasVisita != 1 {
		t.Fatalf("resultado incerto não pode repetir a escrita: %d", fake.tentativasVisita)
	}
}

func TestUnidadeInformadaNaoIgnoraListaMCPVazia(t *testing.T) {
	fixarAgoraDoFluxo(t)
	svc, fake := servicoComMCPFluxo(t)
	fake.resultadoEmpresa = resultadoMCPFluxo(`{"data":{"units":[{"id":"unit-109","unit":"A - 101","resident":{"name":"Morador","mobile_number":"5519981621916"}}]}}`)
	svc.ExtratorPedidos = extratorPedidosFunc(func(_ context.Context, _ pedidos.Entrada) (pedidos.Extracao, error) {
		return pedidos.Extracao{Pedidos: []pedidos.PedidoExtraido{{Caso: "liberacao_visitante", Valores: map[string]any{
			"unidade": "109", "visitante_nome": "Ana Souza", "visitante_telefone": "5511999999999", "data_visita": "2026-10-28", "categoria_motivo": "GUEST",
		}}}}, nil
	})
	sess := falar(t, svc, "unidade-inexistente", "quero liberar Ana Souza hoje")
	if texto := ultimoConcierge(sess).Texto; !strings.Contains(texto, "A liberação não está disponível") || fake.tentativasVisita != 0 {
		t.Fatalf("unidade sem ID operacional não pode ser aceita: resposta=%q chamadas=%d", texto, fake.tentativasVisita)
	}
}

func TestReutilizacaoDeMotivoConsultaOpcoesAtuais(t *testing.T) {
	for _, valido := range []bool{true, false} {
		nome := "indisponivel"
		if valido {
			nome = "valido"
		}
		t.Run(nome, func(t *testing.T) {
			fixarAgoraDoFluxo(t)
			svc, fake := servicoComMCPFluxo(t)
			svc.ExtratorPedidos = extratorVisitaMCP()
			falar(t, svc, "reuso-1", "quero liberar Ana Souza hoje")
			falar(t, svc, "reuso-2", "sim")
			if fake.escritasVisita != 1 {
				t.Fatal("primeira visita não foi concluída")
			}
			if !valido {
				fake.motivosVisita = []any{}
			}
			svc.ExtratorPedidos = extratorPedidosFunc(func(ctx context.Context, in pedidos.Entrada) (pedidos.Extracao, error) {
				if strings.Contains(in.Mensagem, "liberar Beatriz") {
					return pedidos.Extracao{Pedidos: []pedidos.PedidoExtraido{{Caso: "liberacao_visitante", Valores: map[string]any{
						"visitante_nome": "Beatriz Silva", "visitante_telefone": "5511999999999", "data_visita": "2026-10-28",
					}}}}, nil
				}
				return (pedidos.Regras{}).Extrair(ctx, in)
			})
			sess := falar(t, svc, "reuso-3", "quero liberar Beatriz hoje")
			if texto := ultimoConcierge(sess).Texto; !strings.Contains(texto, "reutilizar o motivo") {
				t.Fatalf("deveria oferecer a reutilização: %q", texto)
			}
			sess = falar(t, svc, "reuso-4", "sim")
			texto := ultimoConcierge(sess).Texto
			if valido && !strings.Contains(texto, "Confirma a liberação") {
				t.Fatalf("motivo válido deveria prosseguir: %q", texto)
			}
			if !valido && !strings.Contains(texto, "Não há motivos disponíveis") {
				t.Fatalf("motivo removido deveria impedir confirmação: %q", texto)
			}
			if fake.escritasVisita != 1 {
				t.Fatalf("nova visita não deveria ser escrita sem confirmação: %d", fake.escritasVisita)
			}
		})
	}
}

func TestConfirmacaoNaoEntregueNaoAutorizaVisita(t *testing.T) {
	fixarAgoraDoFluxo(t)
	svc, fake := servicoComMCPFluxo(t)
	svc.ExtratorPedidos = extratorVisitaMCP()
	svc.Sender = senderFunc(func(context.Context, entrada.Destino, string) error { return errors.New("envio indisponível") })
	ev, _ := Parse([]byte(eventoExemplo))
	msg := ev.Normalizar()
	msg.ID, msg.Texto = "confirmacao-1", "quero liberar Ana Souza hoje"
	msg.Origem = entrada.Origem{Conector: "atende-emulado", Meio: "whatsapp", ConexaoID: "console"}
	msg.Destino = entrada.Destino{Conector: "atende-emulado", Meio: "whatsapp", ConexaoID: "console", Protocolo: msg.ConversaID, Telefone: msg.Contato.Phone}
	if _, err := svc.Processar(context.Background(), msg); err == nil {
		t.Fatal("envio da confirmação deveria falhar")
	}
	svc.Sender = &Outbox{}
	msg.ID, msg.Texto = "confirmacao-2", "sim"
	sess, err := svc.Processar(context.Background(), msg)
	if err != nil {
		t.Fatal(err)
	}
	if fake.tentativasVisita != 0 {
		t.Fatalf("visita não pode ser criada sem confirmação entregue: %d", fake.tentativasVisita)
	}
	if texto := ultimoConcierge(sess).Texto; !strings.Contains(texto, "Confirma a liberação") {
		t.Fatalf("o pedido deve apresentar a confirmação novamente: %q", texto)
	}
}

type mcpEmpresaBloqueada struct {
	*mcpFluxoFalso
	iniciou chan struct{}
	liberar chan struct{}
}

func (m *mcpEmpresaBloqueada) CallTool(ctx context.Context, nome string, args map[string]any) (*mcp.CallResult, error) {
	if nome == "gruvi.bff.building-operation.get-company-by-document-controller-handle" {
		close(m.iniciou)
		<-m.liberar
	}
	return m.mcpFluxoFalso.CallTool(ctx, nome, args)
}

func TestFechamentoAntesDaEscritaManualInterrompeVisita(t *testing.T) {
	fixarAgoraDoFluxo(t)
	svc, fake := servicoComMCPFluxo(t)
	svc.ExtratorPedidos = extratorVisitaMCP()
	sess := falar(t, svc, "manual-1", "quero liberar Ana Souza hoje")
	prot := ultimoConcierge(sess).Pedidos[0].Protocolo
	sess = falar(t, svc, "manual-2", "quero falar com um atendente")
	sess.mu.Lock()
	sess.mcpResultados = nil
	sess.mu.Unlock()
	bloqueado := &mcpEmpresaBloqueada{mcpFluxoFalso: fake, iniciou: make(chan struct{}), liberar: make(chan struct{})}
	svc.Config.MCP = bloqueado
	fim := make(chan error, 1)
	go func() {
		_, _, _, err := svc.PreencherPedido(context.Background(), sess.ID, prot, "Ana", nil, true)
		fim <- err
	}()
	select {
	case <-bloqueado.iniciou:
	case <-time.After(5 * time.Second):
		t.Fatal("consulta de vínculo não iniciou")
	}
	ev, _ := Parse([]byte(eventoExemplo))
	msg := ev.Normalizar()
	msg.Origem, msg.Destino = sess.Origem, sess.Destino
	if !svc.Encerrar(msg) {
		t.Fatal("sessão não encontrada")
	}
	close(bloqueado.liberar)
	select {
	case <-fim:
	case <-time.After(5 * time.Second):
		t.Fatal("ação manual não encerrou")
	}
	if fake.tentativasVisita != 0 {
		t.Fatalf("escrita manual iniciou após fechamento: %d", fake.tentativasVisita)
	}
}

func TestAlteracaoDeMotivoNoSimExigeNovaConfirmacao(t *testing.T) {
	fixarAgoraDoFluxo(t)
	svc, fake := servicoComMCPFluxo(t)
	fake.motivosVisita = []any{
		map[string]any{"id": 5, "reason": "Manutenção", "type_provider": "SERVICE_PROVIDER"},
		map[string]any{"id": 6, "reason": "Entrega técnica", "type_provider": "SERVICE_PROVIDER"},
	}
	svc.ExtratorPedidos = extratorPedidosFunc(func(ctx context.Context, in pedidos.Entrada) (pedidos.Extracao, error) {
		if strings.Contains(in.Mensagem, "liberar Ana") {
			return pedidos.Extracao{Pedidos: []pedidos.PedidoExtraido{{Caso: "liberacao_visitante", Valores: map[string]any{
				"visitante_nome": "Ana Souza", "visitante_telefone": "5511999999999", "data_visita": "2026-10-28", "categoria_motivo": "SERVICE_PROVIDER", "motivo": 5,
			}}}}, nil
		}
		return (pedidos.Regras{}).Extrair(ctx, in)
	})
	falar(t, svc, "alteracao-1", "quero liberar Ana hoje")
	falar(t, svc, "alteracao-2", "sim")
	if fake.escritasVisita != 1 {
		t.Fatal("a primeira visita deveria estar concluída")
	}
	svc.ExtratorPedidos = extratorPedidosFunc(func(ctx context.Context, in pedidos.Entrada) (pedidos.Extracao, error) {
		if strings.Contains(in.Mensagem, "liberar Beatriz") {
			return pedidos.Extracao{Pedidos: []pedidos.PedidoExtraido{{Caso: "liberacao_visitante", Valores: map[string]any{
				"visitante_nome": "Beatriz Silva", "visitante_telefone": "5511999999999", "data_visita": "2026-10-28",
			}}}}, nil
		}
		if strings.Contains(in.Mensagem, "motivo 6") && len(in.Abertos) > 0 {
			prot := in.Abertos[0].Protocolo
			return pedidos.Extracao{Confirmacoes: []pedidos.Confirmada{{Protocolo: prot, Resposta: "sim"}}, Pedidos: []pedidos.PedidoExtraido{{Protocolo: prot, Caso: "liberacao_visitante", Valores: map[string]any{"motivo": 6}}}}, nil
		}
		return (pedidos.Regras{}).Extrair(ctx, in)
	})
	sess := falar(t, svc, "alteracao-3", "quero liberar Beatriz hoje")
	if !strings.Contains(ultimoConcierge(sess).Texto, "reutilizar o motivo") {
		t.Fatal("deveria oferecer reutilização")
	}
	sess = falar(t, svc, "alteracao-4", "sim")
	if !strings.Contains(ultimoConcierge(sess).Texto, "Confirma a liberação") {
		t.Fatal("deveria confirmar a visita")
	}
	sess = falar(t, svc, "alteracao-5", "sim, mas use o motivo 6")
	if fake.tentativasVisita != 1 {
		t.Fatalf("não pode criar visita com dados alterados sem nova confirmação: %d", fake.tentativasVisita)
	}
	if texto := ultimoConcierge(sess).Texto; !strings.Contains(texto, "Confirma a liberação") || !strings.Contains(texto, "Entrega técnica") {
		t.Fatalf("deveria confirmar o motivo atualizado: %q", texto)
	}
}

func TestEntregaAnteriorNaoConfirmaValoresAlteradosEmOutraSessao(t *testing.T) {
	fixarAgoraDoFluxo(t)
	svc, _ := servicoComMCPFluxo(t)
	svc.ExtratorPedidos = extratorVisitaMCP()
	iniciou, liberar := make(chan struct{}), make(chan struct{})
	defer func() {
		select {
		case <-liberar:
		default:
			close(liberar)
		}
	}()
	svc.Sender = senderFunc(func(context.Context, entrada.Destino, string) error {
		close(iniciou)
		<-liberar
		return nil
	})
	ev, _ := Parse([]byte(eventoExemplo))
	msg := ev.Normalizar()
	msg.ID, msg.Texto = "sessao-a", "quero liberar Ana Souza hoje"
	msg.Origem = entrada.Origem{Conector: "atende-emulado", Meio: "whatsapp", ConexaoID: "console"}
	msg.Destino = entrada.Destino{Conector: "atende-emulado", Meio: "whatsapp", ConexaoID: "console", Protocolo: msg.ConversaID, Telefone: msg.Contato.Phone}
	msg, sessA := svc.PrepararMensagem(msg)
	fim := make(chan error, 1)
	go func() { _, err := svc.Processar(context.Background(), msg); fim <- err }()
	select {
	case <-iniciou:
	case <-time.After(5 * time.Second):
		t.Fatal("entrega inicial não iniciou")
	}
	cart, err := svc.Pedidos.Ler(context.Background(), contatoDeTeste(t, "adm-a"))
	if err != nil || len(cart.Pedidos) != 1 {
		t.Fatalf("pedido não encontrado: %v", err)
	}
	prot := cart.Pedidos[0].Protocolo
	outra := msg
	outra.ConversaID = "outro-protocolo"
	outra.Destino.Protocolo = outra.ConversaID
	sessB := svc.Sessoes.Abrir(outra)
	sessA.mu.Lock()
	identidade := sessA.Identidade
	sessA.mu.Unlock()
	sessB.mu.Lock()
	sessB.Identidade = identidade
	sessB.mu.Unlock()
	sessB.fixarContato(sessA.contato())
	_, p, saida, err := svc.PreencherPedido(context.Background(), sessB.ID, prot, "Ana", map[string]any{"visitante_telefone": "5511888888888"}, true)
	if err != nil || saida.Tipo != "confirmar" || p.Confirmacao == nil {
		t.Fatalf("dados alterados deveriam exigir nova confirmação: %+v %v", saida, err)
	}
	close(liberar)
	select {
	case err := <-fim:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("entrega inicial não terminou")
	}
	cart, err = svc.Pedidos.Ler(context.Background(), contatoDeTeste(t, "adm-a"))
	if err != nil {
		t.Fatal(err)
	}
	p = cart.Por(prot)
	if p == nil || p.Confirmacao == nil || p.Confirmacao.Entregue == nil || *p.Confirmacao.Entregue {
		t.Fatalf("entrega antiga não pode confirmar valores alterados: %+v", p)
	}
}

func TestFechamentoDuranteEscritaManualPreservaEstadoIncerto(t *testing.T) {
	fixarAgoraDoFluxo(t)
	svc, fake := servicoComMCPFluxo(t)
	svc.ExtratorPedidos = extratorVisitaMCP()
	sess := falar(t, svc, "manual-incerto-1", "quero liberar Ana Souza hoje")
	prot := ultimoConcierge(sess).Pedidos[0].Protocolo
	sess = falar(t, svc, "manual-incerto-2", "quero falar com um atendente")
	bloqueado := &mcpVisitaBloqueado{mcpFluxoFalso: fake, iniciou: make(chan struct{})}
	svc.Config.MCP = bloqueado
	fim := make(chan error, 1)
	go func() {
		_, _, _, err := svc.PreencherPedido(context.Background(), sess.ID, prot, "Ana", nil, true)
		fim <- err
	}()
	select {
	case <-bloqueado.iniciou:
	case <-time.After(5 * time.Second):
		t.Fatal("escrita manual não iniciou")
	}
	ev, _ := Parse([]byte(eventoExemplo))
	msg := ev.Normalizar()
	msg.Origem, msg.Destino = sess.Origem, sess.Destino
	if !svc.Encerrar(msg) {
		t.Fatal("sessão não encontrada")
	}
	select {
	case <-fim:
	case <-time.After(5 * time.Second):
		t.Fatal("escrita manual não encerrou")
	}
	cart, err := svc.Pedidos.Ler(context.Background(), contatoDeTeste(t, "adm-a"))
	if err != nil {
		t.Fatal(err)
	}
	p := cart.Por(prot)
	if p == nil || p.Estado != pedidos.Falhou || !strings.Contains(p.Erro, "incerto") {
		t.Fatalf("resultado incerto não foi preservado: %+v", p)
	}
	if fake.tentativasVisita != 1 {
		t.Fatalf("esperava uma tentativa: %d", fake.tentativasVisita)
	}
}

func TestConfirmacaoEmEntregaParcialNaoAutorizaVisita(t *testing.T) {
	fixarAgoraDoFluxo(t)
	svc, fake := servicoComMCPFluxo(t)
	svc.ExtratorPedidos = extratorVisitaMCP()
	svc.Sender = senderFunc(func(context.Context, entrada.Destino, string) error {
		return &entregaParcial{partes: []string{"primeira parte entregue"}, erro: errors.New("falha na parte seguinte")}
	})
	ev, _ := Parse([]byte(eventoExemplo))
	msg := ev.Normalizar()
	msg.ID, msg.Texto = "confirmacao-parcial-1", "quero liberar Ana Souza hoje"
	msg.Origem = entrada.Origem{Conector: "atende-emulado", Meio: "whatsapp", ConexaoID: "console"}
	msg.Destino = entrada.Destino{Conector: "atende-emulado", Meio: "whatsapp", ConexaoID: "console", Protocolo: msg.ConversaID, Telefone: msg.Contato.Phone}
	if _, err := svc.Processar(context.Background(), msg); err == nil {
		t.Fatal("entrega parcial deve retornar erro")
	}
	svc.Sender = &Outbox{}
	msg.ID, msg.Texto = "confirmacao-parcial-2", "sim"
	sess, err := svc.Processar(context.Background(), msg)
	if err != nil {
		t.Fatal(err)
	}
	if fake.tentativasVisita != 0 {
		t.Fatalf("entrega parcial não pode autorizar visita: %d", fake.tentativasVisita)
	}
	if texto := ultimoConcierge(sess).Texto; !strings.Contains(texto, "Confirma a liberação") {
		t.Fatalf("a confirmação deve ser apresentada novamente: %q", texto)
	}
}

func TestConfirmacaoLegadaSemMarcadoresContinuaValida(t *testing.T) {
	fixarAgoraDoFluxo(t)
	svc, fake := servicoComMCPFluxo(t)
	svc.ExtratorPedidos = extratorVisitaMCP()
	sess := falar(t, svc, "legado-1", "quero liberar Ana Souza hoje")
	prot := ultimoConcierge(sess).Pedidos[0].Protocolo
	cart, err := svc.Pedidos.Ler(context.Background(), contatoDeTeste(t, "adm-a"))
	if err != nil {
		t.Fatal(err)
	}
	p := cart.Por(prot)
	if p == nil || p.Confirmacao == nil {
		t.Fatal("confirmação não encontrada")
	}
	p.Confirmacao.Entregue = nil
	p.Confirmacao.Valores = nil
	if err := svc.Pedidos.Gravar(context.Background(), contatoDeTeste(t, "adm-a"), cart); err != nil {
		t.Fatal(err)
	}
	falar(t, svc, "legado-2", "sim")
	if fake.tentativasVisita != 1 {
		t.Fatalf("confirmação anterior deve continuar válida: %d", fake.tentativasVisita)
	}
}
