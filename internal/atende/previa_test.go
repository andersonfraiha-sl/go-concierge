package atende

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"go-concierge/internal/entrada"
	"go-concierge/internal/ganchos"
	"go-concierge/internal/ganchos/fixos"
	"go-concierge/internal/ganchos/ganchostest"
	"go-concierge/internal/ganchos/regras"
	"go-concierge/internal/pedidos"
)

const regraQueBloqueiaReserva = `{"nome":"Reserva fechada","evento":"antes_da_ferramenta",
"alvo":{"administradora":"adm-a","condominios":["cond-001"]},
"condicoes":[{"tipo":"ferramenta","nomes":["areas.reservar"]}],
"decisao":{"tipo":"bloquear","mensagem":"Reservas fechadas por aqui."},"ativa":true}`

func indiceDaRegraDeReserva(t *testing.T) *ganchos.Indice {
	t.Helper()
	indice, erros := regras.Compilar([]regras.RegraGravada{{ID: "r1", Versao: 1, Ativa: true, Definicao: []byte(regraQueBloqueiaReserva)}})
	if len(erros) > 0 {
		t.Fatal(erros)
	}
	return indice
}

func servicoComRegraDeReserva(t *testing.T) *Servico {
	t.Helper()
	svc, _, _ := servicoComPedidos(t)
	svc.Ganchos = ganchostest.Conjunto(fixos.Todos(), indiceDaRegraDeReserva(t))
	return svc
}

func TestPreviaCancelaAReservaAntesDaPrimeiraPergunta(t *testing.T) {
	svc := servicoComRegraDeReserva(t)
	sess := falar(t, svc, "previa-1", "quero reservar uma área comum")
	r := ultimoConcierge(sess)
	if !strings.Contains(r.Texto, "Reservas fechadas por aqui.") || strings.Contains(r.Texto, "Qual área") {
		t.Fatalf("devia cancelar com a mensagem da regra, sem perguntar a área: %q", r.Texto)
	}
	if len(r.Pedidos) == 0 || r.Pedidos[len(r.Pedidos)-1].Estado != pedidos.Cancelado {
		t.Fatalf("o pedido devia ficar cancelado: %+v", r.Pedidos)
	}
}

// A prévia que cancela o pedido vira uma linha de auditoria, gravada depois da resposta: o
// turno com o auditor vai ao banco antes da resposta tantas vezes quanto o turno sem auditor.
func TestPreviaQueCancelaAuditaUmaLinhaDepoisDaResposta(t *testing.T) {
	msg := mensagemDoMorador(eventoCom(t, "quero reservar uma área comum"), 1, 0)
	base, _, contadorBase := servicoNoPostgres(t) // o conjunto é fixado no primeiro uso: um serviço por medida
	base.Ganchos, base.Auditoria = ganchostest.Conjunto(fixos.Todos(), indiceDaRegraDeReserva(t)), nil
	semAuditor := medir(t, base, contadorBase, msg)
	svc, db, contador := servicoNoPostgres(t)
	comAuditor(svc, db, indiceDaRegraDeReserva(t))

	m := medir(t, svc, contador, msg)

	sess, ok, err := svc.Sessao(t.Context(), msg.ConversaID)
	if err != nil || !ok {
		t.Fatalf("sessão do turno: %v", err)
	}
	if r := ultimoConcierge(sess); !strings.Contains(r.Texto, "Reservas fechadas por aqui.") {
		t.Fatalf("a prévia devia ter bloqueado: %q", r.Texto)
	}
	if m.idas != semAuditor.idas {
		t.Errorf("idas antes da resposta = %d, want %d (as do turno sem auditor)", m.idas, semAuditor.idas)
	}
	linhas := linhasDaRegra(t, db, "r1")
	if len(linhas) != 1 || linhas[0].Decisao != string(ganchos.Bloquear) || linhas[0].CondominioID != "cond-001" {
		t.Fatalf("o bloqueio que cancelou devia ter uma linha: %+v", linhas)
	}
}

func TestPreviaSemRegraNaoMudaOFluxo(t *testing.T) {
	svc, _, _ := servicoComPedidos(t)
	sess := falar(t, svc, "previa-3", "quero reservar uma área comum")
	if r := ultimoConcierge(sess); !strings.Contains(r.Texto, "Qual área você quer reservar?") {
		t.Fatalf("sem regra o fluxo pergunta a área: %q", r.Texto)
	}
}

