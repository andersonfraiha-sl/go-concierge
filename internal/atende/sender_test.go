package atende

import (
	"strings"
	"testing"
)

func TestDividirMensagemUsaLimitesEFormataWhatsApp(t *testing.T) {
	texto := strings.Repeat("a", 3990) + " \n\n" + strings.Repeat("b", 30)
	partes := DividirMensagem(texto, 4000)
	if len(partes) != 2 || len([]rune(partes[0])) > 4000 {
		t.Fatalf("divisão inesperada: %d partes, primeira=%d", len(partes), len([]rune(partes[0])))
	}
	formatado := FormatarMensagem("# Título\n* item **forte**\n- outro\n####### título")
	if formatado != "*Título*\n- item *forte*\n- outro\n####### título" {
		t.Fatalf("formatação inesperada: %q", formatado)
	}
}
