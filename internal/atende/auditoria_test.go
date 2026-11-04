package atende

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"go-concierge/internal/banco"
	"go-concierge/internal/banco/bancotest"
	"go-concierge/internal/banco/ent"
	"go-concierge/internal/banco/ent/ganchosauditoria"
	"go-concierge/internal/entrada"
	"go-concierge/internal/escopo"
	"go-concierge/internal/ganchos"
	"go-concierge/internal/ganchos/auditoria"
	"go-concierge/internal/ganchos/fixos"
	"go-concierge/internal/ganchos/ganchostest"
	"go-concierge/internal/pedidos"
)

// envioQueConta guarda quantas linhas de auditoria já estavam no banco quando a resposta saiu.
type envioQueConta struct {
	db            banco.Banco
	linhasNoEnvio atomic.Int64
	enviou        atomic.Bool
}

func (e *envioQueConta) Enviar(ctx context.Context, _ entrada.Destino, _ string) error {
	n, err := e.db.Ent.GanchosAuditoria.Query().Count(ctx)
	if err != nil {
		return err
	}
	e.linhasNoEnvio.Store(int64(n))
	e.enviou.Store(true)
	return nil
}

// regraQueBloqueia é uma regra global de r-1, versão 3, que bloqueia a resposta.
func regraQueBloqueia() *ganchos.Indice {
	roteiro := &ganchostest.Roteiro[ganchos.AntesDeResponder]{Decisao: ganchos.Decisao{Tipo: ganchos.Bloquear, Motivo: "Hoje não respondo isso."}}
	g := ganchostest.Gancho("regra", roteiro)
	g.Origem = ganchos.Origem{RegraID: "r-1", RegraVersao: 3}
	return &ganchos.Indice{AntesDeResponder: ganchos.Nivel[ganchos.AntesDeResponder]{Globais: []ganchos.Gancho[ganchos.AntesDeResponder]{g}}}
}

// regraNoEncerramento é uma regra de r-enc que decide no encerramento (que nada impede).
func regraNoEncerramento() *ganchos.Indice {
	roteiro := &ganchostest.Roteiro[ganchos.TransbordoOuEncerramento]{Decisao: ganchos.Decisao{Tipo: ganchos.Bloquear, Motivo: "registrar"}}
	g := ganchostest.Gancho("regra", roteiro)
	g.Origem = ganchos.Origem{RegraID: "r-enc", RegraVersao: 1}
	return &ganchos.Indice{TransbordoOuEncerramento: ganchos.Nivel[ganchos.TransbordoOuEncerramento]{Globais: []ganchos.Gancho[ganchos.TransbordoOuEncerramento]{g}}}
}

// comAuditor liga ao serviço o auditor e um conjunto com os fixos e o índice idx.
func comAuditor(svc *Servico, db banco.Banco, idx *ganchos.Indice) *atomic.Int32 {
	var falhas atomic.Int32
	aud := auditoria.NovoAuditor(db, func(string) { falhas.Add(1) }, time.Now)
	svc.Ganchos = ganchos.NovoConjunto(fixos.Todos(), func() *ganchos.Indice { return idx }, aud.Observador())
	svc.Auditoria = aud
	return &falhas
}

