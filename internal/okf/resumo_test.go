package okf

import (
	"strings"
	"testing"
)

func TestResumoDe(t *testing.T) {
	tests := []struct {
		nome, summary, description, quer string
	}{
		{"resumo próprio vence", "  Agenda o dia da mudança. ", "qualquer", "Agenda o dia da mudança."},
		{
			"primeira frase", "",
			"Mostra as unidades vinculadas ao cadastro.\nOutra linha.",
			"Mostra as unidades vinculadas ao cadastro.",
		},
		{
			"espaços colapsados", "",
			"Atualiza dados   cadastrais\n da unidade. Depois.",
			"Atualiza dados cadastrais da unidade.",
		},
		{
			"prefixo \"quando\"", "",
			"Usado quando o morador quer liberar um visitante. Mais.",
			"Quer liberar um visitante.",
		},
		{
			"prefixo dono", "",
			"Usado para o dono ou morador da unidade autorizar a entrada de um visitante na portaria (amigo, parente).",
			"Autorizar a entrada de um visitante na portaria (amigo, parente).",
		},
		{
			"prefixo morador", "",
			"Usado para o morador ver as encomendas.",
			"Ver as encomendas.",
		},
		{
			"corta em 90", "",
			"Palavra" + strings.Repeat(" palavra", 15),
			"Palavra" + strings.Repeat(" palavra", 10) + "…",
		},
		{"vazio", "", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.nome, func(t *testing.T) {
			if got := ResumoDe(tt.summary, tt.description); got != tt.quer {
				t.Errorf("ResumoDe(%q, %q) = %q, quer %q", tt.summary, tt.description, got, tt.quer)
			}
		})
	}
}