const regraDeReservaAte22h = `{"nome":"Reserva só até 22h","evento":"antes_da_ferramenta",
"alvo":{"administradora":"adm-a","condominios":["cond-001"]},
"condicoes":[{"tipo":"ferramenta","nomes":["areas.reservar"]},
{"tipo":"fora_do_horario","inicio":"06:00","fim":"22:00","fonte":{"tipo":"agora"}}],
"decisao":{"tipo":"bloquear","mensagem":"Reservas só das 6h às 22h."},"ativa":true}`

func servicoComRegraAte22h(t *testing.T, agora string) *Servico {
	t.Helper()
	indice, erros := regras.Compilar([]regras.RegraGravada{{ID: "r22", Versao: 1, Ativa: true, Definicao: []byte(regraDeReservaAte22h)}})
	if len(erros) > 0 {
		t.Fatal(erros)
	}
	instante, err := time.Parse(time.RFC3339, agora)
	if err != nil {
		t.Fatal(err)
	}
	svc, _, _ := servicoComPedidos(t)
	svc.Ganchos = ganchostest.Conjunto(fixos.Todos(), indice)
	svc.Agora = func() time.Time { return instante }
	return svc
}

func TestPreviaDaRegraDeHorarioBloqueiaAs22h30(t *testing.T) {
	svc := servicoComRegraAte22h(t, "2026-11-05T01:30:00Z") // 22h30 em São Paulo
	r := ultimoConcierge(falar(t, svc, "previa-22h30", "quero reservar uma área comum"))
	if !strings.Contains(r.Texto, "Reservas só das 6h às 22h.") || strings.Contains(r.Texto, "Qual área") {
		t.Fatalf("às 22h30 devia cancelar antes da primeira pergunta: %q", r.Texto)
	}
}

func TestPreviaDaRegraDeHorarioDeixaPassarAs21h(t *testing.T) {
	svc := servicoComRegraAte22h(t, "2026-11-05T00:00:00Z") // 21h00 em São Paulo
	r := ultimoConcierge(falar(t, svc, "previa-21h", "quero reservar uma área comum"))
	if !strings.Contains(r.Texto, "Qual área você quer reservar?") {
		t.Fatalf("às 21h a pergunta sai: %q", r.Texto)
	}
}

func TestPreviaComGanchoQueFalhaNaoCancela(t *testing.T) {
	quebrado := ganchos.Gancho[ganchos.AntesDaFerramenta]{
		Nome: "regra", Falha: ganchos.FechadaSeGrava, Origem: ganchos.Origem{RegraID: "r-quebrada"},
		Avaliar: func(ganchos.AntesDaFerramenta) (ganchos.AntesDaFerramenta, ganchos.Decisao) { panic("regra quebrada") },
	}
	idx := &ganchos.Indice{AntesDaFerramenta: ganchos.Nivel[ganchos.AntesDaFerramenta]{Globais: []ganchos.Gancho[ganchos.AntesDaFerramenta]{quebrado}}}
	svc, _, _ := servicoComPedidos(t)
	svc.Ganchos = ganchostest.Conjunto(fixos.Todos(), idx)
	r := ultimoConcierge(falar(t, svc, "previa-falha", "quero reservar uma área comum"))
	if !strings.Contains(r.Texto, "Qual área você quer reservar?") {
		t.Fatalf("falha na previsão não cancela o pedido: %q", r.Texto)
	}
}

const regraQueConfirmaReserva = `{"nome":"Confirmar reserva à noite","evento":"antes_da_ferramenta",
"alvo":{"administradora":"adm-a","condominios":["cond-001"]},
"condicoes":[{"tipo":"ferramenta","nomes":["areas.reservar"]},
{"tipo":"fora_do_horario","inicio":"06:00","fim":"22:00","fonte":{"tipo":"agora"}}],
"decisao":{"tipo":"confirmar","pergunta":"Já é tarde, quer mesmo reservar assim?"},"ativa":true}`

const perguntaDaRegraDeReserva = "Já é tarde, quer mesmo reservar assim?"

func indiceDaRegraQueConfirma(t *testing.T) *ganchos.Indice {
	t.Helper()
	indice, erros := regras.Compilar([]regras.RegraGravada{{ID: "rc", Versao: 2, Ativa: true, Definicao: []byte(regraQueConfirmaReserva)}})
	if len(erros) > 0 {
		t.Fatal(erros)
	}
	return indice
}

