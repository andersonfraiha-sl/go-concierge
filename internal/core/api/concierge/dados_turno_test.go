package conciergeapi

import (
	"context"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/metric/noop"

	"go-concierge/internal/core/administracao/regras"
	"go-concierge/internal/core/banco/bancotest"
	okf "go-concierge/internal/core/catalogo/arquivos"
	"go-concierge/internal/core/dados"
	adminpg "go-concierge/internal/core/dados/postgres/admin"
	canaispg "go-concierge/internal/core/dados/postgres/canais"
	sessoespg "go-concierge/internal/core/dados/postgres/sessoes"
	"go-concierge/internal/entrada"
	"go-concierge/internal/escopo"
	harness "go-concierge/internal/harness/agentes"
	atende "go-concierge/internal/harness/atendimento"
	"go-concierge/internal/harness/ganchos"
	"go-concierge/internal/harness/ganchos/auditoria"
	"go-concierge/internal/harness/pedidos"
	"go-concierge/internal/identidade"
)

type contarGravacaoHTTP struct {
	gravador atende.GravacaoTurno
	chamadas atomic.Int32
}

type contarAuditoriaHTTP struct {
	dados.OperacoesAuditoria
	chamadas atomic.Int32
}

func (a *contarAuditoriaHTTP) Gravar(ctx context.Context, empresa string, canal uuid.UUID, linhas []auditoria.Linha) error {
	a.chamadas.Add(1)
	return a.OperacoesAuditoria.Gravar(ctx, empresa, canal, linhas)
}

func TestAuditorDoMainUsaHTTPNoRuntime(t *testing.T) {
	b := bancotest.Novo(t)
	channel := uuid.New()
	if _, err := b.Pool.Exec(t.Context(), `INSERT INTO channels(id,company_id,condo_id,channel_type,provider,endpoint_identifier,feature_mode,enabled,root_agent,updated_at) VALUES($1,'empresa',101,'WEB','test',$2,'INHERIT',true,'root',now())`, channel, channel.String()); err != nil {
		t.Fatal(err)
	}
	pg := dados.NovaAuditoriaPostgres(b)
	contagem := &contarAuditoriaHTTP{OperacoesAuditoria: pg}
	regrasPG := regras.NovoPostgres(b.Ent)
	server, cliente, _, err := dados.IniciarConfigurado(dados.Config{Turno: dados.NovoGravadorPostgres(b), Canais: canaispg.NovoPostgres(b.Pool), Sessoes: sessoespg.NovoSessaoPostgres(b), Memorias: dados.NovasMemoriasPostgres(b), Pedidos: dados.NovosPedidosPostgres(b), Auditoria: contagem, Regras: regrasPG})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Fechar(context.Background()) })
	s := &servidor{banco: b, regras: regrasPG}
	_, auditor, err := s.montarGanchos(t.Context(), noop.NewMeterProvider().Meter("ganchos"), auditoriaTransporte{cliente: cliente, store: pg})
	if err != nil {
		t.Fatal(err)
	}
	linha := auditoria.Linha{CriadoEm: time.Now(), Origem: ganchos.OrigemConcierge, Evento: "mensagem_recebida", Gancho: "fixo", Decisao: "bloquear", AdministradoraID: "empresa", CondominioID: "101", ChannelID: channel.String()}
	auditor.Gravar(atende.ComEscopoSessao(t.Context(), "empresa", channel), []auditoria.Linha{linha})
	if contagem.chamadas.Load() != 1 {
		t.Fatalf("gravações HTTP do runtime = %d", contagem.chamadas.Load())
	}
	auditor.Gravar(identidade.ComUsuario(t.Context(), "gestor"), []auditoria.Linha{linha})
	var n int
	if err := b.Pool.QueryRow(t.Context(), `SELECT count(*) FROM hook_audit_logs`).Scan(&n); err != nil || n != 1 || contagem.chamadas.Load() != 1 {
		t.Fatalf("linhas=%d chamadas HTTP=%d erro=%v", n, contagem.chamadas.Load(), err)
	}
}

func (g *contarGravacaoHTTP) Gravar(ctx context.Context, r atende.RegistroSessao, c escopo.Contato, cart pedidos.Carteira, linhas []auditoria.Linha) error {
	g.chamadas.Add(1)
	return g.gravador.Gravar(ctx, r, c, cart, linhas)
}

func TestAtendimentoGravaTurnoPorHTTP(t *testing.T) {
	b := bancotest.Novo(t)
	channel := uuid.New()
	condo := 101
	if _, err := b.Pool.Exec(t.Context(), `INSERT INTO channels(id,company_id,condo_id,name,channel_type,provider,endpoint_identifier,external_channel_id,feature_mode,enabled,root_agent,config_jsonb,updated_at) VALUES($1,'empresa',$2,'Alpha','WHATSAPP','ATENDE','55119000','tenant','INHERIT',true,'agentes/concierge','{"nexus_account_id":"cond-001","gruvi_company_id":"gruvi"}',now())`, channel, condo); err != nil {
		t.Fatal(err)
	}
	cat, err := okf.NewCatalogo(filepath.Join("..", "..", "..", "..", "okf"))
	if err != nil {
		t.Fatal(err)
	}
	sessoesPG := sessoespg.NovoSessaoPostgres(b)
	servidor, cliente, _, err := dados.IniciarComDados(dados.NovoGravadorPostgres(b), canaispg.NovoPostgres(b.Pool), sessoesPG, dados.NovasMemoriasPostgres(b), dados.NovosPedidosPostgres(b))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = servidor.Fechar(context.Background()) })
	gravacao := &contarGravacaoHTTP{gravador: cliente}
	svc := &atende.Servico{
		Canais: cliente, Cat: cat, Agente: "agentes/concierge", Mapeamento: adminpg.NovoPostgres(b.Ent),
		Sessoes: atende.NewSessoes(dados.NovasSessoes(cliente)), Pedidos: dados.NovosPedidos(cliente),
		Memorias: dados.NovasMemorias(cliente),
		Sender:   &atende.Outbox{}, Config: harness.Config{Mock: true, Env: func(string) string { return "" }}, GravacaoTurno: gravacao,
	}
	t.Cleanup(svc.Aguardar)
	msg := entrada.Mensagem{ID: "evento", ConversaID: "protocolo", ChannelID: channel, CompanyID: "empresa", Provider: "ATENDE", ChannelType: "WHATSAPP", Tenant: "tenant", Endpoint: "55119000", Contato: entrada.Contato{Phone: "5511999999999"}, Origem: entrada.Origem{Conector: "atende-emulado", Meio: "whatsapp"}, Destino: entrada.Destino{Conector: "atende-emulado"}, Texto: "bom dia"}
	sess, err := svc.Processar(t.Context(), msg)
	if err != nil {
		t.Fatal(err)
	}
	svc.Aguardar()
	if gravacao.chamadas.Load() != 1 {
		t.Fatalf("gravações HTTP = %d", gravacao.chamadas.Load())
	}
	ctx := atende.ComEscopoSessao(t.Context(), "empresa", channel)
	if _, ok, err := sessoespg.NovoSessaoPostgres(b).Carregar(ctx, sess.ID); err != nil || !ok {
		t.Fatalf("sessão gravada: encontrada=%v erro=%v", ok, err)
	}
}
