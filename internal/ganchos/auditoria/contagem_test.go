package auditoria_test

import (
	"maps"
	"testing"
	"time"

	"go-concierge/internal/banco/bancotest"
	"go-concierge/internal/ganchos/auditoria"
)

func TestDisparosPorRegraContaSoAsDecisoesDasRegrasPedidasNoPeriodo(t *testing.T) {
	b := bancotest.Novo(t)
	agora := time.Now().UTC()
	daRegra := func(id, decisao string, quando time.Time) auditoria.Linha {
		l := linha("regra", quando)
		l.RegraID, l.RegraVersao, l.Decisao = id, 1, decisao
		return l
	}
	linhas := []auditoria.Linha{
		daRegra("r-1", "bloquear", agora.Add(-time.Hour)),
		daRegra("r-1", "confirmar", agora.Add(-48*time.Hour)),
		daRegra("r-1", "falhou", agora.Add(-time.Hour)),
		daRegra("r-1", "bloquear", agora.Add(-8*24*time.Hour)),
		daRegra("r-2", "bloquear", agora.Add(-time.Hour)),
		daRegra("r-3", "bloquear", agora.Add(-time.Hour)),
		linha("escopo_da_conversa", agora),
	}
	if err := auditoria.Gravar(t.Context(), b, linhas, nil); err != nil {
		t.Fatal(err)
	}

	got, err := auditoria.DisparosPorRegra(t.Context(), b.Ent, agora.Add(-7*24*time.Hour), []string{"r-1", "r-2", "r-9"})

	if err != nil {
		t.Fatal(err)
	}
	if quer := map[string]int{"r-1": 2, "r-2": 1}; !maps.Equal(got, quer) {
		t.Fatalf("disparos = %v, quer %v", got, quer)
	}
}
