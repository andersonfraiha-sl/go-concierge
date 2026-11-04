package ganchos_test

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"go-concierge/internal/ganchos"
)

// passo descreve um gancho fixo de teste: anota o nome em chamados e, ao ajustar,
// acrescenta sufixo ao texto.
type passo struct {
	nome   string
	tipo   ganchos.TipoDecisao
	motivo string
	sufixo string
	regra  bool
}

func montar(passos []passo, chamados *[]string) []ganchos.Gancho[ganchos.AntesDeResponder] {
	gs := make([]ganchos.Gancho[ganchos.AntesDeResponder], 0, len(passos))
	for _, p := range passos {
		gs = append(gs, ganchos.Gancho[ganchos.AntesDeResponder]{
			Nome:  p.nome,
			Fixo:  !p.regra,
			Falha: ganchos.Fechada,
			Avaliar: func(e ganchos.AntesDeResponder) (ganchos.AntesDeResponder, ganchos.Decisao) {
				*chamados = append(*chamados, p.nome)
				e.Texto += p.sufixo
				return e, ganchos.Decisao{Tipo: p.tipo, Motivo: p.motivo}
			},
		})
	}
	return gs
}

func TestAvaliarComposicao(t *testing.T) {
	tests := []struct {
		name          string
		passos        []passo
		wantChamados  []string
		wantTipo      ganchos.TipoDecisao
		wantMotivo    string
		wantOrigem    string
		wantTexto     string
		wantPerguntas []string
		wantRegistros int
		wantCausa     ganchos.Causa
	}{
		{
			name:      "sem ganchos deixa",
			wantTipo:  ganchos.Deixar,
			wantTexto: "oi",
		},
		{
			name:         "decisao vazia vale deixar",
			passos:       []passo{{nome: "a"}},
			wantChamados: []string{"a"},
			wantTipo:     ganchos.Deixar,
			wantTexto:    "oi",
		},
		{
			name: "bloquear encerra e os seguintes nao recebem o evento",
			passos: []passo{
				{nome: "a", tipo: ganchos.Deixar},
				{nome: "b", tipo: ganchos.Bloquear, motivo: "não pode"},
				{nome: "c", tipo: ganchos.Ajustar, sufixo: "!"},
			},
			wantChamados:  []string{"a", "b"},
			wantTipo:      ganchos.Bloquear,
			wantMotivo:    "não pode",
			wantOrigem:    "b",
			wantTexto:     "oi",
			wantRegistros: 1,
		},
		{
			name: "ajustar encadeia",
			passos: []passo{
				{nome: "a", tipo: ganchos.Ajustar, sufixo: " 1"},
				{nome: "b", tipo: ganchos.Ajustar, sufixo: " 2"},
			},
			wantChamados:  []string{"a", "b"},
			wantTipo:      ganchos.Ajustar,
			wantOrigem:    "a",
			wantTexto:     "oi 1 2",
			wantRegistros: 2,
		},
		{
			name:         "deixar nao adota o evento devolvido",
			passos:       []passo{{nome: "a", tipo: ganchos.Deixar, sufixo: " ignorado"}},
			wantChamados: []string{"a"},
			wantTipo:     ganchos.Deixar,
			wantTexto:    "oi",
		},
		{
			name: "duas perguntas viram Perguntas na ordem",
			passos: []passo{
				{nome: "a", tipo: ganchos.Confirmar, motivo: "p1"},
				{nome: "b", tipo: ganchos.Deixar},
				{nome: "c", tipo: ganchos.Confirmar, motivo: "p2"},
			},
			wantChamados:  []string{"a", "b", "c"},
			wantTipo:      ganchos.Confirmar,
			wantMotivo:    "p1",
			wantOrigem:    "a",
			wantTexto:     "oi",
			wantPerguntas: []string{"p1", "p2"},
			wantRegistros: 2,
		},
		{
			name: "confirmar vence ajustar",
			passos: []passo{
				{nome: "a", tipo: ganchos.Ajustar, sufixo: " 1"},
				{nome: "b", tipo: ganchos.Confirmar, motivo: "p1"},
			},
			wantChamados:  []string{"a", "b"},
			wantTipo:      ganchos.Confirmar,
			wantMotivo:    "p1",
			wantOrigem:    "b",
			wantTexto:     "oi 1",
			wantPerguntas: []string{"p1"},
			wantRegistros: 2,
		},
		{
			name: "bloquear vence confirmar",
			passos: []passo{
				{nome: "a", tipo: ganchos.Confirmar, motivo: "p1"},
				{nome: "b", tipo: ganchos.Ajustar, sufixo: " 1"},
				{nome: "c", tipo: ganchos.Bloquear, motivo: "x"},
			},
			wantChamados:  []string{"a", "b", "c"},
			wantTipo:      ganchos.Bloquear,
			wantMotivo:    "x",
			wantOrigem:    "c",
			wantTexto:     "oi 1",
			wantPerguntas: []string{"p1"},
			wantRegistros: 3,
		},
		{
			name:          "decisao desconhecida conta como falha",
			passos:        []passo{{nome: "a", tipo: "liberar"}, {nome: "b"}},
			wantChamados:  []string{"a"},
			wantTipo:      ganchos.Bloquear,
			wantMotivo:    ganchos.MotivoDeFalha,
			wantOrigem:    "a",
			wantTexto:     "oi",
			wantRegistros: 1,
			wantCausa:     ganchos.CausaDecisaoInvalida,
		},
		{
			name:          "bloquear sem motivo conta como falha",
			passos:        []passo{{nome: "a", tipo: ganchos.Bloquear}},
			wantChamados:  []string{"a"},
			wantTipo:      ganchos.Bloquear,
			wantMotivo:    ganchos.MotivoDeFalha,
			wantOrigem:    "a",
			wantTexto:     "oi",
			wantRegistros: 1,
			wantCausa:     ganchos.CausaDecisaoInvalida,
		},
		{
			name:          "regra que ajusta conta como falha e o ajuste nao vale",
			passos:        []passo{{nome: "r", tipo: ganchos.Ajustar, sufixo: " solto", regra: true}},
			wantChamados:  []string{"r"},
			wantTipo:      ganchos.Bloquear,
			wantMotivo:    ganchos.MotivoDeFalha,
			wantOrigem:    "r",
			wantTexto:     "oi",
			wantRegistros: 1,
			wantCausa:     ganchos.CausaDecisaoInvalida,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var chamados []string
			r := ganchos.Avaliar(context.Background(), montar(tt.passos, &chamados), ganchos.AntesDeResponder{Texto: "oi"})

			if !slices.Equal(chamados, tt.wantChamados) {
				t.Errorf("chamados = %v, want %v", chamados, tt.wantChamados)
			}
			if r.Decisao.Tipo != tt.wantTipo || r.Decisao.Motivo != tt.wantMotivo {
				t.Errorf("Decisao = %+v, want %s %q", r.Decisao, tt.wantTipo, tt.wantMotivo)
			}
			if r.Decisao.Origem.Gancho != tt.wantOrigem {
				t.Errorf("Origem.Gancho = %q, want %q", r.Decisao.Origem.Gancho, tt.wantOrigem)
			}
			if r.Evento.Texto != tt.wantTexto {
				t.Errorf("Texto = %q, want %q", r.Evento.Texto, tt.wantTexto)
			}
			if !slices.Equal(r.Perguntas, tt.wantPerguntas) {
				t.Errorf("Perguntas = %v, want %v", r.Perguntas, tt.wantPerguntas)
			}
			if len(r.Registros) != tt.wantRegistros {
				t.Fatalf("Registros = %d, want %d: %+v", len(r.Registros), tt.wantRegistros, r.Registros)
			}
			for _, reg := range r.Registros {
				if reg.Evento != ganchos.EventoAntesDeResponder {
					t.Errorf("Registro.Evento = %q, want %q", reg.Evento, ganchos.EventoAntesDeResponder)
				}
			}
			if tt.wantCausa != "" && r.Registros[0].Causa != tt.wantCausa {
				t.Errorf("Causa = %q, want %q", r.Registros[0].Causa, tt.wantCausa)
			}
		})
	}
}

