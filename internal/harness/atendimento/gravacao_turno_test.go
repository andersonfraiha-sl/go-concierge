package atendimento

import (
	"bytes"
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"go-concierge/internal/core/banco"
	"go-concierge/internal/escopo"
	"go-concierge/internal/harness/ganchos/auditoria"
	"go-concierge/internal/harness/memoria"
	"go-concierge/internal/harness/pedidos"
)

type gravacaoTurnoFunc func(context.Context, RegistroSessao, escopo.Contato, pedidos.Carteira, []auditoria.Linha) error

func (f gravacaoTurnoFunc) Gravar(ctx context.Context, r RegistroSessao, c escopo.Contato, cart pedidos.Carteira, linhas []auditoria.Linha) error {
	return f(ctx, r, c, cart, linhas)
}

type pedidoComResultadoIncerto struct {
	pedidos.Store
	gravacoes int
}

func (s *pedidoComResultadoIncerto) Gravar(context.Context, escopo.Contato, pedidos.Carteira) error {
	s.gravacoes++
	return banco.ErrCommitIncerto
}

type contarGravacoesSessao struct {
	SessaoStore
	gravacoes int
}

type sessaoComResultadoIncerto struct {
	SessaoStore
	gravacoes int
}

func (s *sessaoComResultadoIncerto) Salvar(ctx context.Context, r RegistroSessao) error {
	s.gravacoes++
	if err := s.SessaoStore.Salvar(ctx, r); err != nil {
		return err
	}
	return banco.ErrCommitIncerto
}

func (s *contarGravacoesSessao) Salvar(ctx context.Context, r RegistroSessao) error {
	s.gravacoes++
	return s.SessaoStore.Salvar(ctx, r)
}

type contarGravacoesPedido struct {
	pedidos.Store
	gravacoes int
}

type contarMemoria struct {
	memoria.Store
	gravacoes atomic.Int32
}

type memoriaComResultadoIncerto struct {
	memoria.Store
	gravacoes int
}

func (s *memoriaComResultadoIncerto) Gravar(ctx context.Context, c escopo.Contato, m memoria.Memoria) error {
	s.gravacoes++
	if err := s.Store.Gravar(ctx, c, m); err != nil {
		return err
	}
	return banco.ErrCommitIncerto
}

type memoriaQueNaoLe struct {
	memoria.Store
	err error
}

func (s memoriaQueNaoLe) Ler(context.Context, escopo.Contato) (memoria.Memoria, error) {
	return memoria.Memoria{}, s.err
}

func (s *contarMemoria) Gravar(ctx context.Context, c escopo.Contato, m memoria.Memoria) error {
	s.gravacoes.Add(1)
	return s.Store.Gravar(ctx, c, m)
}

type extratorParado struct {
	iniciou  chan struct{}
	liberar  chan struct{}
	chamadas atomic.Int32
}

func (e *extratorParado) Extrair(context.Context, string, string, memoria.Memoria) ([]memoria.Fato, error) {
	e.chamadas.Add(1)
	close(e.iniciou)
	<-e.liberar
	return []memoria.Fato{{Chave: "teste", Texto: "fato"}}, nil
}

func (s *contarGravacoesPedido) Gravar(ctx context.Context, c escopo.Contato, cart pedidos.Carteira) error {
	s.gravacoes++
	return s.Store.Gravar(ctx, c, cart)
}

