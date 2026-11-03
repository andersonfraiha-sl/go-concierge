package atende

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

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
	cat, err := okf.NewCatalogo(filepath.Join(raizDoRepo(nil), "okf"))
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

func BenchmarkTurnoMock(b *testing.B) {
	svc := servicoParaBenchmark(b)
	ctx := context.Background()
	latencias := make([]time.Duration, 0, b.N)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ev, err := Parse([]byte(strings.Replace(eventoExemplo, "posso fazer barulho depois das 22h?", "quero agendar minha mudança", 1)))
		if err != nil {
			b.Fatal(err)
		}
		ev.ID = fmt.Sprintf("evt-bench-%d", i)
		msg := ev.Normalizar()
		inicio := time.Now()
		if _, err := svc.Processar(ctx, msg); err != nil {
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
