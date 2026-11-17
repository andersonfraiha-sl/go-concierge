// Package banco abre o Postgres do concierge: um pool pgx só, compartilhado pelo ent (acesso
// padrão) e pelo sqlc (exceções de desempenho), e aplica as migrations goose embutidas.
package banco

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"

	"go-concierge/internal/banco/ent"
	"go-concierge/internal/banco/sqlc"
	"go-concierge/internal/falha"
)

const timeoutConexao = 5 * time.Second

//go:embed migrations/*.sql
var migrations embed.FS

// Banco reúne os clientes que falam com o Postgres. Todos usam o mesmo pool.
type Banco struct {
	Pool *pgxpool.Pool
	Ent  *ent.Client
	SQL  *sqlc.Queries
}

// Abrir conecta ao Postgres de url e confirma a conexão com um ping. ajustes mexem na
// configuração do pool antes de abrir (o benchmark liga um tracer).
func Abrir(ctx context.Context, url string, ajustes ...func(*pgxpool.Config)) (Banco, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return Banco{}, fmt.Errorf("ler DATABASE_URL: %w", err)
	}
	cfg.ConnConfig.ConnectTimeout = timeoutConexao
	// prepara cada SQL uma vez por conexão: as queries do turno, repetidas, custam uma ida
	// só (é o padrão do pgx; fica explícito porque o caminho do turno depende disso)
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeCacheStatement
	for _, ajustar := range ajustes {
		ajustar(cfg)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return Banco{}, falha.ComPilha(fmt.Errorf("abrir pool do postgres: %w", err))
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return Banco{}, falha.ComPilha(fmt.Errorf("ping no postgres: %w", err))
	}
	drv := entsql.OpenDB(dialect.Postgres, stdlib.OpenDBFromPool(pool))
	return Banco{Pool: pool, Ent: ent.NewClient(ent.Driver(drv)), SQL: sqlc.New(pool)}, nil
}

// Fechar encerra o ent e depois o pool que ele usa (fechar o ent não fecha o pool).
func (b Banco) Fechar() error {
	err := b.Ent.Close()
	b.Pool.Close()
	if err != nil {
		return fmt.Errorf("fechar ent: %w", err)
	}
	return nil
}

// Migrar aplica as migrations pendentes. Um advisory lock do Postgres impede que duas
// instâncias migrem ao mesmo tempo.
func Migrar(ctx context.Context, b Banco) (err error) {
	trava, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return fmt.Errorf("criar trava das migrations: %w", err)
	}
	arquivos, err := fs.Sub(migrations, "migrations")
	if err != nil {
		return fmt.Errorf("ler migrations embutidas: %w", err)
	}
	p, err := goose.NewProvider(goose.DialectPostgres, stdlib.OpenDBFromPool(b.Pool), arquivos,
		goose.WithSessionLocker(trava))
	if err != nil {
		return fmt.Errorf("abrir goose: %w", err)
	}
	defer func() {
		if errFechar := p.Close(); errFechar != nil {
			err = errors.Join(err, fmt.Errorf("fechar goose: %w", errFechar))
		}
	}()
	if _, err := p.Up(ctx); err != nil {
		return falha.ComPilha(fmt.Errorf("aplicar migrations: %w", err))
	}
	return nil
}
