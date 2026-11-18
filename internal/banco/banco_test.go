package banco_test

import (
	"os"
	"os/exec"
	"testing"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"go-concierge/internal/banco"
	"go-concierge/internal/banco/bancotest"
)

func TestMain(m *testing.M) {
	codigo := m.Run()
	bancotest.Encerrar()
	os.Exit(codigo)
}

func TestMigrarIdempotente(t *testing.T) {
	ctx := t.Context()
	b := bancotest.Novo(t)

	if err := banco.Migrar(ctx, b); err != nil {
		t.Fatalf("primeira Migrar: %v", err)
	}
	primeira := versaoDoBanco(t, b)
	if err := banco.Migrar(ctx, b); err != nil {
		t.Fatalf("segunda Migrar: %v", err)
	}
	segunda := versaoDoBanco(t, b)

	if maxima := versaoMaxima(t, b); primeira != maxima || segunda != maxima {
		t.Fatalf("versões após Migrar: %d e %d, quero as duas em %d", primeira, segunda, maxima)
	}
}

// TestMigracoesReversiveis desce todas as migrations (Down, na ordem inversa) e sobe de
// novo (Up): cada Down precisa deixar o banco exatamente como o Up anterior o encontrou,
// senão um rollback em produção quebraria o esquema.
func TestMigracoesReversiveis(t *testing.T) {
	ctx := t.Context()
	b := bancotest.Novo(t)

	if err := banco.Migrar(ctx, b); err != nil {
		t.Fatalf("Up inicial: %v", err)
	}
	maxima := versaoMaxima(t, b)

	p := provedorGoose(t, b)
	if _, err := p.DownTo(ctx, 0); err != nil {
		t.Fatalf("Down até zero: %v", err)
	}
	if v := versaoDoBanco(t, b); v != 0 {
		t.Fatalf("versão após Down até zero = %d, quero 0", v)
	}

	if _, err := p.Up(ctx); err != nil {
		t.Fatalf("Up de novo após Down: %v", err)
	}
	if v := versaoDoBanco(t, b); v != maxima {
		t.Fatalf("versão após Up de novo = %d, quero %d", v, maxima)
	}
}

// TestEsquemaEmDia falha quando alguém muda o esquema do ent sem gerar a migration, ou põe
// coluna ou índice à mão numa migration: o Atlas compara o banco migrado com o esquema do
// ent, nos dois sentidos, e não pode sobrar diferença.
func TestEsquemaEmDia(t *testing.T) {
	b := bancotest.Novo(t)
	command := exec.CommandContext(t.Context(), "go", "run", "./internal/banco/v2/cmd/gerar-baseline", "-check")
	command.Dir = "../.."
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("baseline v2 diverge da fonte canônica: %v\n%s", err, out)
	}
	var total int
	if err := b.Pool.QueryRow(t.Context(), `SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_type='BASE TABLE' AND table_name<>'goose_db_version'`).Scan(&total); err != nil {
		t.Fatal(err)
	}
	if total != 24 {
		t.Fatalf("tabelas = %d, esperado 24", total)
	}
}

func TestBootstrapRecusaDestinoLegadoSemAlterarSeusDados(t *testing.T) {
	b := bancotest.Novo(t)
	ctx := t.Context()
	if _, err := b.Pool.Exec(ctx, `CREATE TABLE administradoras(id text PRIMARY KEY); INSERT INTO administradoras VALUES('preservada')`); err != nil {
		t.Fatal(err)
	}
	if err := banco.Migrar(ctx, b); err == nil {
		t.Fatal("migration aceitou tabela legada")
	}
	if novo, err := banco.Abrir(ctx, b.Pool.Config().ConnString()); err == nil {
		_ = novo.Fechar()
		t.Fatal("bootstrap aceitou tabela legada")
	}
	var id string
	if err := b.Pool.QueryRow(ctx, `SELECT id FROM administradoras`).Scan(&id); err != nil || id != "preservada" {
		t.Fatalf("dado legado alterado: %q %v", id, err)
	}
}

func versaoDoBanco(t *testing.T, b banco.Banco) int64 {
	t.Helper()
	v, err := provedorGoose(t, b).GetDBVersion(t.Context())
	if err != nil {
		t.Fatalf("ler versão do goose: %v", err)
	}
	return v
}

func versaoMaxima(t *testing.T, b banco.Banco) int64 {
	t.Helper()
	fontes := provedorGoose(t, b).ListSources()
	if len(fontes) == 0 {
		t.Fatal("nenhuma migration em migrations/")
	}
	return fontes[len(fontes)-1].Version
}

func provedorGoose(t *testing.T, b banco.Banco) *goose.Provider {
	t.Helper()
	p, err := goose.NewProvider(goose.DialectPostgres, stdlib.OpenDBFromPool(b.Pool), os.DirFS("v2/migrations"))
	if err != nil {
		t.Fatalf("abrir goose: %v", err)
	}
	t.Cleanup(func() {
		if err := p.Close(); err != nil {
			t.Errorf("fechar goose: %v", err)
		}
	})
	return p
}
