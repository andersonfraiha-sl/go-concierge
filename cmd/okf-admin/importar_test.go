package main

import (
	"strings"
	"testing"

	"go-concierge/internal/importacao"
)

func TestImprimirRelatorioUmaLinhaPorArea(t *testing.T) {
	var b strings.Builder
	imprimirRelatorio(&b, importacao.Relatorio{
		importacao.AreaMapeamento:              {Lidos: 6, Gravados: 6},
		importacao.AreaMemorias:                {Lidos: 2, Gravados: 1},
		importacao.AvisoProtocolosRedesenhados: {Lidos: 3, Gravados: 3},
	})
	quer := "mapeamento: lidos=6 gravados=6\nmemorias: lidos=2 gravados=1\npedidos: lidos=0 gravados=0\n"
	if !strings.HasPrefix(b.String(), quer) || !strings.Contains(b.String(), "aviso: 3 protocolo(s)") {
		t.Errorf("saída = %q", b.String())
	}
}
