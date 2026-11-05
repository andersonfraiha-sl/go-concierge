package banco_test

import (
	"errors"
	"os"
	"strings"
	"testing"

	atlasmigrate "ariga.io/atlas/sql/migrate"
	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"entgo.io/ent/dialect/sql/schema"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"go-concierge/internal/banco"
	"go-concierge/internal/banco/bancotest"
	entmigrate "go-concierge/internal/banco/ent/migrate"
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
	plano := atlasmigrate.OpenMemDir(t.Name())
	defer plano.Close()
	drv := entsql.OpenDB(dialect.Postgres, stdlib.OpenDBFromPool(b.Pool))
	defer func() {
		if err := drv.Close(); err != nil {
			t.Errorf("fechar driver do ent: %v", err)
		}
	}()
	m, err := schema.NewMigrate(drv,
		schema.WithDir(plano),
		schema.WithErrNoPlan(true),
		schema.WithDialect(dialect.Postgres),
		schema.WithDropColumn(true),
		schema.WithDropIndex(true),
	)
	if err != nil {
		t.Fatalf("abrir migrador do ent: %v", err)
	}

	err = m.NamedDiff(t.Context(), "pendente", entmigrate.Tables...)

	if errors.Is(err, atlasmigrate.ErrNoPlan) {
		return
	}
	if err != nil {
		t.Fatalf("diff do Atlas: %v", err)
	}
	t.Fatalf("esquema do ent difere das migrations; rode go run ./internal/banco/cmd/gerar-migration <nome>:\n%s", conteudo(t, plano))
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
	p, err := goose.NewProvider(goose.DialectPostgres, stdlib.OpenDBFromPool(b.Pool), os.DirFS("migrations"))
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

func conteudo(t *testing.T, dir *atlasmigrate.MemDir) string {
	t.Helper()
	arquivos, err := dir.Files()
	if err != nil {
		t.Fatalf("ler plano: %v", err)
	}
	var sb strings.Builder
	for _, a := range arquivos {
		sb.Write(a.Bytes())
	}
	return sb.String()
}
