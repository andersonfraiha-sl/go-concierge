package ganchos_test

import (
	"context"
	"maps"
	"slices"
	"testing"

	"go-concierge/internal/admin"
	"go-concierge/internal/ganchos"
	"go-concierge/internal/ganchos/ganchostest"
)

// anotador deixa passar e anota o próprio nome em ordem: serve a qualquer evento.
func anotador[E ganchos.Ocorrencia](nome string, ordem *[]string) ganchos.Gancho[E] {
	return ganchos.Gancho[E]{
		Nome: nome,
		Avaliar: func(e E) (E, ganchos.Decisao) {
			*ordem = append(*ordem, nome)
			return e, ganchos.Decisao{Tipo: ganchos.Deixar}
		},
	}
}

// cenario monta, para um evento, fixos e regras em todos os níveis (mais uma regra de
// outro condomínio e de outra administradora, que não podem rodar) e devolve a ordem.
func cenario[E ganchos.Ocorrencia](ordem *[]string, checagemExtra ...ganchos.Gancho[E]) (ganchos.Pontos[E], ganchos.Nivel[E]) {
	p := ganchos.Pontos[E]{
		Checagem:   append(checagemExtra, anotador[E]("fixo-1", ordem), anotador[E]("fixo-2", ordem)),
		Saneamento: []ganchos.Gancho[E]{anotador[E]("saneamento", ordem)},
	}
	n := ganchos.Nivel[E]{
		PorCondominio: map[string][]ganchos.Gancho[E]{
			"c1": {anotador[E]("regra-c1-a", ordem), anotador[E]("regra-c1-b", ordem)},
			"c2": {anotador[E]("regra-c2", ordem)},
		},
		PorAdministradora: map[string][]ganchos.Gancho[E]{
			"a1": {anotador[E]("regra-a1", ordem)},
			"a2": {anotador[E]("regra-a2", ordem)},
		},
		Globais: []ganchos.Gancho[E]{anotador[E]("regra-global", ordem)},
	}
	return p, n
}

var ordemEsperada = []string{"fixo-1", "fixo-2", "regra-c1-a", "regra-c1-b", "regra-a1", "regra-global", "saneamento"}

func TestConjuntoOrdem(t *testing.T) {
	var ordem []string
	var f ganchos.Fixos
	var idx ganchos.Indice

	// Em mensagem_recebida o condomínio só existe depois da checagem de escopo: as
	// regras saem do evento já ajustado por ela.
	escopo := ganchos.Gancho[ganchos.MensagemRecebida]{
		Nome: "escopo",
		Avaliar: func(e ganchos.MensagemRecebida) (ganchos.MensagemRecebida, ganchos.Decisao) {
			e.Condominio = admin.Condominio{ID: "c1", Administradora: "a1"}
			return e, ganchos.Decisao{Tipo: ganchos.Ajustar}
		},
	}
	f.MensagemRecebida, idx.MensagemRecebida = cenario(&ordem, escopo)
	f.AntesDaFerramenta, idx.AntesDaFerramenta = cenario[ganchos.AntesDaFerramenta](&ordem)
	f.DepoisDaFerramenta, idx.DepoisDaFerramenta = cenario[ganchos.DepoisDaFerramenta](&ordem)
	f.AntesDeResponder, idx.AntesDeResponder = cenario[ganchos.AntesDeResponder](&ordem)
	f.TransbordoOuEncerramento, idx.TransbordoOuEncerramento = cenario[ganchos.TransbordoOuEncerramento](&ordem)

	var observados [][]ganchos.Registro
	c := ganchos.NovoConjunto(f, func() *ganchos.Indice { return &idx }, ganchos.ComObservador(
		func(_ context.Context, rs []ganchos.Registro) { observados = append(observados, rs) },
	))
	ctx := context.Background()

	chamadas := []struct {
		name  string
		rodar func()
	}{
		{"mensagem_recebida", func() { c.MensagemRecebida(ctx, ganchos.MensagemRecebida{}) }},
		{"antes_da_ferramenta", func() {
			c.AntesDaFerramenta(ctx, ganchos.AntesDaFerramenta{CondominioID: "c1", AdministradoraID: "a1"})
		}},
		{"depois_da_ferramenta", func() {
			c.DepoisDaFerramenta(ctx, ganchos.DepoisDaFerramenta{CondominioID: "c1", AdministradoraID: "a1"})
		}},
		{"antes_de_responder", func() {
			c.AntesDeResponder(ctx, ganchos.AntesDeResponder{CondominioID: "c1", AdministradoraID: "a1"})
		}},
		{"transbordo_ou_encerramento", func() {
			c.TransbordoOuEncerramento(ctx, ganchos.TransbordoOuEncerramento{CondominioID: "c1", AdministradoraID: "a1"})
		}},
	}
	for _, ch := range chamadas {
		t.Run(ch.name, func(t *testing.T) {
			ordem = nil
			ch.rodar()
			if !slices.Equal(ordem, ordemEsperada) {
				t.Errorf("ordem = %v, want %v", ordem, ordemEsperada)
			}
		})
	}
	if len(observados) != len(chamadas) {
		t.Errorf("observar chamado %d vezes, want %d", len(observados), len(chamadas))
	}
	if len(observados) > 0 && len(observados[0]) != 1 {
		t.Errorf("registros de mensagem_recebida = %+v, want só o ajuste do escopo", observados[0])
	}
}