type modoDeFalha string

const (
	comPanic        modoDeFalha = "panic"
	comPrazoPerdido modoDeFalha = "prazo"
)

// falhando é uma regra que ajusta a ferramenta e depois falha do jeito pedido.
func falhando(modo modoDeFalha, p ganchos.Politica) ganchos.Gancho[ganchos.AntesDaFerramenta] {
	return ganchos.Gancho[ganchos.AntesDaFerramenta]{
		Nome:  "falho",
		Prazo: time.Millisecond,
		Falha: p,
		Avaliar: func(e ganchos.AntesDaFerramenta) (ganchos.AntesDaFerramenta, ganchos.Decisao) {
			e.Ferramenta = "trocada"
			if modo == comPanic {
				panic("quebrou")
			}
			time.Sleep(10 * time.Millisecond)
			return e, ganchos.Decisao{Tipo: ganchos.Deixar}
		},
	}
}

func TestAvaliarPoliticaDeFalha(t *testing.T) {
	tests := []struct {
		name         string
		modo         modoDeFalha
		politica     ganchos.Politica
		escrita      bool
		wantTipo     ganchos.TipoDecisao
		wantSeguinte bool
		wantPolitica ganchos.Politica
		wantCausa    ganchos.Causa
	}{
		{"panic fechada", comPanic, ganchos.Fechada, false, ganchos.Bloquear, false, ganchos.Fechada, ganchos.CausaPanic},
		{"panic aberta", comPanic, ganchos.Aberta, true, ganchos.Deixar, true, ganchos.Aberta, ganchos.CausaPanic},
		{"panic fechada se grava com escrita", comPanic, ganchos.FechadaSeGrava, true, ganchos.Bloquear, false, ganchos.FechadaSeGrava, ganchos.CausaPanic},
		{"panic fechada se grava sem escrita", comPanic, ganchos.FechadaSeGrava, false, ganchos.Deixar, true, ganchos.FechadaSeGrava, ganchos.CausaPanic},
		{"panic sem politica fecha", comPanic, "", false, ganchos.Bloquear, false, ganchos.Fechada, ganchos.CausaPanic},
		{"prazo fechada", comPrazoPerdido, ganchos.Fechada, false, ganchos.Bloquear, false, ganchos.Fechada, ganchos.CausaPrazo},
		{"prazo aberta", comPrazoPerdido, ganchos.Aberta, true, ganchos.Deixar, true, ganchos.Aberta, ganchos.CausaPrazo},
		{"prazo fechada se grava com escrita", comPrazoPerdido, ganchos.FechadaSeGrava, true, ganchos.Bloquear, false, ganchos.FechadaSeGrava, ganchos.CausaPrazo},
		{"prazo fechada se grava sem escrita", comPrazoPerdido, ganchos.FechadaSeGrava, false, ganchos.Deixar, true, ganchos.FechadaSeGrava, ganchos.CausaPrazo},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seguinteRodou := false
			seguinte := ganchos.Gancho[ganchos.AntesDaFerramenta]{
				Nome: "seguinte",
				Avaliar: func(e ganchos.AntesDaFerramenta) (ganchos.AntesDaFerramenta, ganchos.Decisao) {
					seguinteRodou = true
					return e, ganchos.Decisao{Tipo: ganchos.Deixar}
				},
			}
			gs := []ganchos.Gancho[ganchos.AntesDaFerramenta]{falhando(tt.modo, tt.politica), seguinte}
			ev := ganchos.AntesDaFerramenta{Ferramenta: "reservar", Escrita: tt.escrita}

			r := ganchos.Avaliar(context.Background(), gs, ev)

			if r.Decisao.Tipo != tt.wantTipo {
				t.Errorf("Decisao.Tipo = %q, want %q", r.Decisao.Tipo, tt.wantTipo)
			}
			if tt.wantTipo == ganchos.Bloquear && r.Decisao.Motivo != ganchos.MotivoDeFalha {
				t.Errorf("Decisao.Motivo = %q, want %q", r.Decisao.Motivo, ganchos.MotivoDeFalha)
			}
			if seguinteRodou != tt.wantSeguinte {
				t.Errorf("seguinte rodou = %v, want %v", seguinteRodou, tt.wantSeguinte)
			}
			if r.Evento.Ferramenta != "reservar" {
				t.Errorf("Ferramenta = %q: o que um gancho que falhou devolveu não vale", r.Evento.Ferramenta)
			}
			if len(r.Registros) != 1 {
				t.Fatalf("Registros = %d, want 1", len(r.Registros))
			}
			reg := r.Registros[0]
			if !reg.Falhou || reg.Decisao.Tipo != tt.wantTipo || reg.Decisao.Origem.Gancho != "falho" {
				t.Errorf("Registro = %+v, want falha de %q com %q", reg, "falho", tt.wantTipo)
			}
			if reg.Politica != tt.wantPolitica || reg.Causa != tt.wantCausa {
				t.Errorf("Politica, Causa = %q, %q, want %q, %q", reg.Politica, reg.Causa, tt.wantPolitica, tt.wantCausa)
			}
			if reg.Evento != ganchos.EventoAntesDaFerramenta {
				t.Errorf("Registro.Evento = %q", reg.Evento)
			}
		})
	}
}

