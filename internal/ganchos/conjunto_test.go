package ganchos_test

import (
	"context"
	"maps"
	"slices"
	"strings"
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
// outro condomínio e de outra administradora, que não podem rodar).
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

	var observadas []ganchos.Observacao
	c := ganchos.NovoConjunto(f, func() *ganchos.Indice { return &idx }, ganchos.ComObservador(
		func(_ context.Context, o ganchos.Observacao) { observadas = append(observadas, o) },
	))
	turno := c.Turno()
	ctx := context.Background()

	chamadas := []struct {
		evento ganchos.NomeDoEvento
		rodar  func()
	}{
		{ganchos.EventoMensagemRecebida, func() { turno.MensagemRecebida(ctx, ganchos.MensagemRecebida{}) }},
		{ganchos.EventoAntesDaFerramenta, func() {
			turno.AntesDaFerramenta(ctx, ganchos.AntesDaFerramenta{CondominioID: "c1", AdministradoraID: "a1"})
		}},
		{ganchos.EventoDepoisDaFerramenta, func() {
			turno.DepoisDaFerramenta(ctx, ganchos.DepoisDaFerramenta{CondominioID: "c1", AdministradoraID: "a1"})
		}},
		{ganchos.EventoAntesDeResponder, func() {
			turno.AntesDeResponder(ctx, ganchos.AntesDeResponder{CondominioID: "c1", AdministradoraID: "a1"})
		}},
		{ganchos.EventoTransbordoOuEncerramento, func() {
			turno.TransbordoOuEncerramento(ctx, ganchos.TransbordoOuEncerramento{CondominioID: "c1", AdministradoraID: "a1"})
		}},
	}
	for i, ch := range chamadas {
		t.Run(string(ch.evento), func(t *testing.T) {
			ordem = nil
			ch.rodar()
			if !slices.Equal(ordem, ordemEsperada) {
				t.Errorf("ordem = %v, want %v", ordem, ordemEsperada)
			}
			if len(observadas) != i+1 {
				t.Fatalf("observar chamado %d vezes, want %d", len(observadas), i+1)
			}
			o := observadas[i]
			if o.Evento != ch.evento || o.CondominioID != "c1" || o.AdministradoraID != "a1" {
				t.Errorf("Observacao = %+v, want %q em c1/a1", o, ch.evento)
			}
		})
	}
	if len(observadas) > 0 && len(observadas[0].Registros) != 1 {
		t.Errorf("registros de mensagem_recebida = %+v, want só o ajuste do escopo", observadas[0].Registros)
	}
}

func TestRegraNaoAfrouxaFixo(t *testing.T) {
	escopo := &ganchostest.Roteiro[ganchos.AntesDaFerramenta]{
		Decisao: ganchos.Decisao{Tipo: ganchos.Bloquear, Motivo: "fora do escopo"},
	}
	regra := &ganchostest.Roteiro[ganchos.AntesDaFerramenta]{}
	mascara := &ganchostest.Roteiro[ganchos.AntesDaFerramenta]{}

	f := ganchos.Fixos{AntesDaFerramenta: ganchos.Pontos[ganchos.AntesDaFerramenta]{
		Checagem:   []ganchos.Gancho[ganchos.AntesDaFerramenta]{ganchostest.Gancho("escopo", escopo)},
		Saneamento: []ganchos.Gancho[ganchos.AntesDaFerramenta]{ganchostest.Gancho("mascara", mascara)},
	}}
	idx := &ganchos.Indice{AntesDaFerramenta: ganchos.Nivel[ganchos.AntesDaFerramenta]{
		Globais: []ganchos.Gancho[ganchos.AntesDaFerramenta]{ganchostest.Gancho("regra", regra)},
	}}

	r := ganchostest.Conjunto(f, idx).Turno().AntesDaFerramenta(context.Background(), ganchos.AntesDaFerramenta{Ferramenta: "reservar"})

	if r.Decisao.Tipo != ganchos.Bloquear || r.Decisao.Motivo != "fora do escopo" {
		t.Errorf("Decisao = %+v, want o bloqueio do fixo", r.Decisao)
	}
	if !r.Decisao.Origem.Fixo || r.Decisao.Origem.Gancho != "escopo" {
		t.Errorf("Origem = %+v, want o fixo escopo, carimbado pela fase", r.Decisao.Origem)
	}
	if len(escopo.Eventos()) != 1 {
		t.Errorf("fixo recebeu %d eventos, want 1", len(escopo.Eventos()))
	}
	if len(regra.Eventos()) != 0 {
		t.Errorf("depois do bloqueio a regra rodou %d vezes, want 0", len(regra.Eventos()))
	}
	if len(mascara.Eventos()) != 1 {
		t.Errorf("o saneamento rodou %d vezes depois do bloqueio, want 1 (sempre roda)", len(mascara.Eventos()))
	}
}