func TestGravacaoTurnoIncertaNaoRepeteNemExecutaFallback(t *testing.T) {
	svc, _, basePedidos := servicoComPedidos(t)
	sess := falar(t, svc, "gravacao-incerta", textoTurno)
	contato := sess.contato()
	cart, err := basePedidos.Ler(t.Context(), contato)
	if err != nil {
		t.Fatal(err)
	}
	baseSessoes := svc.Sessoes.store
	sessoes := &contarGravacoesSessao{SessaoStore: baseSessoes}
	spyPedidos := &contarGravacoesPedido{Store: basePedidos}
	svc.Sessoes.store = sessoes
	svc.Pedidos = spyPedidos
	svc.adiarCarteira(contato, cart)
	chamadas := 0
	svc.GravacaoTurno = gravacaoTurnoFunc(func(ctx context.Context, r RegistroSessao, c escopo.Contato, cart pedidos.Carteira, _ []auditoria.Linha) error {
		chamadas++
		if err := baseSessoes.Salvar(ctx, r); err != nil {
			t.Fatal(err)
		}
		if err := basePedidos.Gravar(ctx, c, cart); err != nil {
			t.Fatal(err)
		}
		return errors.New("resposta perdida após aplicação")
	})
	linhas := []auditoria.Linha{{Motivo: "original"}}
	if err := svc.gravarDepoisDaResposta(t.Context(), sess, linhas); !errors.Is(err, ErrGravacaoIncerta) {
		t.Fatalf("resultado = %v", err)
	}
	if sess.pendente == nil || len(sess.pendente.carteira) == 0 || len(sess.pendente.auditoria) != 1 {
		t.Fatal("comando incerto não foi preservado")
	}
	carteiraPendente := bytes.Clone(sess.pendente.carteira)
	registroPendente := bytes.Clone(sess.pendente.registro.Dados)
	linhas[0].Motivo = "alterado"
	cart.Pedidos[0].Valores["teste"] = "alterado"
	sess.registrar(Turno{Quem: "sistema", Texto: "alteração posterior"})
	if !bytes.Equal(sess.pendente.carteira, carteiraPendente) || !bytes.Equal(sess.pendente.registro.Dados, registroPendente) || sess.pendente.auditoria[0].Motivo != "original" {
		t.Fatal("comando pendente foi alterado por estado vivo")
	}
	if _, ok := svc.carteiraAdiada(contato); !ok {
		t.Fatal("carteira pendente foi descartada")
	}
	if err := svc.gravarDepoisDaResposta(t.Context(), sess, nil); !errors.Is(err, ErrGravacaoIncerta) {
		t.Fatalf("segunda gravação = %v", err)
	}
	if _, err := svc.carregarCarteira(t.Context(), contato); !errors.Is(err, ErrGravacaoIncerta) {
		t.Fatalf("carregamento = %v", err)
	}
	if _, err := svc.carteiraDoTurno(t.Context(), contato); !errors.Is(err, ErrGravacaoIncerta) {
		t.Fatalf("carteira do turno = %v", err)
	}
	if err := svc.Sessoes.Persistir(t.Context(), sess); !errors.Is(err, ErrGravacaoIncerta) {
		t.Fatalf("persistência posterior = %v", err)
	}
	if _, _, _, err := svc.PreencherPedido(t.Context(), sess.ID, "pedido", "Ana", nil, false); !errors.Is(err, ErrGravacaoIncerta) {
		t.Fatalf("ação posterior: erro=%v", err)
	}
	if chamadas != 1 || sessoes.gravacoes != 0 || spyPedidos.gravacoes != 0 {
		t.Fatalf("chamadas=%d sessões=%d pedidos=%d", chamadas, sessoes.gravacoes, spyPedidos.gravacoes)
	}
}

func TestPersistenciaIndividualIncertaNaoRepete(t *testing.T) {
	svc, _, _ := servicoComPedidos(t)
	sess := falar(t, svc, "sessao-individual", textoTurno)
	store := &sessaoComResultadoIncerto{SessaoStore: svc.Sessoes.store}
	svc.Sessoes.store = store
	linhas := []auditoria.Linha{{Motivo: "auditoria pendente"}}
	if err := svc.persistirComAuditoria(t.Context(), sess, linhas); !errors.Is(err, ErrGravacaoIncerta) {
		t.Fatalf("persistência = %v", err)
	}
	if sess.pendente == nil || len(sess.pendente.auditoria) != 1 || sess.pendente.auditoria[0].Motivo != "auditoria pendente" {
		t.Fatal("sessão e auditoria pendentes não foram preservadas")
	}
	if err := svc.Sessoes.Persistir(t.Context(), sess); !errors.Is(err, ErrGravacaoIncerta) || store.gravacoes != 1 {
		t.Fatalf("nova persistência: erro=%v gravações=%d", err, store.gravacoes)
	}
}

func TestGravacaoTurnoRejeitadaPermiteFallback(t *testing.T) {
	svc, _, basePedidos := servicoComPedidos(t)
	sess := falar(t, svc, "gravacao-rejeitada", textoTurno)
	contato := sess.contato()
	cart, err := basePedidos.Ler(t.Context(), contato)
	if err != nil {
		t.Fatal(err)
	}
	sessoes := &contarGravacoesSessao{SessaoStore: svc.Sessoes.store}
	spyPedidos := &contarGravacoesPedido{Store: basePedidos}
	svc.Sessoes.store = sessoes
	svc.Pedidos = spyPedidos
	svc.adiarCarteira(contato, cart)
	svc.GravacaoTurno = gravacaoTurnoFunc(func(context.Context, RegistroSessao, escopo.Contato, pedidos.Carteira, []auditoria.Linha) error {
		return ErrGravacaoNaoAplicada
	})
	if err := svc.gravarDepoisDaResposta(t.Context(), sess, nil); err != nil {
		t.Fatal(err)
	}
	if sessoes.gravacoes != 1 || spyPedidos.gravacoes != 1 {
		t.Fatalf("sessões=%d pedidos=%d", sessoes.gravacoes, spyPedidos.gravacoes)
	}
}

