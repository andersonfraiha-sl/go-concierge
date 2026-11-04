package atende

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"go-concierge/internal/admin"
	"go-concierge/internal/banco"
	"go-concierge/internal/banco/bancotest"
	"go-concierge/internal/entrada"
	"go-concierge/internal/gruvi"
	"go-concierge/internal/harness"
	"go-concierge/internal/memoria"
	"go-concierge/internal/okf"
	"go-concierge/internal/pedidos"
)

// idas conta as idas ao banco (uma query ou um lote inteiro) e o tempo gasto nelas.
type idas struct {
	mu     sync.Mutex
	n      int
	tempo  time.Duration
	inicio map[*pgx.Conn]time.Time
}

func (i *idas) comecar(conn *pgx.Conn) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.inicio[conn] = time.Now()
}

func (i *idas) terminar(conn *pgx.Conn) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.n++
	i.tempo += time.Since(i.inicio[conn])
	delete(i.inicio, conn)
}

func (i *idas) TraceQueryStart(ctx context.Context, conn *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	i.comecar(conn)
	return ctx
}

func (i *idas) TraceQueryEnd(_ context.Context, conn *pgx.Conn, _ pgx.TraceQueryEndData) {
	i.terminar(conn)
}

func (i *idas) TraceBatchStart(ctx context.Context, conn *pgx.Conn, _ pgx.TraceBatchStartData) context.Context {
	i.comecar(conn)
	return ctx
}

func (*idas) TraceBatchQuery(context.Context, *pgx.Conn, pgx.TraceBatchQueryData) {}

func (i *idas) TraceBatchEnd(_ context.Context, conn *pgx.Conn, _ pgx.TraceBatchEndData) {
	i.terminar(conn)
}

func (i *idas) zerar() {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.n, i.tempo = 0, 0
}

func (i *idas) ate() (int, time.Duration) {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.n, i.tempo
}

// marcaEnvio guarda as idas ao banco contadas até o Enviar da resposta.
type marcaEnvio struct {
	idas  *idas
	n     int
	tempo time.Duration
}

func (m *marcaEnvio) Enviar(context.Context, entrada.Destino, string) error {
	m.n, m.tempo = m.idas.ate()
	return nil
}

// servicoNoPostgres monta o serviço como em produção: sessões, pedidos, memória e mapeamento
// no Postgres, com os caches e os lotes do turno, e um contador de idas ao banco.
func servicoNoPostgres(tb testing.TB) (*Servico, banco.Banco, *idas) {
	tb.Helper()
	contador := &idas{inicio: map[*pgx.Conn]time.Time{}}
	db := bancotest.Novo(tb, func(cfg *pgxpool.Config) { cfg.ConnConfig.Tracer = contador })
	cat, err := okf.NewCatalogo(filepath.Join(raizDoRepo(tb), "okf"))
	if err != nil {
		tb.Fatal(err)
	}
	fix, err := okf.LerRecursosFixture(filepath.Join(cat.Root, "nexus", "cond-001.yaml"))
	if err != nil {
		tb.Fatal(err)
	}
	cat.Recursos = func() []okf.Recurso { return fix }
	adicionarToolsMCP(cat)
	svc := &Servico{
		Cat: cat, Agente: "agentes/concierge", Mapeamento: mapeamentoNoPostgres(tb, db), Sender: &marcaEnvio{idas: contador},
		Config:      harness.Config{Mock: true, Env: func(string) string { return "" }, MCP: &mcpServicoFalso{}},
		TelefoneB2C: "5519981621916", Gruvi: &gruvi.Client{CompanyID: "5982"},
		Pool: db.Pool,
	}
	esfriar(svc, db)
	return svc, db, contador
}

// esfriar troca os caches por contato e as sessões vivas por novos, vazios.
func esfriar(svc *Servico, db banco.Banco) {
	svc.Sessoes = NewSessoes(NovoSessaoPostgres(db))
	svc.Pedidos = pedidos.NovoCache(pedidos.NovoPostgres(db))
	svc.Memorias = memoria.NovoCache(memoria.NovoPostgres(db))
}

// mapeamentoNoPostgres grava no banco o mesmo mapeamento de mapeamentoDeTeste e devolve o
// cache do turno, já aquecido.
func mapeamentoNoPostgres(tb testing.TB, db banco.Banco) *admin.Cache {
	tb.Helper()
	m, err := mapeamentoDeTeste(tb).Ler(tb.Context())
	if err != nil {
		tb.Fatal(err)
	}
	repo := admin.NovoPostgres(db.Ent)
	for _, a := range m.Administradoras {
		if _, err := repo.SalvarAdministradora(tb.Context(), a); err != nil {
			tb.Fatal(err)
		}
	}
	for _, c := range m.Condominios {
		if _, err := repo.SalvarCondominio(tb.Context(), c); err != nil {
			tb.Fatal(err)
		}
	}
	cache := admin.NovoCache(repo)
	if _, err := cache.Ler(tb.Context()); err != nil {
		tb.Fatal(err)
	}
	return cache
}