func TestAvaliarPrazoPadrao(t *testing.T) {
	lento := ganchos.Gancho[ganchos.AntesDeResponder]{
		Nome: "lento",
		Avaliar: func(e ganchos.AntesDeResponder) (ganchos.AntesDeResponder, ganchos.Decisao) {
			time.Sleep(4 * ganchos.PrazoPadrao)
			return e, ganchos.Decisao{Tipo: ganchos.Deixar}
		},
	}
	r := ganchos.Avaliar(context.Background(), []ganchos.Gancho[ganchos.AntesDeResponder]{lento}, ganchos.AntesDeResponder{})
	if r.Decisao.Tipo != ganchos.Bloquear || len(r.Registros) != 1 || r.Registros[0].Causa != ganchos.CausaPrazo {
		t.Fatalf("Resultado = %+v, want falha fechada pelo prazo padrão", r)
	}
	if r.Registros[0].Politica != ganchos.Fechada {
		t.Errorf("Politica = %q, want %q", r.Registros[0].Politica, ganchos.Fechada)
	}
}

func TestAvaliarFixoLentoRodaDeNovo(t *testing.T) {
	tests := []struct {
		name          string
		fixo          bool
		lentas        int // quantas execuções iniciais passam do prazo
		wantExecucoes int
		wantTipo      ganchos.TipoDecisao
		wantLentos    int
	}{
		{"fixo lento uma vez passa na segunda", true, 1, 2, ganchos.Deixar, 1},
		{"fixo lento duas vezes falha", true, 2, 2, ganchos.Bloquear, 1},
		{"fixo rapido roda uma vez", true, 0, 1, ganchos.Deixar, 0},
		{"regra lenta falha sem segunda chance", false, 1, 1, ganchos.Bloquear, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			execucoes := 0
			g := ganchos.Gancho[ganchos.AntesDeResponder]{
				Nome: "medido", Fixo: tt.fixo, Prazo: time.Millisecond,
				Avaliar: func(e ganchos.AntesDeResponder) (ganchos.AntesDeResponder, ganchos.Decisao) {
					execucoes++
					if execucoes <= tt.lentas {
						time.Sleep(10 * time.Millisecond)
					}
					return e, ganchos.Decisao{Tipo: ganchos.Deixar}
				},
			}

			r := ganchos.Avaliar(context.Background(), []ganchos.Gancho[ganchos.AntesDeResponder]{g}, ganchos.AntesDeResponder{})

			if execucoes != tt.wantExecucoes || r.Decisao.Tipo != tt.wantTipo || r.Lentos != tt.wantLentos {
				t.Errorf("execuções, decisão, lentos = %d, %q, %d, want %d, %q, %d",
					execucoes, r.Decisao.Tipo, r.Lentos, tt.wantExecucoes, tt.wantTipo, tt.wantLentos)
			}
		})
	}
}