func TestGravacaoTurnoRecusaDeEscopoNaoExecutaFallback(t *testing.T) {
	svc, _, basePedidos := servicoComPedidos(t)
	sess := falar(t, svc, "escopo-recusado", textoTurno)
	contato := sess.contato()
	cart, err := basePedidos.Ler(t.Context(), contato)
	if err != nil {
		t.Fatal(err)
	}
	sessoes := &contarGravacoesSessao{SessaoStore: svc.Sessoes.store}
	spyPedidos := &contarGravacoesPedido{Store: basePedidos}
	svc.Sessoes.store = sessoes
	svc.Pedidos = spyPedidos
	svc.adiarCarteira(contato, cart)
	sess.registrar(Turno{Quem: "sistema", Texto: "não deve voltar após recusa"})
	chamadas := 0
	svc.GravacaoTurno = gravacaoTurnoFunc(func(context.Context, RegistroSessao, escopo.Contato, pedidos.Carteira, []auditoria.Linha) error {
		chamadas++
		return errors.Join(ErrGravacaoNaoAplicada, ErrGravacaoRecusada)
	})
	if err := svc.gravarDepoisDaResposta(t.Context(), sess, nil); !errors.Is(err, ErrGravacaoRecusada) || errors.Is(err, ErrGravacaoIncerta) {
		t.Fatalf("resultado = %v", err)
	}
	if chamadas != 1 || sessoes.gravacoes != 0 || spyPedidos.gravacoes != 0 {
		t.Fatalf("chamadas=%d sessões=%d pedidos=%d", chamadas, sessoes.gravacoes, spyPedidos.gravacoes)
	}
	if err := svc.Sessoes.Persistir(t.Context(), sess); !errors.Is(err, ErrGravacaoRecusada) {
		t.Fatalf("ponteiro anterior: %v", err)
	}
	restaurada, ok, err := svc.Sessoes.Get(t.Context(), sess.ID)
	if err != nil || !ok || restaurada == sess {
		t.Fatalf("restauração: encontrada=%v mesma=%v erro=%v", ok, restaurada == sess, err)
	}
	for _, turno := range restaurada.Turnos {
		if turno.Texto == "não deve voltar após recusa" {
			t.Fatal("estado recusado reapareceu")
		}
	}
	recarregada, err := svc.carregarCarteira(t.Context(), contato)
	if err != nil || spyPedidos.gravacoes != 0 {
		t.Fatalf("recarregar após recusa: %v, gravações=%d", err, spyPedidos.gravacoes)
	}
	svc.adiarCarteira(contato, recarregada)
	svc.GravacaoTurno = gravacaoTurnoFunc(func(context.Context, RegistroSessao, escopo.Contato, pedidos.Carteira, []auditoria.Linha) error {
		chamadas++
		return nil
	})
	if err := svc.gravarDepoisDaResposta(t.Context(), restaurada, nil); err != nil || chamadas != 2 {
		t.Fatalf("operação posterior: %v, chamadas=%d", err, chamadas)
	}
}