// contextoB2C é o fixo de checagem que fixa contexto, permissão e escrita.
var contextoB2C = ganchos.Gancho[ganchos.AntesDaFerramenta]{
	Nome: "contexto_b2c",
	Avaliar: func(e ganchos.AntesDaFerramenta) (ganchos.AntesDaFerramenta, ganchos.Decisao) {
		e.Params = map[string]any{"companyId": "certo"}
		e.ContextoB2C = map[string]any{"condominio": "c1"}
		e.Permitida, e.Escrita = true, true
		return e, ganchos.Decisao{Tipo: ganchos.Ajustar}
	},
}

type afrouxar func(ganchos.AntesDaFerramenta) ganchos.AntesDaFerramenta

func regraQueAjusta(f afrouxar, fixo bool, p ganchos.Politica) ganchos.Gancho[ganchos.AntesDaFerramenta] {
	return ganchos.Gancho[ganchos.AntesDaFerramenta]{
		Nome: "regra", Fixo: fixo, Falha: p,
		Avaliar: func(e ganchos.AntesDaFerramenta) (ganchos.AntesDaFerramenta, ganchos.Decisao) {
			return f(e), ganchos.Decisao{Tipo: ganchos.Ajustar}
		},
	}
}

var regraQuePanica = ganchos.Gancho[ganchos.AntesDaFerramenta]{
	Nome: "regra-quebrada", Falha: ganchos.FechadaSeGrava,
	Avaliar: func(ganchos.AntesDaFerramenta) (ganchos.AntesDaFerramenta, ganchos.Decisao) { panic("quebrou") },
}

