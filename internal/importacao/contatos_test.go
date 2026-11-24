package importacao

import (
	"testing"
	"time"
)

func TestRelatorioCompleto(t *testing.T) {
	tests := []struct {
		nome string
		rel  Relatorio
		quer bool
	}{
		{"vazio", Relatorio{}, true},
		{"tudo gravado", Relatorio{AreaMapeamento: {3, 3}, AreaMemorias: {2, 2}, AreaPedidos: {1, 1}}, true},
		{"memória a menos", Relatorio{AreaMapeamento: {3, 3}, AreaMemorias: {2, 1}}, false},
		{"pedido a menos", Relatorio{AreaPedidos: {4, 3}}, false},
		{"aviso não conta", Relatorio{AvisoProtocolosRedesenhados: {2, 2}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.nome, func(t *testing.T) {
			if got := tt.rel.Completo(); got != tt.quer {
				t.Errorf("Completo() = %v, quer %v", got, tt.quer)
			}
		})
	}
}

func instante(dia string) time.Time {
	t, err := time.Parse(time.DateOnly, dia)
	if err != nil {
		panic(err)
	}
	return t
}
