package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"go-concierge/internal/admin"
	"go-concierge/internal/env"
	"go-concierge/internal/importacao"
	"go-concierge/internal/memoria"
	"go-concierge/internal/pedidos"
)

const (
	subcomandoImportarJSON = "importar-json"
	timeoutImportacao      = 30 * time.Minute
)

// importarJSON é `okf-admin importar-json -dir <pasta>`: leva o mapeamento, as memórias e os
// pedidos em JSON para o Postgres de DATABASE_URL. Devolve o código de saída: 1 se algo
// falhou ou se alguma área gravou menos do que leu.
func importarJSON(args []string, saida, erro io.Writer) int {
	fs := flag.NewFlagSet(subcomandoImportarJSON, flag.ContinueOnError)
	fs.SetOutput(erro)
	fs.Usage = func() {
		fmt.Fprintf(erro, "uso: okf-admin %s [-dir pasta] [-env arquivo]\n", subcomandoImportarJSON)
		fmt.Fprintln(erro, "ATENÇÃO: importar de novo sobrescreve no Postgres o que o Admin editou desde a importação anterior.")
		fs.PrintDefaults()
	}
	dir := fs.String("dir", "okf/var", "pasta com admin/, memorias/ e pedidos/ em JSON")
	dotenv := fs.String("env", ".env", "arquivo NOME=valor com DATABASE_URL")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	env.Carregar(*dotenv)

	db, err := abrirBanco(os.Getenv("DATABASE_URL"), migrarPorPadrao(os.Getenv("AMBIENTE")))
	if err != nil {
		fmt.Fprintf(erro, "%s: %v\n", subcomandoImportarJSON, err)
		return 1
	}
	defer func() {
		if err := db.Fechar(); err != nil {
			fmt.Fprintf(erro, "%s: fechar banco: %v\n", subcomandoImportarJSON, err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), timeoutImportacao)
	defer cancel()
	return executarImportacao(ctx, *dir, importacao.Destinos{
		Mapeamento: admin.NovoPostgres(db.Ent),
		Memorias:   memoria.NovoPostgres(db),
		Pedidos:    pedidos.NovoPostgres(db),
	}, saida, erro)
}

// executarImportacao roda a importação e devolve o código de saída: 1 se houve erro ou se
// alguma área gravou menos do que leu.
func executarImportacao(ctx context.Context, dir string, destinos importacao.Destinos, saida, erro io.Writer) int {
	destinos.Aviso = func(texto string) { fmt.Fprintf(saida, "aviso: %s\n", texto) }
	rel, err := importacao.Importar(ctx, dir, destinos)
	imprimirRelatorio(saida, rel)
	if err != nil {
		fmt.Fprintf(erro, "%s: %v\n", subcomandoImportarJSON, err)
		return 1
	}
	if !rel.Completo() {
		fmt.Fprintf(erro, "%s: alguma área gravou menos do que leu\n", subcomandoImportarJSON)
		return 1
	}
	return 0
}

func imprimirRelatorio(w io.Writer, rel importacao.Relatorio) {
	for _, area := range []string{importacao.AreaMapeamento, importacao.AreaMemorias, importacao.AreaPedidos} {
		c := rel[area]
		fmt.Fprintf(w, "%s: lidos=%d gravados=%d\n", area, c.Lidos, c.Gravados)
	}
	if n := rel[importacao.AvisoProtocolosRedesenhados].Lidos; n > 0 {
		fmt.Fprintf(w, "aviso: %d protocolo(s) de pedido já eram de outro contato e foram sorteados de novo (cada troca está acima)\n", n)
	}
}
