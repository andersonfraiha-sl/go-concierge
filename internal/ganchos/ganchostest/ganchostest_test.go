package ganchostest_test

import (
	"context"
	"testing"
	"time"

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
			if len(r.Eventos()) != 1 || r.Eventos()[0].Texto != "oi" {
				t.Errorf("Recebidos = %+v, want o evento uma vez", r.Eventos())
			}
		})
	}
}

func TestConjunto(t *testing.T) {
	regra := &ganchostest.Roteiro[ganchos.AntesDeResponder]{Decisao: ganchos.Decisao{Tipo: ganchos.Bloquear, Motivo: "não"}}
	idx := &ganchos.Indice{AntesDeResponder: ganchos.Nivel[ganchos.AntesDeResponder]{
		Globais: []ganchos.Gancho[ganchos.AntesDeResponder]{ganchostest.Gancho("regra", regra)},
	}}

	res := ganchostest.Conjunto(ganchos.Fixos{}, idx).Turno().AntesDeResponder(context.Background(), ganchos.AntesDeResponder{})

	if res.Decisao.Tipo != ganchos.Bloquear || len(regra.Eventos()) != 1 {
		t.Errorf("Decisao = %+v, regra recebeu %d, want bloquear e 1", res.Decisao, len(regra.Eventos()))
	}
}

// Uma pausa do escalonador maior que o prazo padrão (2 ms) no meio da regra não muda o
// turno do teste: era o que fazia a prévia deixar a reserva passar e o turno ir ao
// transbordo nas rodadas cheias de go test.
func TestConjuntoNaoFalhaRegraPorPausaDoEscalonador(t *testing.T) {
	regra := ganchos.Gancho[ganchos.AntesDeResponder]{Nome: "pausada", Avaliar: func(e ganchos.AntesDeResponder) (ganchos.AntesDeResponder, ganchos.Decisao) {
		time.Sleep(2 * ganchos.PrazoPadrao)
		return e, ganchos.Decisao{Tipo: ganchos.Bloquear, Motivo: "fechado"}
	}}
	idx := &ganchos.Indice{AntesDeResponder: ganchos.Nivel[ganchos.AntesDeResponder]{Globais: []ganchos.Gancho[ganchos.AntesDeResponder]{regra}}}

	r := ganchostest.Conjunto(ganchos.Fixos{}, idx).Turno().AntesDeResponder(context.Background(), ganchos.AntesDeResponder{})

	if len(r.Registros) != 1 || r.Registros[0].Falhou || r.Decisao.Motivo != "fechado" {
		t.Fatalf("a regra devia decidir, sem falha de prazo: %+v", r.Registros)
	}
}
