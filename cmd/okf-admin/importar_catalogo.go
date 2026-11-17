package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"go-concierge/internal/catalogo"
	"go-concierge/internal/env"
)

const (
	subcomandoImportarCatalogo = "importar-catalogo"
	timeoutImportarCatalogo    = 5 * time.Minute
)

var errPastaSemCatalogo = errors.New("a pasta não tem arquivos em agentes/ nem nexus/; importar assim apagaria o catálogo do banco")

// importarCatalogo é `okf-admin importar-catalogo -dir <pasta>`: deixa o catálogo do Postgres
// igual à pasta (agentes/ e nexus/). É o caminho para levar ao banco uma skill que chegou como
// arquivo. Devolve o código de saída.
func importarCatalogo(args []string, saida, erro io.Writer) int {
	fs := flag.NewFlagSet(subcomandoImportarCatalogo, flag.ContinueOnError)
	fs.SetOutput(erro)
	fs.Usage = func() {
		fmt.Fprintf(erro, "uso: okf-admin %s [-dir pasta] [-env arquivo]\n", subcomandoImportarCatalogo)
		fmt.Fprintln(erro, "ATENÇÃO: o banco passa a ter só o que está na pasta; o que foi editado no Studio e não está nela é apagado.")
		fs.PrintDefaults()
	}
	dir := fs.String("dir", "okf", "pasta com agentes/ e nexus/")
	dotenv := fs.String("env", ".env", "arquivo NOME=valor com DATABASE_URL")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	env.Carregar(*dotenv)

	db, err := abrirBanco(os.Getenv("DATABASE_URL"), migrarPorPadrao(os.Getenv("AMBIENTE")))
	if err != nil {
		fmt.Fprintf(erro, "%s: %v\n", subcomandoImportarCatalogo, err)
		return 1
	}
	defer func() {
		if err := db.Fechar(); err != nil {
			fmt.Fprintf(erro, "%s: fechar banco: %v\n", subcomandoImportarCatalogo, err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), timeoutImportarCatalogo)
	defer cancel()
	m, err := substituirCatalogo(ctx, catalogo.NovoPostgres(db.Ent), *dir)
	if err != nil {
		fmt.Fprintf(erro, "%s: %v\n", subcomandoImportarCatalogo, err)
		return 1
	}
	fmt.Fprintf(saida, "catálogo: gravados=%d removidos=%d\n", m.Gravados, m.Removidos)
	return 0
}

// substituirCatalogo recusa a pasta sem arquivo: um -dir errado não pode esvaziar o banco.
func substituirCatalogo(ctx context.Context, pg *catalogo.Postgres, dir string) (catalogo.Mudanca, error) {
	arquivos, err := catalogo.LerDisco(dir)
	if err != nil {
		return catalogo.Mudanca{}, err
	}
	if len(arquivos) == 0 {
		return catalogo.Mudanca{}, fmt.Errorf("%s: %w", dir, errPastaSemCatalogo)
	}
	return pg.Substituir(ctx, arquivos)
}