// conversaDaReserva fala com o serviço numa conversa só e devolve a resposta de cada fala.
func conversaDaReserva(t *testing.T, svc *Servico) func(texto string) string {
	t.Helper()
	pedidos.Agora = func() time.Time { return time.Date(2026, 11, 4, 12, 0, 0, 0, time.Local) }
	t.Cleanup(func() { pedidos.Agora = time.Now })
	instante := time.Date(2026, 11, 5, 1, 30, 0, 0, time.UTC) // 22h30 em São Paulo
	svc.Agora = func() time.Time { return instante }
	svc.Config.Agora = svc.Agora // o relógio dos eventos do harness
	i := 0
	return func(texto string) string {
		t.Helper()
		i++
		sess, err := svc.Processar(t.Context(), mensagemDoMorador(eventoCom(t, texto), 7, i))
		if err != nil {
			t.Fatalf("%q: %v", texto, err)
		}
		svc.Aguardar()
		return ultimoConcierge(sess).Texto
	}
}

// ateAPerguntaDaRegra leva a reserva até o sim do caso: a escrita roda e a regra, que ainda
// não teve a pergunta dela feita, segura a escrita e pergunta.
func ateAPerguntaDaRegra(t *testing.T, fala func(string) string) {
	t.Helper()
	ateAConfirmacaoDoCaso(t, fala)
	r := fala("sim")
	if !strings.Contains(r, perguntaDaRegraDeReserva) || strings.Contains(r, "Não consegui concluir") || strings.Contains(r, "Reserva confirmada") {
		t.Fatalf("o sim do caso leva à pergunta da regra, sem falha: %q", r)
	}
}

// ateAConfirmacaoDoCaso leva a reserva até a confirmação do caso, sem a pergunta da regra.
func ateAConfirmacaoDoCaso(t *testing.T, fala func(string) string) {
	t.Helper()
	if r := fala("quero reservar uma área comum"); !strings.Contains(r, "Qual área") {
		t.Fatalf("a regra não pergunta antes da escrita: %q", r)
	}
	if r := fala("salão"); !strings.Contains(r, "Para qual data?") {
		t.Fatalf("depois da área, a data: %q", r)
	}
	if r := fala("15/11"); !strings.Contains(r, "Confirma a reserva de Salão de festas") || strings.Contains(r, perguntaDaRegraDeReserva) {
		t.Fatalf("primeiro só a confirmação do caso: %q", r)
	}
}

func TestPerguntaDaRegraSaiDepoisDoSimDoCasoEOProximoSimGrava(t *testing.T) {
	svc, db, _ := servicoNoPostgres(t)
	comAuditor(svc, db, indiceDaRegraQueConfirma(t))
	fala := conversaDaReserva(t, svc)

	ateAPerguntaDaRegra(t, fala)
	linhas := linhasDaRegra(t, db, "rc")
	if len(linhas) != 1 || linhas[0].Decisao != string(ganchos.Confirmar) || linhas[0].RegraVersao != 2 {
		t.Fatalf("a pergunta da escrita real tem uma linha; as prévias, nenhuma: %+v", linhas)
	}
	if r := fala("sim"); !strings.Contains(r, "Reserva confirmada") {
		t.Fatalf("o sim à pergunta da regra deixa a escrita passar: %q", r)
	}
	if n := len(linhasDaRegra(t, db, "rc")); n != 1 {
		t.Fatalf("a escrita que passou não ganha linha: %d linhas", n)
	}
}

func TestNaoAPerguntaDaRegraCancelaOPedido(t *testing.T) {
	svc, _, _ := servicoComPedidos(t)
	svc.Ganchos = ganchostest.Conjunto(fixos.Todos(), indiceDaRegraQueConfirma(t))
	fala := conversaDaReserva(t, svc)

	ateAPerguntaDaRegra(t, fala)
	if r := fala("não"); !strings.Contains(r, "cancelado") || strings.Contains(r, "Reserva confirmada") {
		t.Fatalf("o não cancela como o não da confirmação do caso: %q", r)
	}
}

func TestSimQueNaoVemDepoisDaPerguntaNaoRespondeARegra(t *testing.T) {
	sess := &Sessao{PerguntasDeRegra: []string{perguntaDaRegraDeReserva}}
	if got := sess.tomarPerguntasConfirmadas("quero outra data"); len(got) != 0 || len(sess.PerguntasDeRegra) != 0 {
		t.Fatalf("mensagem que não confirma não responde e esquece a pergunta: %v %v", got, sess.PerguntasDeRegra)
	}
	sess.PerguntasDeRegra = []string{perguntaDaRegraDeReserva}
	if got := sess.tomarPerguntasConfirmadas("sim"); len(got) != 1 || len(sess.PerguntasDeRegra) != 0 {
		t.Fatalf("o sim responde a pergunta da resposta anterior, uma vez só: %v %v", got, sess.PerguntasDeRegra)
	}
	if got := sess.tomarPerguntasConfirmadas("sim"); len(got) != 0 {
		t.Fatalf("um segundo sim já não tem pergunta a responder: %v", got)
	}
}

