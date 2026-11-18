package harness

import (
	"go-concierge/internal/okf"
	"testing"
)

func TestCanalPreservaConhecimentoEBloqueiaCasoDiretoEDescendente(t *testing.T) {
	cat, def := catalogoExemplo(t)
	var conhecimento, caso *okf.Agent
	for _, sub := range def.Subagentes {
		if sub.Nome == "regimento" {
			conhecimento = sub
		}
		if sub.Caso != nil {
			caso = sub
		}
	}
	if conhecimento == nil || caso == nil {
		t.Fatal("fixture sem conhecimento/caso")
	}
	cfg := Config{Mock: true, Env: semEnv, CompanyID: "Nexus", Casos: func(_ string, id string) bool { return id == "" }}
	ag, err := Build(t.Context(), cat, def, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if ag.Subagentes[conhecimento.Nome] == nil || ag.Subagentes[caso.Nome] != nil {
		t.Fatal("filtro de casos removeu conhecimento ou preservou caso indisponível")
	}
	if _, err := Build(t.Context(), cat, caso, cfg); err == nil {
		t.Fatal("execução direta de caso desativado aceita")
	}
	aux := *conhecimento
	aux.Subagentes = []*okf.Agent{caso}
	raiz := *def
	raiz.Subagentes = []*okf.Agent{&aux}
	ag, err = Build(t.Context(), cat, &raiz, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if ag.Subagentes[aux.Nome].Subagentes[caso.Nome] != nil {
		t.Fatal("auxiliar expôs caso descendente indisponível")
	}
}