func TestAvaliarDetalheDoPanic(t *testing.T) {
	tests := []struct {
		name       string
		panicar    func()
		wantContem []string
		wantSem    string
	}{
		{
			name:       "erro do runtime leva a mensagem e o local",
			panicar:    func() { var m map[string]string; m["x"] = "y" },
			wantContem: []string{"assignment to entry in nil map", "ganchos_test.go"},
		},
		{
			name:       "valor arbitrario leva so o tipo, nunca o valor",
			panicar:    func() { panic("cpf 123.456.789-09") },
			wantContem: []string{"string", "ganchos_test.go"},
			wantSem:    "123.456.789-09",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := ganchos.Gancho[ganchos.AntesDeResponder]{
				Nome: "quebrado", Falha: ganchos.Aberta,
				Avaliar: func(e ganchos.AntesDeResponder) (ganchos.AntesDeResponder, ganchos.Decisao) {
					tt.panicar()
					return e, ganchos.Decisao{}
				},
			}

			r := ganchos.Avaliar(context.Background(), []ganchos.Gancho[ganchos.AntesDeResponder]{g}, ganchos.AntesDeResponder{})

			if len(r.Registros) != 1 || r.Registros[0].Causa != ganchos.CausaPanic {
				t.Fatalf("Registros = %+v, want uma falha por panic", r.Registros)
			}
			detalhe := r.Registros[0].Detalhe
			for _, trecho := range tt.wantContem {
				if !strings.Contains(detalhe, trecho) {
					t.Errorf("Detalhe = %q, want conter %q", detalhe, trecho)
				}
			}
			if tt.wantSem != "" && strings.Contains(detalhe, tt.wantSem) {
				t.Errorf("Detalhe = %q vazou o valor do panic", detalhe)
			}
		})
	}
}