func TestRegraNaoDesfazOFixo(t *testing.T) {
	trocarCompany := func(e ganchos.AntesDaFerramenta) ganchos.AntesDaFerramenta {
		e.Params = map[string]any{"companyId": "outro"}
		return e
	}
	tests := []struct {
		name          string
		regras        []ganchos.Gancho[ganchos.AntesDaFerramenta]
		wantTipo      ganchos.TipoDecisao
		wantRegistros int
	}{
		{"troca o companyId dos params", []ganchos.Gancho[ganchos.AntesDaFerramenta]{regraQueAjusta(trocarCompany, false, ganchos.Aberta)}, ganchos.Ajustar, 2},
		{"troca o contexto b2c", []ganchos.Gancho[ganchos.AntesDaFerramenta]{regraQueAjusta(func(e ganchos.AntesDaFerramenta) ganchos.AntesDaFerramenta {
			e.ContextoB2C = map[string]any{"condominio": "c9"}
			return e
		}, false, ganchos.Aberta)}, ganchos.Ajustar, 2},
		{"troca a ferramenta", []ganchos.Gancho[ganchos.AntesDaFerramenta]{regraQueAjusta(func(e ganchos.AntesDaFerramenta) ganchos.AntesDaFerramenta {
			e.Ferramenta = "apagar_tudo"
			return e
		}, false, ganchos.Aberta)}, ganchos.Ajustar, 2},
		{"muda a permissao", []ganchos.Gancho[ganchos.AntesDaFerramenta]{regraQueAjusta(func(e ganchos.AntesDaFerramenta) ganchos.AntesDaFerramenta {
			e.Permitida = false
			return e
		}, false, ganchos.Aberta)}, ganchos.Ajustar, 2},
		{"troca o condominio", []ganchos.Gancho[ganchos.AntesDaFerramenta]{regraQueAjusta(func(e ganchos.AntesDaFerramenta) ganchos.AntesDaFerramenta {
			e.CondominioID = "c9"
			return e
		}, false, ganchos.Aberta)}, ganchos.Ajustar, 2},
		{"troca a identidade", []ganchos.Gancho[ganchos.AntesDaFerramenta]{regraQueAjusta(func(e ganchos.AntesDaFerramenta) ganchos.AntesDaFerramenta {
			e.Identidade = ganchos.Identidade{Cadastrado: true, Papeis: []string{"sindico"}}
			return e
		}, false, ganchos.Aberta)}, ganchos.Ajustar, 2},
		{"regra que se diz fixa tambem nao ajusta", []ganchos.Gancho[ganchos.AntesDaFerramenta]{regraQueAjusta(trocarCompany, true, ganchos.Aberta)}, ganchos.Ajustar, 2},
		{"regra fechada que ajusta bloqueia", []ganchos.Gancho[ganchos.AntesDaFerramenta]{regraQueAjusta(trocarCompany, false, ganchos.Fechada)}, ganchos.Bloquear, 2},
		{"zera a escrita e a falha seguinte ainda fecha", []ganchos.Gancho[ganchos.AntesDaFerramenta]{
			regraQueAjusta(func(e ganchos.AntesDaFerramenta) ganchos.AntesDaFerramenta {
				e.Escrita = false
				return e
			}, false, ganchos.Aberta),
			regraQuePanica,
		}, ganchos.Bloquear, 3},
		{"escrita fixada pela checagem fecha a falha da regra", []ganchos.Gancho[ganchos.AntesDaFerramenta]{regraQuePanica}, ganchos.Bloquear, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := ganchos.Fixos{AntesDaFerramenta: ganchos.Pontos[ganchos.AntesDaFerramenta]{
				Checagem: []ganchos.Gancho[ganchos.AntesDaFerramenta]{contextoB2C},
			}}
			idx := &ganchos.Indice{AntesDaFerramenta: ganchos.Nivel[ganchos.AntesDaFerramenta]{Globais: tt.regras}}
			ev := ganchos.AntesDaFerramenta{Ferramenta: "reservar", CondominioID: "c1", Escrita: false}

			r := ganchostest.Conjunto(f, idx).Turno().AntesDaFerramenta(context.Background(), ev)

			if r.Decisao.Tipo != tt.wantTipo {
				t.Errorf("Decisao = %+v, want %q", r.Decisao, tt.wantTipo)
			}
			if r.Evento.Params["companyId"] != "certo" || r.Evento.ContextoB2C["condominio"] != "c1" ||
				r.Evento.Ferramenta != "reservar" || !r.Evento.Permitida || r.Evento.CondominioID != "c1" ||
				r.Evento.Identidade.Cadastrado || !r.Evento.Escrita {
				t.Errorf("Evento = %+v, want o que o fixo fixou", r.Evento)
			}
			if len(r.Registros) != tt.wantRegistros {
				t.Fatalf("Registros = %+v, want %d", r.Registros, tt.wantRegistros)
			}
			falha := r.Registros[1]
			if !falha.Falhou || falha.Decisao.Origem.Fixo {
				t.Errorf("Registro da regra = %+v, want falha de não fixo", falha)
			}
		})
	}
}

func TestRegraNaoSeguraTransbordo(t *testing.T) {
	tests := []struct {
		name    string
		decisao ganchos.Decisao
	}{
		{"bloquear", ganchos.Decisao{Tipo: ganchos.Bloquear, Motivo: "fica aqui"}},
		{"confirmar", ganchos.Decisao{Tipo: ganchos.Confirmar, Motivo: "posso transbordar?"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			regra := &ganchostest.Roteiro[ganchos.TransbordoOuEncerramento]{Decisao: tt.decisao}
			idx := &ganchos.Indice{TransbordoOuEncerramento: ganchos.Nivel[ganchos.TransbordoOuEncerramento]{
				Globais: []ganchos.Gancho[ganchos.TransbordoOuEncerramento]{ganchostest.Gancho("regra", regra)},
			}}

			r := ganchostest.Conjunto(ganchos.Fixos{}, idx).Turno().TransbordoOuEncerramento(context.Background(), ganchos.TransbordoOuEncerramento{})

			if r.Decisao.Tipo != ganchos.Deixar || len(r.Perguntas) != 0 {
				t.Errorf("Decisao = %+v, Perguntas = %v, want deixar sem pergunta", r.Decisao, r.Perguntas)
			}
			if len(r.Registros) != 1 || r.Registros[0].Causa != ganchos.CausaDecisaoInvalida {
				t.Errorf("Registros = %+v, want a regra como decisão inválida", r.Registros)
			}
		})
	}
}