func linhasDaRegra(t *testing.T, db banco.Banco, regraID string) []*ent.GanchosAuditoria {
	t.Helper()
	linhas, err := db.Ent.GanchosAuditoria.Query().Where(ganchosauditoria.RegraID(regraID)).All(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	return linhas
}

func TestAuditoriaDoBloqueioVaiAoBancoDepoisDoEnvio(t *testing.T) {
	svc, db, _ := servicoNoPostgres(t)
	envio := &envioQueConta{db: db}
	svc.Sender = envio
	comAuditor(svc, db, regraQueBloqueia())
	inicio := time.Now()

	sess, err := svc.Processar(context.Background(), mensagemDoMorador(eventoCom(t, textoSemPedido), 1, 0))

	if err != nil {
		t.Fatal(err)
	}
	if !envio.enviou.Load() || envio.linhasNoEnvio.Load() != 0 {
		t.Fatalf("enviou=%v com %d linhas de auditoria já gravadas, want envio antes da auditoria",
			envio.enviou.Load(), envio.linhasNoEnvio.Load())
	}
	linhas := linhasDaRegra(t, db, "r-1")
	if len(linhas) != 1 {
		t.Fatalf("linhas da regra = %d, want 1 (sem gravação dupla)", len(linhas))
	}
	l := linhas[0]
	if l.RegraVersao != 3 || l.Decisao != "bloquear" || l.Evento != string(ganchos.EventoAntesDeResponder) ||
		l.CondominioID != sess.contato().CondominioID || l.CondominioID == "" || l.SessaoID != sess.ID || l.Protocolo != sess.Protocolo {
		t.Errorf("linha = %+v", l)
	}
	if l.CriadoEm.Before(inicio) || l.CriadoEm.After(time.Now()) {
		t.Errorf("criado_em %v fora do turno (começou %v)", l.CriadoEm, inicio)
	}
}

func TestAuditoriaDoEncerramentoGravaNaHoraUmaVez(t *testing.T) {
	svc, db, _ := servicoNoPostgres(t)
	comAuditor(svc, db, regraNoEncerramento())
	ev := eventoCom(t, textoSemPedido)
	sess, err := svc.Processar(context.Background(), mensagemDoMorador(ev, 1, 0))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := svc.Encerrar(context.Background(), mensagemDoMorador(ev, 1, 1)); err != nil {
		t.Fatal(err)
	}

	linhas := linhasDaRegra(t, db, "r-enc")
	if len(linhas) != 1 || linhas[0].SessaoID != sess.ID || linhas[0].Evento != string(ganchos.EventoTransbordoOuEncerramento) {
		t.Fatalf("linhas do encerramento = %+v, want uma, da sessão %s", linhas, sess.ID)
	}
}

func TestAuditoriaNoLotePosResposta(t *testing.T) {
	agora := time.Now()
	linha := func(regra, motivo string) auditoria.Linha {
		return auditoria.Linha{CriadoEm: agora, Origem: ganchos.OrigemConcierge, Evento: "antes_de_responder",
			Gancho: "regra", RegraID: regra, RegraVersao: 1, Decisao: "bloquear", Motivo: motivo}
	}
	tests := []struct {
		nome      string
		linhas    []auditoria.Linha
		idas      int
		gravadas  int
		perdidas  int32
		descricao string
	}{
		{"lote aceito grava tudo numa ida", []auditoria.Linha{linha("r-a", "ok"), linha("r-b", "ok")}, 1, 2, 0, ""},
		// o Postgres recusa NUL em text: o lote inteiro volta e cada parte grava sozinha
		{"lote recusado cai em linha a linha", []auditoria.Linha{linha("r-a", "ok"), linha("r-b", "nul\x00")}, 0, 1, 1, ""},
	}
	for _, tt := range tests {
		t.Run(tt.nome, func(t *testing.T) {
			svc, db, contador := servicoNoPostgres(t)
			falhas := comAuditor(svc, db, nil)
			sess, contato, cart := turnoComCarteiraAdiada(t, svc, db)
			contador.zerar()

			err := svc.gravarDepoisDaResposta(context.Background(), sess, tt.linhas)

			if err != nil {
				t.Fatalf("err = %v, want nil: linha de auditoria perdida não é falha do turno", err)
			}
			if idas, _ := contador.ate(); tt.idas > 0 && idas != tt.idas {
				t.Errorf("idas = %d, want %d", idas, tt.idas)
			}
			if n := len(linhasDaRegra(t, db, "r-a")) + len(linhasDaRegra(t, db, "r-b")); n != tt.gravadas {
				t.Errorf("linhas gravadas = %d, want %d", n, tt.gravadas)
			}
			if falhas.Load() != tt.perdidas {
				t.Errorf("falhas contadas = %d, want %d", falhas.Load(), tt.perdidas)
			}
			exigirCarteiraGravada(t, db, contato, cart)
		})
	}
}

// turnoComCarteiraAdiada roda um turno que abre pedido e deixa uma mudança menor da carteira
// para depois da resposta, como o turno faz.
func turnoComCarteiraAdiada(t *testing.T, svc *Servico, db banco.Banco) (*Sessao, escopo.Contato, pedidos.Carteira) {
	t.Helper()
	msg := mensagemDoMorador(eventoDoTurno(t), 1, 0)
	exigirAgendamento(t, svc, db, msg)
	sess, ok, err := svc.Sessao(t.Context(), msg.ConversaID)
	if err != nil || !ok {
		t.Fatalf("sessão do turno: %v", err)
	}
	contato := sess.contato()
	cart, err := svc.Pedidos.Ler(t.Context(), contato)
	if err != nil || len(cart.Pedidos) == 0 {
		t.Fatalf("carteira %+v: %v", cart.Pedidos, err)
	}
	cart.Pedidos[0].Valores["observacao"] = "mudança depois da resposta"
	svc.adiarCarteira(contato, cart)
	return sess, contato, cart
}

func exigirCarteiraGravada(t *testing.T, db banco.Banco, contato escopo.Contato, cart pedidos.Carteira) {
	t.Helper()
	gravada, err := pedidos.NovoPostgres(db).Ler(t.Context(), contato)
	if err != nil {
		t.Fatal(err)
	}
	p := gravada.Por(cart.Pedidos[0].Protocolo)
	if p == nil || p.Valores["observacao"] != "mudança depois da resposta" {
		t.Fatalf("carteira adiada não gravada: %+v", gravada.Pedidos)
	}
}

// servicoDoFluxoAuditado é o fluxo MCP de liberação de visitante com o auditor gravando
// num Postgres próprio e um Sender que conta as linhas já gravadas no envio.
func servicoDoFluxoAuditado(t *testing.T, idx *ganchos.Indice) (*Servico, banco.Banco, *envioQueConta) {
	t.Helper()
	fixarAgoraDoFluxo(t)
	svc, _ := servicoComMCPFluxo(t)
	db := bancotest.Novo(t)
	envio := &envioQueConta{db: db}
	svc.Sender, svc.ExtratorPedidos = envio, extratorVisitaMCP()
	comAuditor(svc, db, idx)
	return svc, db, envio
}

func exigirAuditadaDepoisDoEnvio(t *testing.T, db banco.Banco, envio *envioQueConta, regraID string, evento ganchos.NomeDoEvento) {
	t.Helper()
	if !envio.enviou.Load() || envio.linhasNoEnvio.Load() != 0 {
		t.Fatalf("enviou=%v com %d linhas já gravadas, want envio antes da auditoria", envio.enviou.Load(), envio.linhasNoEnvio.Load())
	}
	linhas := linhasDaRegra(t, db, regraID)
	if len(linhas) == 0 {
		t.Fatalf("decisão de %s não auditada", regraID)
	}
	for _, l := range linhas {
		if l.Evento != string(evento) {
			t.Errorf("linha de %s no evento %s, want %s", regraID, l.Evento, evento)
		}
	}
}

// O gancho de ferramenta roda dentro do harness: a decisão dele vai para o coletor do turno,
// não para uma gravação antes da resposta.
func TestAuditoriaDoGanchoDeFerramentaVaiNoLote(t *testing.T) {
	bloqueiaNaChamada := ganchos.Gancho[ganchos.AntesDaFerramenta]{
		Nome: "regra", Origem: ganchos.Origem{RegraID: "r-ferramenta", RegraVersao: 1},
		Avaliar: func(e ganchos.AntesDaFerramenta) (ganchos.AntesDaFerramenta, ganchos.Decisao) {
			if e.Previa {
				return e, ganchos.Decisao{}
			}
			return e, ganchos.Decisao{Tipo: ganchos.Bloquear, Motivo: "ferramenta fechada"}
		},
	}
	idx := &ganchos.Indice{AntesDaFerramenta: ganchos.Nivel[ganchos.AntesDaFerramenta]{Globais: []ganchos.Gancho[ganchos.AntesDaFerramenta]{bloqueiaNaChamada}}}
	svc, db, envio := servicoDoFluxoAuditado(t, idx)

	falar(t, svc, "auditoria-ferramenta", "quero liberar Ana Souza hoje")

	exigirAuditadaDepoisDoEnvio(t, db, envio, "r-ferramenta", ganchos.EventoAntesDaFerramenta)
}

// O atendente preenche o pedido e a resposta ao morador sai antes da auditoria dela.
func TestAuditoriaDoPreencherPedidoVaiDepoisDoEnvio(t *testing.T) {
	regra := &ganchostest.Roteiro[ganchos.AntesDeResponder]{}
	g := ganchostest.Gancho("regra", regra)
	g.Origem = ganchos.Origem{RegraID: "r-manual", RegraVersao: 1}
	svc, db, envio := servicoDoFluxoAuditado(t, &ganchos.Indice{
		AntesDeResponder: ganchos.Nivel[ganchos.AntesDeResponder]{Globais: []ganchos.Gancho[ganchos.AntesDeResponder]{g}},
	})
	sess := falar(t, svc, "auditoria-manual", "quero liberar Ana Souza hoje")
	// a regra só bloqueia depois do turno: o que se mede é o envio do atendente
	regra.Decisao = ganchos.Decisao{Tipo: ganchos.Bloquear, Motivo: "Resposta trocada pela regra."}
	cart, _, err := svc.Carteira(context.Background(), sess.contato())
	if err != nil || len(cart.Abertos()) != 1 {
		t.Fatalf("pedido aberto: %v %+v", err, cart)
	}

	if _, _, _, err := svc.PreencherPedido(context.Background(), sess.Protocolo, cart.Abertos()[0].Protocolo, "ana", nil, true); err != nil {
		t.Fatal(err)
	}

	exigirAuditadaDepoisDoEnvio(t, db, envio, "r-manual", ganchos.EventoAntesDeResponder)
	if l := linhasDaRegra(t, db, "r-manual"); len(l) != 1 || l[0].UsuarioID != "ana" || l[0].SessaoID != sess.ID {
		t.Errorf("linhas do pedido manual = %+v, want uma, do atendente ana na sessão", l)
	}
}