func TestAvaliarContextoCancelado(t *testing.T) {
	tests := []struct {
		name           string
		cancelarAntes  bool
		politicaDoB    ganchos.Politica
		wantChamados   []string
		wantTipo       ganchos.TipoDecisao
		wantRegistros  int
		wantOrigemFalh string
	}{
		{"cancelado no meio, restante fechado bloqueia", false, ganchos.Fechada, []string{"a"}, ganchos.Bloquear, 1, "b"},
		{"cancelado no meio, restante aberto segue", false, ganchos.Aberta, []string{"a"}, ganchos.Deixar, 1, "b"},
		{"cancelado antes nao roda nenhum", true, ganchos.Aberta, nil, ganchos.Deixar, 2, "a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tt.cancelarAntes {
				cancel()
			}
			var chamados []string
			anotar := func(nome string, p ganchos.Politica) ganchos.Gancho[ganchos.AntesDeResponder] {
				return ganchos.Gancho[ganchos.AntesDeResponder]{
					Nome: nome, Falha: p,
					Avaliar: func(e ganchos.AntesDeResponder) (ganchos.AntesDeResponder, ganchos.Decisao) {
						chamados = append(chamados, nome)
						cancel()
						return e, ganchos.Decisao{Tipo: ganchos.Deixar}
					},
				}
			}
			gs := []ganchos.Gancho[ganchos.AntesDeResponder]{anotar("a", ganchos.Aberta), anotar("b", tt.politicaDoB)}

			r := ganchos.Avaliar(ctx, gs, ganchos.AntesDeResponder{})

			if !slices.Equal(chamados, tt.wantChamados) {
				t.Errorf("chamados = %v, want %v", chamados, tt.wantChamados)
			}
			if r.Decisao.Tipo != tt.wantTipo {
				t.Errorf("Decisao = %+v, want %q", r.Decisao, tt.wantTipo)
			}
			if len(r.Registros) != tt.wantRegistros {
				t.Fatalf("Registros = %+v, want %d", r.Registros, tt.wantRegistros)
			}
			reg := r.Registros[0]
			if reg.Causa != ganchos.CausaCancelado || reg.Decisao.Origem.Gancho != tt.wantOrigemFalh {
				t.Errorf("Registro = %+v, want cancelado com origem %q", reg, tt.wantOrigemFalh)
			}
		})
	}
}

