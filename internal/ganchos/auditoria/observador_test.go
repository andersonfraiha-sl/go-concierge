package auditoria_test

import (
	"context"
	"testing"
	"time"

	"go-concierge/internal/banco/bancotest"
	"go-concierge/internal/ganchos"
	"go-concierge/internal/ganchos/auditoria"
	"go-concierge/internal/ganchos/fixos"
	"go-concierge/internal/ganchos/ganchostest"
)

// bloqueioNaResposta avalia antes_de_responder com uma regra que bloqueia, num turno normal
// ou de prévia.
func bloqueioNaResposta(ctx context.Context, aud *auditoria.Auditor, previa bool) {
	r := &ganchostest.Roteiro[ganchos.AntesDeResponder]{Decisao: ganchos.Decisao{Tipo: ganchos.Bloquear, Motivo: "não"}}
	g := ganchostest.Gancho("regra", r)
	g.Origem = ganchos.Origem{RegraID: "r-1", RegraVersao: 2}
	idx := &ganchos.Indice{AntesDeResponder: ganchos.Nivel[ganchos.AntesDeResponder]{Globais: []ganchos.Gancho[ganchos.AntesDeResponder]{g}}}
	conjunto := ganchos.NovoConjunto(fixos.Todos(), func() *ganchos.Indice { return idx }, aud.Observador())
	turno := conjunto.Turno()
	if previa {
		turno = conjunto.TurnoDePrevia()
	}
	turno.AntesDeResponder(ctx, ganchos.AntesDeResponder{CondominioID: "cond-1", AdministradoraID: "adm-a"})
}

func TestAuditorGravaCadaObservacaoNumDestinoSo(t *testing.T) {
	tests := []struct {
		nome                         string
		coletar, fecharAntes, previa bool
		noBanco, coletadas           int
	}{
		{"sem coletor grava na hora", false, false, false, 1, 0},
		{"coletor aberto guarda para o lote", true, false, false, 0, 1},
		{"coletor fechado grava na hora", true, true, false, 1, 0},
		{"prévia sem coletor não vai ao banco", false, false, true, 0, 0},
		{"prévia com coletor não é coletada", true, false, true, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.nome, func(t *testing.T) {
			b := bancotest.Novo(t)
			aud := auditoria.NovoAuditor(b, nil, time.Now)
			ctx := auditoria.ComBase(t.Context(), auditoria.Linha{SessaoID: "s-1"})
			var coletor *auditoria.Coletor
			if tt.coletar {
				ctx, coletor = auditoria.ComColetor(ctx)
			}
			if tt.fecharAntes {
				coletor.Fechar()
			}

			bloqueioNaResposta(ctx, aud, tt.previa)

			if n := contar(t, b); n != tt.noBanco {
				t.Errorf("no banco = %d, want %d", n, tt.noBanco)
			}
			coletadas := coletor.Fechar()
			if len(coletadas) != tt.coletadas {
				t.Fatalf("coletadas = %d, want %d", len(coletadas), tt.coletadas)
			}
			for _, l := range coletadas {
				if l.SessaoID != "s-1" || l.CondominioID != "cond-1" || l.RegraID != "r-1" || l.Origem != ganchos.OrigemConcierge || l.CriadoEm.IsZero() {
					t.Errorf("linha coletada = %+v", l)
				}
			}
		})
	}
}