// sessaoDaReserva é a sessão da conversa de conversaDaReserva.
func sessaoDaReserva(t *testing.T, svc *Servico) *Sessao {
	t.Helper()
	sess, ok, err := svc.Sessao(t.Context(), mensagemDoMorador(eventoCom(t, "x"), 7, 0).ConversaID)
	if err != nil || !ok {
		t.Fatalf("sessão da conversa: %v", err)
	}
	return sess
}

func perguntasGuardadas(sess *Sessao) []string {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	return slices.Clone(sess.PerguntasDeRegra)
}

func TestPerguntaDaRegraFicaGuardadaDepoisDaRespostaEnviada(t *testing.T) {
	svc, _, _ := servicoComPedidos(t)
	svc.Ganchos = ganchostest.Conjunto(fixos.Todos(), indiceDaRegraQueConfirma(t))
	ateAPerguntaDaRegra(t, conversaDaReserva(t, svc))
	if got := perguntasGuardadas(sessaoDaReserva(t, svc)); !slices.Equal(got, []string{perguntaDaRegraDeReserva}) {
		t.Fatalf("a pergunta que saiu fica para o próximo sim: %v", got)
	}
}

// envioQueFalha devolve o erro dado em todo envio.
type envioQueFalha struct{ err error }

func (e envioQueFalha) Enviar(context.Context, entrada.Destino, string) error { return e.err }

func TestTurnoSemRespostaEnviadaNaoGuardaAPergunta(t *testing.T) {
	casos := []struct {
		nome string
		err  error
	}{
		{"envio que falha", errors.New("canal fora do ar")},
		{"turno cancelado na entrega", context.Canceled},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			svc, _, _ := servicoComPedidos(t)
			svc.Ganchos = ganchostest.Conjunto(fixos.Todos(), indiceDaRegraQueConfirma(t))
			fala := conversaDaReserva(t, svc)
			ateAConfirmacaoDoCaso(t, fala)
			svc.Sender = envioQueFalha{err: c.err}

			_, _ = svc.Processar(t.Context(), mensagemDoMorador(eventoCom(t, "sim"), 7, 99))
			svc.Aguardar()

			if got := perguntasGuardadas(sessaoDaReserva(t, svc)); len(got) != 0 {
				t.Fatalf("pergunta que não chegou ao morador não pode ser respondida: %v", got)
			}
		})
	}
}

func TestPerguntasDeRegraSomemQuandoAConversaMuda(t *testing.T) {
	casos := []struct {
		nome  string
		mudar func(svc *Servico, sess *Sessao)
	}{
		{"atendente fala", func(_ *Servico, sess *Sessao) {
			sess.registrarHumano(entrada.Mensagem{ID: "evt-atendente", Autor: "atendente", Texto: "sigo com a reserva?"})
		}},
		{"protocolo encerra", func(_ *Servico, sess *Sessao) { sess.encerrar(entrada.Mensagem{ID: "evt-fim"}) }},
		{"transbordo", func(svc *Servico, sess *Sessao) { svc.marcarTransbordo(sess, "pediu atendente", "morador", "") }},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			svc, _, _ := servicoComPedidos(t)
			sess := &Sessao{PerguntasDeRegra: []string{perguntaDaRegraDeReserva}}
			c.mudar(svc, sess)
			if got := perguntasGuardadas(sess); len(got) != 0 {
				t.Fatalf("a pergunta não pode valer para o sim que vem depois: %v", got)
			}
		})
	}
}

func TestPerguntasDeRegraSobrevivemAGravacaoDaSessao(t *testing.T) {
	sess, err := NewSessoes(NovaSessaoEmMemoria()).Abrir(t.Context(), entrada.Mensagem{ConversaID: "P-10", Tenant: "t", Contato: entrada.Contato{Phone: "5511999"}})
	if err != nil {
		t.Fatal(err)
	}
	sess.PerguntasDeRegra = []string{perguntaDaRegraDeReserva}
	r, err := sess.Registro()
	if err != nil {
		t.Fatal(err)
	}
	restaurada, err := RestaurarSessao(r)
	if err != nil {
		t.Fatal(err)
	}
	if got := restaurada.tomarPerguntasConfirmadas("sim"); !slices.Equal(got, []string{perguntaDaRegraDeReserva}) {
		t.Fatalf("o sim depois de um restart responde a pergunta gravada: %v", got)
	}
}