func TestFixoRodaComContextoCancelado(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rodou := false
	fixo := ganchos.Gancho[ganchos.AntesDeResponder]{
		Nome: "mascara", Fixo: true,
		Avaliar: func(e ganchos.AntesDeResponder) (ganchos.AntesDeResponder, ganchos.Decisao) {
			rodou = true
			return e, ganchos.Decisao{}
		},
	}
	regra := ganchos.Gancho[ganchos.AntesDeResponder]{
		Nome: "regra", Falha: ganchos.Aberta,
		Avaliar: func(e ganchos.AntesDeResponder) (ganchos.AntesDeResponder, ganchos.Decisao) {
			t.Error("regra rodou com o contexto cancelado")
			return e, ganchos.Decisao{}
		},
	}

	r := ganchos.Avaliar(ctx, []ganchos.Gancho[ganchos.AntesDeResponder]{regra, fixo}, ganchos.AntesDeResponder{})

	if !rodou {
		t.Error("fixo não rodou com o contexto cancelado")
	}
	if r.Decisao.Tipo != ganchos.Deixar || len(r.Registros) != 1 || r.Registros[0].Causa != ganchos.CausaCancelado {
		t.Errorf("Resultado = %+v, want só a regra registrada como cancelada", r)
	}
}

func TestTransbordoSempreSegue(t *testing.T) {
	cancelado, cancel := context.WithCancel(context.Background())
	cancel()
	tests := []struct {
		name      string
		ctx       context.Context
		regra     bool
		avaliar   func(ganchos.TransbordoOuEncerramento) (ganchos.TransbordoOuEncerramento, ganchos.Decisao)
		wantCausa ganchos.Causa
	}{
		{
			name: "bloquear vira decisao invalida",
			ctx:  context.Background(),
			avaliar: func(e ganchos.TransbordoOuEncerramento) (ganchos.TransbordoOuEncerramento, ganchos.Decisao) {
				return e, ganchos.Decisao{Tipo: ganchos.Bloquear, Motivo: "fica"}
			},
			wantCausa: ganchos.CausaDecisaoInvalida,
		},
		{
			name: "confirmar vira decisao invalida",
			ctx:  context.Background(),
			avaliar: func(e ganchos.TransbordoOuEncerramento) (ganchos.TransbordoOuEncerramento, ganchos.Decisao) {
				return e, ganchos.Decisao{Tipo: ganchos.Confirmar, Motivo: "posso transbordar?"}
			},
			wantCausa: ganchos.CausaDecisaoInvalida,
		},
		{
			name: "panic em politica fechada",
			ctx:  context.Background(),
			avaliar: func(ganchos.TransbordoOuEncerramento) (ganchos.TransbordoOuEncerramento, ganchos.Decisao) {
				panic("quebrou")
			},
			wantCausa: ganchos.CausaPanic,
		},
		{
			name:  "regra com contexto cancelado",
			ctx:   cancelado,
			regra: true,
			avaliar: func(e ganchos.TransbordoOuEncerramento) (ganchos.TransbordoOuEncerramento, ganchos.Decisao) {
				return e, ganchos.Decisao{}
			},
			wantCausa: ganchos.CausaCancelado,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := ganchos.Gancho[ganchos.TransbordoOuEncerramento]{Nome: "g", Fixo: !tt.regra, Falha: ganchos.Fechada, Avaliar: tt.avaliar}

			r := ganchos.Avaliar(tt.ctx, []ganchos.Gancho[ganchos.TransbordoOuEncerramento]{g}, ganchos.TransbordoOuEncerramento{})

			if r.Decisao.Tipo != ganchos.Deixar {
				t.Errorf("Decisao = %+v, want deixar: o transbordo sempre segue", r.Decisao)
			}
			if len(r.Registros) != 1 || r.Registros[0].Causa != tt.wantCausa {
				t.Errorf("Registros = %+v, want uma falha %q", r.Registros, tt.wantCausa)
			}
		})
	}
}