func TestTransbordoCanceladoSaiMascarado(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	mascarar := ganchos.Gancho[ganchos.TransbordoOuEncerramento]{
		Nome: "transbordo_sem_pii", Falha: ganchos.Aberta,
		Avaliar: func(e ganchos.TransbordoOuEncerramento) (ganchos.TransbordoOuEncerramento, ganchos.Decisao) {
			e.Motivo = strings.ReplaceAll(e.Motivo, "123.456.789-09", "«cpf:1»")
			return e, ganchos.Decisao{Tipo: ganchos.Ajustar, Quantidade: 1}
		},
	}
	regra := &ganchostest.Roteiro[ganchos.TransbordoOuEncerramento]{}
	f := ganchos.Fixos{TransbordoOuEncerramento: ganchos.Pontos[ganchos.TransbordoOuEncerramento]{
		Saneamento: []ganchos.Gancho[ganchos.TransbordoOuEncerramento]{mascarar},
	}}
	idx := &ganchos.Indice{TransbordoOuEncerramento: ganchos.Nivel[ganchos.TransbordoOuEncerramento]{
		Globais: []ganchos.Gancho[ganchos.TransbordoOuEncerramento]{ganchostest.Gancho("regra", regra)},
	}}

	r := ganchostest.Conjunto(f, idx).Turno().TransbordoOuEncerramento(ctx, ganchos.TransbordoOuEncerramento{Motivo: "morador 123.456.789-09 pediu"})

	if r.Evento.Motivo != "morador «cpf:1» pediu" {
		t.Errorf("Motivo = %q, want mascarado mesmo com o contexto cancelado", r.Evento.Motivo)
	}
	if r.Decisao.Tipo != ganchos.Ajustar {
		t.Errorf("Decisao = %+v, want o ajuste da máscara: o transbordo segue", r.Decisao)
	}
	if len(regra.Eventos()) != 0 {
		t.Error("regra rodou com o contexto cancelado")
	}
	if len(r.Registros) != 2 || r.Registros[0].Causa != ganchos.CausaCancelado || r.Registros[0].Decisao.Origem.Gancho != "regra" {
		t.Errorf("Registros = %+v, want a regra cancelada e o ajuste da máscara", r.Registros)
	}
}

func TestTurnoVeUmRetratoSo(t *testing.T) {
	bloqueio := &ganchostest.Roteiro[ganchos.AntesDeResponder]{
		Decisao: ganchos.Decisao{Tipo: ganchos.Bloquear, Motivo: "nova regra"},
	}
	antigo := &ganchos.Indice{}
	novo := &ganchos.Indice{AntesDeResponder: ganchos.Nivel[ganchos.AntesDeResponder]{
		Globais: []ganchos.Gancho[ganchos.AntesDeResponder]{ganchostest.Gancho("nova", bloqueio)},
	}}
	atual := antigo
	c := ganchos.NovoConjunto(ganchos.Fixos{}, func() *ganchos.Indice { return atual })
	ctx := context.Background()

	turno := c.Turno()
	antes := turno.AntesDeResponder(ctx, ganchos.AntesDeResponder{})
	atual = novo // regra salva no meio do turno
	depois := turno.AntesDeResponder(ctx, ganchos.AntesDeResponder{})
	proximo := c.Turno().AntesDeResponder(ctx, ganchos.AntesDeResponder{})

	if antes.Decisao.Tipo != ganchos.Deixar || depois.Decisao.Tipo != ganchos.Deixar {
		t.Errorf("no mesmo turno: antes %q, depois %q, want deixar nos dois", antes.Decisao.Tipo, depois.Decisao.Tipo)
	}
	if proximo.Decisao.Tipo != ganchos.Bloquear {
		t.Errorf("próximo turno = %q, want a regra nova", proximo.Decisao.Tipo)
	}
}