func TestTurnoSeguinteAguardaPersistenciaIncerta(t *testing.T) {
	svc, out, _ := servicoComPedidos(t)
	sess := falar(t, svc, "seed", textoTurno)
	ex := &extratorParado{iniciou: make(chan struct{}), liberar: make(chan struct{})}
	svc.Memorias = memoria.NovoFake()
	svc.Extrator = ex
	iniciou := make(chan struct{})
	liberar := make(chan struct{})
	var chamadas atomic.Int32
	svc.GravacaoTurno = gravacaoTurnoFunc(func(context.Context, RegistroSessao, escopo.Contato, pedidos.Carteira, []auditoria.Linha) error {
		if chamadas.Add(1) == 1 {
			close(iniciou)
			<-liberar
		}
		return errors.New("resposta perdida")
	})
	ctx, cancelar := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancelar()
	primeira := mensagemDeTexto(t, "bom dia")
	primeira.ID = "primeira"
	primeiroFim := make(chan error, 1)
	go func() { _, err := svc.Processar(ctx, primeira); primeiroFim <- err }()
	select {
	case <-iniciou:
	case err := <-primeiroFim:
		t.Fatalf("primeiro turno não iniciou gravação: %v", err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	entregues := len(out.Itens(sess.ID))
	segunda := mensagemDeTexto(t, "outra mensagem")
	segunda.ID = "segunda"
	segundoFim := make(chan error, 1)
	go func() { _, err := svc.Processar(ctx, segunda); segundoFim <- err }()
	select {
	case err := <-segundoFim:
		t.Fatalf("segundo turno terminou antes da confirmação: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	if len(out.Itens(sess.ID)) != entregues {
		t.Fatal("segundo turno produziu entrega durante a gravação")
	}
	close(liberar)
	if err := <-primeiroFim; !errors.Is(err, ErrGravacaoIncerta) {
		t.Fatalf("primeiro turno: %v", err)
	}
	if err := <-segundoFim; !errors.Is(err, ErrGravacaoIncerta) {
		t.Fatalf("segundo turno: %v", err)
	}
	svc.Aguardar()
	if chamadas.Load() != 1 || ex.chamadas.Load() != 0 || len(out.Itens(sess.ID)) != entregues {
		t.Fatalf("gravações=%d extrações=%d entregas=%d", chamadas.Load(), ex.chamadas.Load(), len(out.Itens(sess.ID)))
	}
}

func TestExtracaoAntigaNaoGravaAposResultadoIncerto(t *testing.T) {
	svc, _, basePedidos := servicoComPedidos(t)
	sess := falar(t, svc, "seed-extracao", textoTurno)
	contato := sess.contato()
	cart, err := basePedidos.Ler(t.Context(), contato)
	if err != nil {
		t.Fatal(err)
	}
	mem := &contarMemoria{Store: memoria.NovoFake()}
	ex := &extratorParado{iniciou: make(chan struct{}), liberar: make(chan struct{})}
	svc.Memorias, svc.Extrator = mem, ex
	fim := make(chan []memoria.Fato, 1)
	go func() {
		atual := memoria.Memoria{}
		fim <- svc.lembrar(t.Context(), &atual, sess, "mensagem", "resposta", 0)
	}()
	select {
	case <-ex.iniciou:
	case <-time.After(5 * time.Second):
		t.Fatal("extração não iniciou")
	}
	svc.adiarCarteira(contato, cart)
	svc.GravacaoTurno = gravacaoTurnoFunc(func(context.Context, RegistroSessao, escopo.Contato, pedidos.Carteira, []auditoria.Linha) error {
		return errors.New("resposta perdida")
	})
	if err := svc.gravarDepoisDaResposta(t.Context(), sess, nil); !errors.Is(err, ErrGravacaoIncerta) {
		t.Fatalf("gravação = %v", err)
	}
	close(ex.liberar)
	select {
	case fatos := <-fim:
		if len(fatos) != 0 || mem.gravacoes.Load() != 0 {
			t.Fatalf("fatos=%v gravações=%d", fatos, mem.gravacoes.Load())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("extração não terminou")
	}
}

func TestMemoriaIncertaNaoRepeteNemIniciaOutroTurno(t *testing.T) {
	svc, out, _ := servicoComPedidos(t)
	sess := falar(t, svc, "seed-memoria", textoTurno)
	contato := sess.contato()
	store := &memoriaComResultadoIncerto{Store: memoria.NovoFake()}
	svc.Memorias = store
	m := memoria.Memoria{CompanyID: contato.Canonico().CompanyID, Telefone: contato.Telefone, Fatos: []memoria.Fato{{Chave: "teste", Texto: "original", CondoID: contato.CondoID}}}
	if err := svc.gravarMemoria(t.Context(), contato, m); !errors.Is(err, ErrGravacaoIncerta) {
		t.Fatalf("gravação = %v", err)
	}
	pendente := svc.memoriasBloqueadas[contato.ChavePessoa()][contato.Chave()]
	dados := bytes.Clone(pendente.dados)
	m.Fatos[0].Texto = "alterado"
	if !bytes.Equal(dados, pendente.dados) {
		t.Fatal("memória pendente foi alterada")
	}
	if err := svc.gravarMemoria(t.Context(), contato, m); !errors.Is(err, ErrGravacaoIncerta) || store.gravacoes != 1 {
		t.Fatalf("repetição: erro=%v gravações=%d", err, store.gravacoes)
	}
	if _, err := svc.EsquecerMemoria(t.Context(), contato, ""); !errors.Is(err, ErrGravacaoIncerta) {
		t.Fatalf("esquecimento = %v", err)
	}
	entregues := len(out.Itens(sess.ID))
	msg := mensagemDeTexto(t, "outra mensagem")
	msg.ID = "memoria-pendente"
	if _, err := svc.Processar(t.Context(), msg); !errors.Is(err, ErrGravacaoIncerta) {
		t.Fatalf("turno posterior = %v", err)
	}
	if len(out.Itens(sess.ID)) != entregues {
		t.Fatal("turno posterior gerou entrega")
	}
	outro := contato.Canonico()
	condo := 202
	outro.CondoID = &condo
	if err := svc.bloqueioMemoria(outro); err != nil {
		t.Fatalf("condomínio independente bloqueado: %v", err)
	}
	amplo := contato.Canonico()
	amplo.CondoID, amplo.CondominioID = nil, ""
	if err := svc.bloqueioMemoria(amplo); !errors.Is(err, ErrGravacaoIncerta) {
		t.Fatalf("consulta ampla não bloqueada: %v", err)
	}
}

func TestFalhaDeLeituraDeMemoriaNaoViraVazio(t *testing.T) {
	svc, _, _ := servicoComPedidos(t)
	falar(t, svc, "seed-leitura", textoTurno)
	falha := errors.New("memória indisponível")
	svc.Memorias = memoriaQueNaoLe{Store: memoria.NovoFake(), err: falha}
	msg := mensagemDeTexto(t, "outra mensagem")
	msg.ID = "erro-memoria"
	if _, err := svc.Processar(t.Context(), msg); !errors.Is(err, falha) {
		t.Fatalf("turno = %v", err)
	}
}

func TestFallbackInterrompeAoReceberResultadoIncertoDoPedido(t *testing.T) {
	svc, _, basePedidos := servicoComPedidos(t)
	sess := falar(t, svc, "pedido-incerto", textoTurno)
	contato := sess.contato()
	cart, err := basePedidos.Ler(t.Context(), contato)
	if err != nil {
		t.Fatal(err)
	}
	sessoes := &contarGravacoesSessao{SessaoStore: svc.Sessoes.store}
	spyPedidos := &pedidoComResultadoIncerto{Store: basePedidos}
	svc.Sessoes.store = sessoes
	svc.Pedidos = spyPedidos
	svc.adiarCarteira(contato, cart)
	if err := svc.gravarDepoisDaResposta(t.Context(), sess, nil); !errors.Is(err, ErrGravacaoIncerta) {
		t.Fatalf("resultado = %v", err)
	}
	if _, err := svc.carregarCarteira(t.Context(), contato); !errors.Is(err, ErrGravacaoIncerta) {
		t.Fatalf("carregamento = %v", err)
	}
	if sessoes.gravacoes != 0 || spyPedidos.gravacoes != 1 {
		t.Fatalf("sessões=%d pedidos=%d", sessoes.gravacoes, spyPedidos.gravacoes)
	}
}

func TestGravacaoIncertaBloqueiaSomenteEscoposSobrepostos(t *testing.T) {
	alpha, beta := 1, 2
	base := escopo.Contato{CompanyID: "empresa", Telefone: "5511999999999", CondoID: &alpha}
	for _, tc := range []struct {
		nome     string
		incerto  escopo.Contato
		consulta escopo.Contato
		bloqueia bool
	}{
		{"mesmo condomínio", base, base, true},
		{"outro condomínio", base, escopo.Contato{CompanyID: "empresa", Telefone: base.Telefone, CondoID: &beta}, false},
		{"consulta ampla", base, escopo.Contato{CompanyID: "empresa", Telefone: base.Telefone}, true},
		{"escrita ampla", escopo.Contato{CompanyID: "empresa", Telefone: base.Telefone}, escopo.Contato{CompanyID: "empresa", Telefone: base.Telefone, CondoID: &beta}, true},
		{"outra empresa", base, escopo.Contato{CompanyID: "outra", Telefone: base.Telefone, CondoID: &alpha}, false},
		{"outra pessoa", base, escopo.Contato{CompanyID: "empresa", Telefone: "5511888888888", CondoID: &alpha}, false},
		{"condomínio legado", base, escopo.Contato{CompanyID: "empresa", Telefone: base.Telefone, CondominioID: "2"}, false},
	} {
		t.Run(tc.nome, func(t *testing.T) {
			svc := &Servico{}
			svc.bloquearCarteira(tc.incerto, ErrGravacaoIncerta)
			if bloqueou := errors.Is(svc.bloqueioCarteira(tc.consulta), ErrGravacaoIncerta); bloqueou != tc.bloqueia {
				t.Fatalf("bloqueio = %v, esperado %v", bloqueou, tc.bloqueia)
			}
		})
	}
}
