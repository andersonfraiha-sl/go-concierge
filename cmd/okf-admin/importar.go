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
	defer db.Fechar()

	ctx, cancel := context.WithTimeout(context.Background(), timeoutImportacao)
	defer cancel()
	rel, err := importacao.Importar(ctx, *dir, importacao.Destinos{
		Mapeamento: admin.NovoPostgres(db.Ent),
		Memorias:   memoria.NovoPostgres(db),
		Pedidos:    pedidos.NovoPostgres(db),
	})
	imprimirRelatorio(saida, rel)
	if err != nil {
		fmt.Fprintf(erro, "%s: %v\n", subcomandoImportarJSON, err)
		return 1
	}
	if !rel.Completo() {
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
		fmt.Fprintf(w, "aviso: %d protocolo(s) de pedido já eram de outro contato e foram sorteados de novo\n", n)
	}
}
