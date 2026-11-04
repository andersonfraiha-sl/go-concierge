package regras

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"runtime/debug"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go-concierge/internal/ganchos"
)

const (
	limiteMemoriaDezMilRegras = 20 << 20
	p99MaximoMicros           = 50
)

func definicaoDeCarga(i int, administradora, condominio string) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(`{
		"nome":"regra de carga %05d","evento":"antes_da_ferramenta",
		"alvo":{"administradora":%q,"condominios":[%q]},
		"condicoes":[
			{"tipo":"ferramenta","nomes":["areas.reservar"]},
			{"tipo":"fora_do_horario","inicio":"06:00","fim":"22:00","fonte":{"tipo":"agora"}}],
		"decisao":{"tipo":"confirmar","pergunta":"Já é tarde, quer mesmo reservar assim?"},
		"ativa":true}`, i, administradora, condominio))
}

func regrasDeCarga(n, condominios int) []RegraGravada {
	rs := make([]RegraGravada, n)
	for i := range rs {
		adm := fmt.Sprintf("adm-%d", i%20)
		cond := fmt.Sprintf("cond-%d", i%condominios)
		rs[i] = RegraGravada{ID: fmt.Sprintf("r%05d", i), Versao: 1, Ativa: true, Definicao: definicaoDeCarga(i, adm, cond)}
	}
	return rs
}

func heapEmUso() uint64 {
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapAlloc
}

func TestIndiceDezMilRegras(t *testing.T) {
	rs := regrasDeCarga(10_000, 500)
	antes := heapEmUso()

	indice, erros := Compilar(rs)

	depois := heapEmUso()
	require.Empty(t, erros)
	require.Equal(t, 10_000, indice.Ativas[ganchos.EventoAntesDaFerramenta])
	aumento := int64(depois) - int64(antes)
	t.Logf("aumento do heap com 10 mil regras compiladas: %.1f MB", float64(aumento)/(1<<20))
	require.LessOrEqual(t, aumento, int64(limiteMemoriaDezMilRegras))
	runtime.KeepAlive(rs)
	runtime.KeepAlive(indice)
}

// BenchmarkGanchos mede AntesDaFerramenta com 20 regras ativas no condomínio, todas
// avaliadas por inteiro (nenhuma dispara). Reporta o p99 por chamada, em µs.
func BenchmarkGanchos(b *testing.B) {
	rs := regrasDeCarga(20, 1)
	for i := range rs { // mesmo condomínio e mesma administradora para as 20
		rs[i].Definicao = definicaoDeCarga(i, "adm-0", "cond-0")
	}
	indice, erros := Compilar(rs)
	require.Empty(b, erros)
	conjunto := ganchos.NovoConjunto(ganchos.Fixos{}, func() *ganchos.Indice { return indice })
	meioDia := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC) // 09h em São Paulo: dentro do horário
	ev := ganchos.AntesDaFerramenta{
		Agora: meioDia, Ferramenta: "areas.reservar", AdministradoraID: "adm-0", CondominioID: "cond-0",
	}
	ctx := context.Background()
	defer debug.SetGCPercent(debug.SetGCPercent(-1))
	duracoes := make([]time.Duration, 0, b.N)

	b.ResetTimer()
	for range b.N {
		inicio := time.Now()
		turno := conjunto.Turno()
		turno.AntesDaFerramenta(ctx, ev)
		duracoes = append(duracoes, time.Since(inicio))
		if len(duracoes)%4096 == 0 {
			runtime.GC() // fora da medição: o p99 mede a avaliação, não a pausa do coletor
		}
	}
	b.StopTimer()

	slices.Sort(duracoes)
	p99 := duracoes[len(duracoes)*99/100]
	b.ReportMetric(float64(p99.Microseconds()), "p99-µs")
	if relogioMedeMicros() && b.N >= 1000 && p99 > p99MaximoMicros*time.Microsecond { // p99 de poucas amostras não diz nada
		b.Errorf("p99 de %v passa de %d µs", p99, p99MaximoMicros)
	}
}

// relogioMedeMicros diz se o relógio mede microssegundos: no Windows ele anda em
// passos de 500 µs e o p99 não significa nada, então lá o benchmark só reporta.
func relogioMedeMicros() bool { return runtime.GOOS != "windows" }