func percentil(ordenado []time.Duration, p float64) time.Duration {
	return ordenado[int(float64(len(ordenado)-1)*p)]
}

const (
	textoTurno = "quero agendar minha mudança"
	casoTurno  = "agendar_mudanca"
)

// textoSemPedido é a pergunta do eventoExemplo: o concierge responde sem abrir pedido.
const textoSemPedido = "posso fazer barulho depois das 22h?"

func eventoDoTurno(tb testing.TB) Evento { return eventoCom(tb, textoTurno) }

func eventoCom(tb testing.TB, texto string) Evento {
	tb.Helper()
	ev, err := Parse([]byte(strings.Replace(eventoExemplo, textoSemPedido, texto, 1)))
	if err != nil {
		tb.Fatal(err)
	}
	return *ev
}

// mensagemDoMorador é a mensagem i da conversa: conversas diferentes, protocolos diferentes.
func mensagemDoMorador(base Evento, conversa, i int) entrada.Mensagem {
	ev := base
	ev.ID = fmt.Sprintf("evt-bench-%d-%d", conversa, i)
	ev.Data.Protocol.Number = fmt.Sprintf("protocolo-bench-%d", conversa)
	return ev.Normalizar()
}

// exigirAgendamento falha se o turno não fez o trabalho que se mede.
func exigirAgendamento(tb testing.TB, svc *Servico, db banco.Banco, msg entrada.Mensagem) {
	tb.Helper()
	sess, err := svc.Processar(context.Background(), msg)
	if err != nil {
		tb.Fatal(err)
	}
	svc.Aguardar()
	if ultimoConcierge(sess).Texto == "" {
		tb.Fatal("turno sem resposta do concierge")
	}
	carteira, err := pedidos.NovoPostgres(db).Ler(context.Background(), contatoDeTeste(tb, "adm-a"))
	if err != nil {
		tb.Fatal(err)
	}
	for _, p := range carteira.Abertos() {
		if p.Caso == casoTurno {
			return
		}
	}
	tb.Fatalf("pedido %s não gravado no banco: %+v", casoTurno, carteira.Pedidos)
}

// novaConversa zera, fora do cronômetro, sessões, pedidos e memória no banco e nos caches: o
// morador é o único identificado pelo token B2C, então a conversa nova vem de estado novo.
func novaConversa(tb testing.TB, svc *Servico, db banco.Banco) {
	tb.Helper()
	ctx := tb.Context()
	apagar := []func(context.Context) (int, error){
		db.Ent.Sessao.Delete().Exec,
		db.Ent.Pedido.Delete().Exec, // eventos vão junto (ON DELETE CASCADE)
		db.Ent.MemoriaFato.Delete().Exec,
		db.Ent.MemoriaContato.Delete().Exec,
	}
	for _, apagarTabela := range apagar {
		if _, err := apagarTabela(ctx); err != nil {
			tb.Fatal(err)
		}
	}
	esfriar(svc, db)
}

// medida de um turno: a latência inteira (Processar + Aguardar) e as idas ao banco até o
// Enviar da resposta.
type medida struct {
	turno, banco time.Duration
	idas         int
}

func medir(tb testing.TB, svc *Servico, contador *idas, msg entrada.Mensagem) medida {
	tb.Helper()
	contador.zerar()
	inicio := time.Now()
	if _, err := svc.Processar(context.Background(), msg); err != nil {
		tb.Fatal(err)
	}
	svc.Aguardar()
	turno := time.Since(inicio)
	envio := svc.Sender.(*marcaEnvio)
	return medida{turno: turno, banco: envio.tempo, idas: envio.n}
}

func relatar(b *testing.B, medidas []medida) {
	turnos := make([]time.Duration, 0, len(medidas))
	bancos := make([]time.Duration, 0, len(medidas))
	maxIdas := 0
	for _, m := range medidas {
		turnos, bancos = append(turnos, m.turno), append(bancos, m.banco)
		maxIdas = max(maxIdas, m.idas)
	}
	slices.Sort(turnos)
	slices.Sort(bancos)
	ms := func(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }
	b.ReportMetric(ms(percentil(turnos, 0.50)), "p50-ms")
	b.ReportMetric(ms(percentil(turnos, 0.95)), "p95-ms")
	b.ReportMetric(ms(percentil(bancos, 0.50)), "banco-antes-envio-p50-ms")
	b.ReportMetric(ms(percentil(bancos, 0.95)), "banco-antes-envio-p95-ms")
	b.ReportMetric(float64(maxIdas), "idas-antes-envio-max")
}

// BenchmarkTurnoMock: cada iteração é o primeiro turno de uma conversa nova de um contato
// frio (caches vazios), para que o histórico não cresça com -benchtime. Mesma carga do
// "Antes" do documento de medição.
func BenchmarkTurnoMock(b *testing.B) {
	svc, db, contador := servicoNoPostgres(b)
	ev := eventoDoTurno(b)
	exigirAgendamento(b, svc, db, mensagemDoMorador(ev, 0, 0))
	medidas := make([]medida, 0, b.N)
	b.ResetTimer()
	for i := 1; i <= b.N; i++ {
		b.StopTimer()
		novaConversa(b, svc, db)
		msg := mensagemDoMorador(ev, i, 0)
		b.StartTimer()
		medidas = append(medidas, medir(b, svc, contador, msg))
	}
	b.StopTimer()
	relatar(b, medidas)
}