func TestRegraNaoAfrouxaFixo(t *testing.T) {
	escopo := &ganchostest.Roteiro[ganchos.AntesDaFerramenta]{
		Decisao: ganchos.Decisao{Tipo: ganchos.Bloquear, Motivo: "fora do escopo"},
	}
	regra := &ganchostest.Roteiro[ganchos.AntesDaFerramenta]{}
	mascara := &ganchostest.Roteiro[ganchos.AntesDaFerramenta]{}

	fixo := ganchostest.Gancho("escopo", escopo)
	fixo.Fixo = true
	f := ganchos.Fixos{AntesDaFerramenta: ganchos.Pontos[ganchos.AntesDaFerramenta]{
		Checagem:   []ganchos.Gancho[ganchos.AntesDaFerramenta]{fixo},
		Saneamento: []ganchos.Gancho[ganchos.AntesDaFerramenta]{ganchostest.Gancho("mascara", mascara)},
	}}
	idx := &ganchos.Indice{AntesDaFerramenta: ganchos.Nivel[ganchos.AntesDaFerramenta]{
		Globais: []ganchos.Gancho[ganchos.AntesDaFerramenta]{ganchostest.Gancho("regra", regra)},
	}}

	r := ganchostest.Conjunto(f, idx).AntesDaFerramenta(context.Background(), ganchos.AntesDaFerramenta{Ferramenta: "reservar"})

	if r.Decisao.Tipo != ganchos.Bloquear || r.Decisao.Motivo != "fora do escopo" {
		t.Errorf("Decisao = %+v, want o bloqueio do fixo", r.Decisao)
	}
	if !r.Decisao.Origem.Fixo || r.Decisao.Origem.Gancho != "escopo" {
		t.Errorf("Origem = %+v, want o fixo escopo", r.Decisao.Origem)
	}
	if len(escopo.Recebidos) != 1 {
		t.Errorf("fixo recebeu %d eventos, want 1", len(escopo.Recebidos))
	}
	if len(regra.Recebidos) != 0 || len(mascara.Recebidos) != 0 {
		t.Errorf("depois do bloqueio rodaram regra=%d saneamento=%d, want 0", len(regra.Recebidos), len(mascara.Recebidos))
	}
}

func TestConjuntoSemRegras(t *testing.T) {
	tests := []struct {
		name   string
		regras func() *ganchos.Indice
	}{
		{"regras nil", nil},
		{"indice nil", func() *ganchos.Indice { return nil }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixo := &ganchostest.Roteiro[ganchos.AntesDeResponder]{}
			f := ganchos.Fixos{AntesDeResponder: ganchos.Pontos[ganchos.AntesDeResponder]{
				Saneamento: []ganchos.Gancho[ganchos.AntesDeResponder]{ganchostest.Gancho("mascara", fixo)},
			}}

			r := ganchos.NovoConjunto(f, tt.regras).AntesDeResponder(context.Background(), ganchos.AntesDeResponder{Texto: "oi"})

			if r.Decisao.Tipo != ganchos.Deixar || len(fixo.Recebidos) != 1 {
				t.Errorf("Decisao = %+v, fixo recebeu %d, want deixar e 1", r.Decisao, len(fixo.Recebidos))
			}
		})
	}
}

func TestNomesDosFixos(t *testing.T) {
	semNada := func(nome string) ganchos.Gancho[ganchos.AntesDaFerramenta] {
		return ganchostest.Gancho(nome, &ganchostest.Roteiro[ganchos.AntesDaFerramenta]{})
	}
	f := ganchos.Fixos{
		MensagemRecebida: ganchos.Pontos[ganchos.MensagemRecebida]{
			Checagem:   []ganchos.Gancho[ganchos.MensagemRecebida]{ganchostest.Gancho("escopo", &ganchostest.Roteiro[ganchos.MensagemRecebida]{})},
			Saneamento: []ganchos.Gancho[ganchos.MensagemRecebida]{ganchostest.Gancho("pii", &ganchostest.Roteiro[ganchos.MensagemRecebida]{})},
		},
		AntesDaFerramenta: ganchos.Pontos[ganchos.AntesDaFerramenta]{
			Checagem:   []ganchos.Gancho[ganchos.AntesDaFerramenta]{semNada("permissao"), semNada("contexto"), semNada("confirmacao")},
			Saneamento: []ganchos.Gancho[ganchos.AntesDaFerramenta]{semNada("pii")},
		},
	}

	got := ganchos.NovoConjunto(f, nil).NomesDosFixos()

	want := map[ganchos.NomeDoEvento][]string{
		ganchos.EventoMensagemRecebida:         {"escopo", "pii"},
		ganchos.EventoAntesDaFerramenta:        {"permissao", "contexto", "confirmacao", "pii"},
		ganchos.EventoDepoisDaFerramenta:       nil,
		ganchos.EventoAntesDeResponder:         nil,
		ganchos.EventoTransbordoOuEncerramento: nil,
	}
	if !maps.EqualFunc(got, want, slices.Equal) {
		t.Errorf("NomesDosFixos = %v, want %v", got, want)
	}
}
