package ganchostest_test

import (
	"context"
	"testing"

	"go-concierge/internal/ganchos"
	"go-concierge/internal/ganchos/ganchostest"
)

func TestGancho(t *testing.T) {
	tests := []struct {
		name     string
		decisao  ganchos.Decisao
		wantTipo ganchos.TipoDecisao
	}{
		{"roteiro vazio deixa", ganchos.Decisao{}, ganchos.Deixar},
		{"devolve a decisao do roteiro", ganchos.Decisao{Tipo: ganchos.Confirmar, Motivo: "posso?"}, ganchos.Confirmar},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &ganchostest.Roteiro[ganchos.AntesDeResponder]{Decisao: tt.decisao}
			g := ganchostest.Gancho("roteirizado", r)
			ev := ganchos.AntesDeResponder{Texto: "oi"}

			res := ganchos.Avaliar(context.Background(), []ganchos.Gancho[ganchos.AntesDeResponder]{g}, ev)

			if g.Nome != "roteirizado" {
				t.Errorf("Nome = %q", g.Nome)
			}
			if res.Decisao.Tipo != tt.wantTipo {
				t.Errorf("Decisao.Tipo = %q, want %q", res.Decisao.Tipo, tt.wantTipo)
			}
			if len(r.Recebidos) != 1 || r.Recebidos[0].Texto != "oi" {
				t.Errorf("Recebidos = %+v, want o evento uma vez", r.Recebidos)
			}
		})
	}
}

func TestConjunto(t *testing.T) {
	regra := &ganchostest.Roteiro[ganchos.AntesDeResponder]{Decisao: ganchos.Decisao{Tipo: ganchos.Bloquear, Motivo: "não"}}
	idx := &ganchos.Indice{AntesDeResponder: ganchos.Nivel[ganchos.AntesDeResponder]{
		Globais: []ganchos.Gancho[ganchos.AntesDeResponder]{ganchostest.Gancho("regra", regra)},
	}}

	res := ganchostest.Conjunto(ganchos.Fixos{}, idx).AntesDeResponder(context.Background(), ganchos.AntesDeResponder{})

	if res.Decisao.Tipo != ganchos.Bloquear || len(regra.Recebidos) != 1 {
		t.Errorf("Decisao = %+v, regra recebeu %d, want bloquear e 1", res.Decisao, len(regra.Recebidos))
	}
}
