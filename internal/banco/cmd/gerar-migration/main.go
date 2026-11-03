// gerar-migration escreve a próxima migration goose a partir do esquema do ent. O Atlas
// reaplica as migrations existentes num Postgres 18 descartável (testcontainers), compara
// com o esquema do ent e grava a diferença em internal/banco/migrations.
//
//	go generate ./internal/banco/ent
//	go run ./internal/banco/cmd/gerar-migration <nome>
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"time"

	atlasmigrate "ariga.io/atlas/sql/migrate"
	"ariga.io/atlas/sql/sqltool"
	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"entgo.io/ent/dialect/sql/schema"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	entmigrate "go-concierge/internal/banco/ent/migrate"
)

const timeoutGeracao = 5 * time.Minute

func main() {
	pasta := flag.String("dir", "internal/banco/migrations", "pasta das migrations goose")
	flag.Parse()
	if flag.NArg() != 1 {
		log.Fatal("uso: gerar-migration [-dir pasta] <nome>")
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeoutGeracao)
	defer cancel()
	if err := gerar(ctx, *pasta, flag.Arg(0)); err != nil {
		log.Fatal(err)
	}
}

func gerar(ctx context.Context, pasta, nome string) (err error) {
	dir, err := sqltool.NewGooseDir(pasta)
	if err != nil {
		return fmt.Errorf("abrir %s: %w", pasta, err)
	}
	c, err := postgres.Run(ctx, "postgres:18-alpine", postgres.BasicWaitStrategies())
	if err != nil {
		return fmt.Errorf("subir postgres descartável: %w", err)
	}
	defer func() {
		if errFim := testcontainers.TerminateContainer(c); errFim != nil {
			err = errors.Join(err, fmt.Errorf("encerrar postgres descartável: %w", errFim))
		}
	}()
	url, err := c.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return fmt.Errorf("endereço do postgres descartável: %w", err)
	}
	cfg, err := pgx.ParseConfig(url)
	if err != nil {
		return fmt.Errorf("ler endereço do postgres descartável: %w", err)
	}
	drv := entsql.OpenDB(dialect.Postgres, stdlib.OpenDB(*cfg))
	defer func() {
		if errFim := drv.Close(); errFim != nil {
			err = errors.Join(err, fmt.Errorf("fechar conexão: %w", errFim))
		}
	}()
	m, err := schema.NewMigrate(drv,
		schema.WithDir(dir),
		schema.WithMigrationMode(schema.ModeReplay),
		schema.WithDialect(dialect.Postgres),
		schema.WithFormatter(sqltool.GooseFormatter),
		schema.WithErrNoPlan(true),
	)
	if err != nil {
		return fmt.Errorf("abrir migrador do ent: %w", err)
	}
	err = m.NamedDiff(ctx, nome, entmigrate.Tables...)
	if errors.Is(err, atlasmigrate.ErrNoPlan) {
		log.Print("gerar-migration: o esquema do ent já está nas migrations, nada a gerar")
		return nil
	}
	if err != nil {
		return fmt.Errorf("gerar migration %s: %w", nome, err)
	}
	log.Printf("gerar-migration: migration %s escrita em %s; revise e commite", nome, pasta)
	return nil
}