func TestConjuntoCopiaECarimbaOsFixos(t *testing.T) {
	bloqueia := &ganchostest.Roteiro[ganchos.AntesDeResponder]{
		Decisao: ganchos.Decisao{Tipo: ganchos.Bloquear, Motivo: "não"},
	}
	checagem := []ganchos.Gancho[ganchos.AntesDeResponder]{ganchostest.Gancho("fixo", bloqueia)}
	c := ganchos.NovoConjunto(ganchos.Fixos{AntesDeResponder: ganchos.Pontos[ganchos.AntesDeResponder]{Checagem: checagem}}, nil)

	checagem[0] = ganchostest.Gancho("trocado", &ganchostest.Roteiro[ganchos.AntesDeResponder]{})
	r := c.Turno().AntesDeResponder(context.Background(), ganchos.AntesDeResponder{})

	if r.Decisao.Tipo != ganchos.Bloquear || r.Decisao.Origem.Gancho != "fixo" || !r.Decisao.Origem.Fixo {
		t.Errorf("Decisao = %+v, want o fixo original, marcado como fixo", r.Decisao)
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

			r := ganchos.NovoConjunto(f, tt.regras).Turno().AntesDeResponder(context.Background(), ganchos.AntesDeResponder{Texto: "oi"})

			if r.Decisao.Tipo != ganchos.Deixar || len(fixo.Eventos()) != 1 {
				t.Errorf("Decisao = %+v, fixo recebeu %d, want deixar e 1", r.Decisao, len(fixo.Eventos()))
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

func TestSaneamentoDepoisDoBloqueioMantemOPrimeiroMotivo(t *testing.T) {
	bloqueio := ganchos.Gancho[ganchos.AntesDaFerramenta]{
		Nome: "escopo", Falha: ganchos.Fechada,
		Avaliar: func(e ganchos.AntesDaFerramenta) (ganchos.AntesDaFerramenta, ganchos.Decisao) {
			return e, ganchos.Decisao{Tipo: ganchos.Bloquear, Motivo: "fora do escopo"}
		},
	}
	saneamentos := map[string]ganchos.Gancho[ganchos.AntesDaFerramenta]{
		"falha": {Nome: "mascara", Falha: ganchos.Fechada,
			Avaliar: func(ganchos.AntesDaFerramenta) (ganchos.AntesDaFerramenta, ganchos.Decisao) { panic("quebrou") }},
		"ajusta": {Nome: "mascara", Falha: ganchos.Fechada,
			Avaliar: func(e ganchos.AntesDaFerramenta) (ganchos.AntesDaFerramenta, ganchos.Decisao) {
				return e, ganchos.Decisao{Tipo: ganchos.Ajustar, Quantidade: 1}
			}},
	}
	for nome, saneamento := range saneamentos {
		t.Run(nome, func(t *testing.T) {
			f := ganchos.Fixos{AntesDaFerramenta: ganchos.Pontos[ganchos.AntesDaFerramenta]{
				Checagem:   []ganchos.Gancho[ganchos.AntesDaFerramenta]{bloqueio},
				Saneamento: []ganchos.Gancho[ganchos.AntesDaFerramenta]{saneamento},
			}}
			r := ganchostest.Conjunto(f, &ganchos.Indice{}).Turno().AntesDaFerramenta(context.Background(), ganchos.AntesDaFerramenta{Ferramenta: "reservar"})
			if r.Decisao.Tipo != ganchos.Bloquear || r.Decisao.Motivo != "fora do escopo" || r.Decisao.Origem.Gancho != "escopo" {
				t.Errorf("Decisao = %+v, want o bloqueio do escopo", r.Decisao)
			}
		})
	}
}

func TestComObservadorEmSequenciaAvisaTodosNaOrdem(t *testing.T) {
	var ordem []string
	observar := func(nome string) ganchos.Opcao {
		return ganchos.ComObservador(func(context.Context, ganchos.Observacao) { ordem = append(ordem, nome) })
	}
	c := ganchos.NovoConjunto(ganchos.Fixos{}, nil, observar("log"), observar("auditoria"))

	c.Turno().AntesDeResponder(context.Background(), ganchos.AntesDeResponder{})

	if !slices.Equal(ordem, []string{"log", "auditoria"}) {
		t.Fatalf("observadores avisados = %v, want [log auditoria]", ordem)
	}
}