// BenchmarkTurnoMockQuente: o segundo turno de uma conversa já aberta, com o contato quente;
// o primeiro turno roda fora do cronômetro.
func BenchmarkTurnoMockQuente(b *testing.B) {
	svc, db, contador := servicoNoPostgres(b)
	ev := eventoDoTurno(b)
	exigirAgendamento(b, svc, db, mensagemDoMorador(ev, 0, 0))
	medidas := make([]medida, 0, b.N)
	b.ResetTimer()
	for i := 1; i <= b.N; i++ {
		b.StopTimer()
		novaConversa(b, svc, db)
		if _, err := svc.Processar(context.Background(), mensagemDoMorador(ev, i, 0)); err != nil {
			b.Fatal(err)
		}
		svc.Aguardar()
		msg := mensagemDoMorador(ev, i, 1)
		b.StartTimer()
		medidas = append(medidas, medir(b, svc, contador, msg))
	}
	b.StopTimer()
	relatar(b, medidas)
}

// BenchmarkTurnoMockQuenteNovoPedido: o segundo turno da conversa, com o contato quente,
// abre o pedido (a gravação vai antes da resposta); o primeiro turno, uma pergunta sem
// pedido, roda fora do cronômetro.
func BenchmarkTurnoMockQuenteNovoPedido(b *testing.B) {
	svc, db, contador := servicoNoPostgres(b)
	pergunta, agendamento := eventoCom(b, textoSemPedido), eventoDoTurno(b)
	exigirAgendamento(b, svc, db, mensagemDoMorador(agendamento, 0, 0))
	medidas := make([]medida, 0, b.N)
	b.ResetTimer()
	for i := 1; i <= b.N; i++ {
		b.StopTimer()
		novaConversa(b, svc, db)
		if _, err := svc.Processar(context.Background(), mensagemDoMorador(pergunta, i, 0)); err != nil {
			b.Fatal(err)
		}
		svc.Aguardar()
		msg := mensagemDoMorador(agendamento, i, 1)
		b.StartTimer()
		medidas = append(medidas, medir(b, svc, contador, msg))
	}
	b.StopTimer()
	relatar(b, medidas)
}

// BenchmarkIdaAoBanco mede o piso de uma ida ao banco neste ambiente: SELECT 1.
func BenchmarkIdaAoBanco(b *testing.B) {
	db := bancotest.Novo(b)
	var um int
	if err := db.Pool.QueryRow(b.Context(), "SELECT 1").Scan(&um); err != nil {
		b.Fatal(err)
	}
	latencias := make([]time.Duration, 0, b.N)
	b.ResetTimer()
	for range b.N {
		inicio := time.Now()
		if err := db.Pool.QueryRow(b.Context(), "SELECT 1").Scan(&um); err != nil {
			b.Fatal(err)
		}
		latencias = append(latencias, time.Since(inicio))
	}
	b.StopTimer()
	slices.Sort(latencias)
	b.ReportMetric(float64(percentil(latencias, 0.50).Microseconds())/1000, "p50-ms")
	b.ReportMetric(float64(percentil(latencias, 0.95).Microseconds())/1000, "p95-ms")
}

// Antes da resposta: o turno de um contato frio vai ao banco uma vez (o lote que carrega
// sessão, memória e pedidos); um turno que abre pedido grava a carteira antes da resposta,
// uma ida; um turno sem mudança de pedido com o contato quente não vai ao banco. A sessão
// chega ao banco depois da resposta.
func TestTurnoNoPostgresIdasAntesDaResposta(t *testing.T) {
	svc, db, contador := servicoNoPostgres(t)
	pergunta, agendamento := eventoCom(t, textoSemPedido), eventoDoTurno(t)
	tests := []struct {
		nome string
		msg  entrada.Mensagem
		idas int
	}{
		{"contato frio, sem pedido", mensagemDoMorador(pergunta, 1, 0), 1},
		{"contato quente, pedido novo", mensagemDoMorador(agendamento, 1, 1), 1},
		{"contato quente, pedido sem mudança", mensagemDoMorador(agendamento, 1, 2), 0},
	}
	for _, tt := range tests {
		if got := medir(t, svc, contador, tt.msg).idas; got != tt.idas {
			t.Fatalf("%s: %d idas antes da resposta, want %d", tt.nome, got, tt.idas)
		}
	}
	exigirAgendamento(t, svc, db, mensagemDoMorador(agendamento, 1, 3))
	roteada := normalizarRoteamento(mensagemDoMorador(agendamento, 1, 3))
	if _, achou, err := NovoSessaoPostgres(db).Carregar(t.Context(), idSessao(roteada.Origem, roteada.ConversaID)); err != nil || !achou {
		t.Fatalf("sessão não gravada depois da resposta: %v", err)
	}
}
