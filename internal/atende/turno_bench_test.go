package atende

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"go-concierge/internal/entrada"
	"go-concierge/internal/gruvi"
	"go-concierge/internal/harness"
	"go-concierge/internal/memoria"
	"go-concierge/internal/okf"
	"go-concierge/internal/pedidos"
)

// servicoParaBenchmark isola a montagem dos stores: a Task 13 troca só esta função
// quando a persistência passar para Postgres.
func servicoParaBenchmark(b *testing.B) *Servico {
	b.Helper()
	cat, err := okf.NewCatalogo(filepath.Join(raizDoRepo(b), "okf"))
	if err != nil {
		b.Fatal(err)
	}
	fix, err := okf.LerRecursosFixture(filepath.Join(cat.Root, "nexus", "cond-001.yaml"))
	if err != nil {
		b.Fatal(err)
	}
	cat.Recursos = func() []okf.Recurso { return fix }
	adicionarToolsMCP(cat)
	return &Servico{
		Cat: cat, Agente: "agentes/concierge", Account: "cond-001", Sessoes: NewSessoes(), Sender: &Outbox{},
		Config:      harness.Config{Mock: true, Env: func(string) string { return "" }, MCP: &mcpServicoFalso{}},
		TelefoneB2C: "5519981621916", Gruvi: &gruvi.Client{CompanyID: "5982"},
		Pedidos:  pedidos.NewArquivos(b.TempDir()),
		Memorias: memoria.NewArquivos(b.TempDir()),
	}
}

func percentil(ordenado []time.Duration, p float64) time.Duration {
	return ordenado[int(float64(len(ordenado)-1)*p)]
}

const (
	textoTurno = "quero agendar minha mudança"
	casoTurno  = "agendar_mudanca"
)

// mensagemDoMorador é o primeiro turno de uma conversa nova do morador.
func mensagemDoMorador(base Evento, i int) entrada.Mensagem {
	ev := base
	ev.ID = fmt.Sprintf("evt-bench-%d", i)
	ev.Data.Protocol.Number = fmt.Sprintf("protocolo-bench-%d", i)
	return ev.Normalizar()
}

// exigirAgendamento falha o benchmark se o turno não fez o trabalho que mede.
func exigirAgendamento(b *testing.B, svc *Servico, msg entrada.Mensagem) {
	b.Helper()
	sess, err := svc.Processar(context.Background(), msg)
	if err != nil {
		b.Fatal(err)
	}
	svc.Aguardar()
	if ultimoConcierge(sess).Texto == "" {
		b.Fatal("turno sem resposta do concierge")
	}
	carteira, err := svc.Pedidos.Ler(context.Background(), svc.Account, msg.Contato.Phone)
	if err != nil {
		b.Fatal(err)
	}
	for _, p := range carteira.Abertos() {
		if p.Caso == casoTurno {
			return
		}
	}
	b.Fatalf("pedido %s não aberto: %+v", casoTurno, carteira.Pedidos)
}

// novaConversa zera sessões e stores (fora do cronômetro): o morador é o único
// identificado pelo token B2C, então a conversa nova vem de estado novo, não de telefone novo.
func novaConversa(b *testing.B, svc *Servico) {
	b.Helper()
	svc.Sessoes = NewSessoes()
	svc.Pedidos = pedidos.NewArquivos(b.TempDir())
	svc.Memorias = memoria.NewArquivos(b.TempDir())
}

// Cada iteração é o primeiro turno de uma conversa nova, para que o histórico não
// cresça com -benchtime.
func BenchmarkTurnoMock(b *testing.B) {
	svc := servicoParaBenchmark(b)
	ev, err := Parse([]byte(strings.Replace(eventoExemplo, "posso fazer barulho depois das 22h?", textoTurno, 1)))
	if err != nil {
		b.Fatal(err)
	}
	exigirAgendamento(b, svc, mensagemDoMorador(*ev, 0))
	latencias := make([]time.Duration, 0, b.N)
	b.ResetTimer()
	for i := 1; i <= b.N; i++ {
		b.StopTimer()
		novaConversa(b, svc)
		msg := mensagemDoMorador(*ev, i)
		b.StartTimer()
		inicio := time.Now()
		if _, err := svc.Processar(context.Background(), msg); err != nil {
			b.Fatal(err)
		}
		svc.Aguardar()
		latencias = append(latencias, time.Since(inicio))
	}
	b.StopTimer()
	slices.Sort(latencias)
	b.ReportMetric(float64(percentil(latencias, 0.50).Microseconds())/1000, "p50-ms")
	b.ReportMetric(float64(percentil(latencias, 0.95).Microseconds())/1000, "p95-ms")
}
