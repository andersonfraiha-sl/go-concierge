package conciergeapi

import (
	"context"
	"fmt"
	"io"
	"time"

	"go-concierge/internal/core/administracao/importacao"
)

const (
	subcomandoImportarJSON = "importar-json"
	timeoutImportacao      = 30 * time.Minute
)

// importarJSON é `okf-admin importar-json -dir <pasta>`: leva o mapeamento, as memórias e os
// pedidos em JSON para o Postgres de DATABASE_URL. Devolve o código de saída: 1 se algo
// falhou ou se alguma área gravou menos do que leu.
func importarJSON(args []string, saida, erro io.Writer) int {
	fmt.Fprintln(erro, "importar-json: o banco v2 inicia vazio; configure empresa Nexus, canais e casos explicitamente por SQL/seed. A importação de identidades locais legadas foi removida.")
	return 1
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
