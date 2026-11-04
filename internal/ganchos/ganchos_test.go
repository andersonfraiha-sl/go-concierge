package ganchos_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"go-concierge/internal/ganchos"
)

// passo descreve um gancho de teste: anota o nome em chamados e, ao ajustar, acrescenta
// sufixo ao texto.
type passo struct {
	nome   string
	tipo   ganchos.TipoDecisao
	motivo string
	sufixo string
}

func montar(passos []passo, chamados *[]string) []ganchos.Gancho[ganchos.AntesDeResponder] {
	gs := make([]ganchos.Gancho[ganchos.AntesDeResponder], 0, len(passos))
	for _, p := range passos {
		gs = append(gs, ganchos.Gancho[ganchos.AntesDeResponder]{
			Nome: p.nome,
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
	}{
		{
			name:          "sem ganchos deixa",
			wantTipo:      ganchos.Deixar,
			wantTexto:     "oi",
			wantRegistros: 0,
		},
		{
			name:          "decisao vazia vale deixar",
			passos:        []passo{{nome: "a"}},
			wantChamados:  []string{"a"},
			wantTipo:      ganchos.Deixar,
			wantTexto:     "oi",
			wantRegistros: 0,
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
			name: "deixar nao adota o evento devolvido",
			passos: []passo{
				{nome: "a", tipo: ganchos.Deixar, sufixo: " ignorado"},
			},
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
			name: "decisao desconhecida conta como falha fechada",
			passos: []passo{
				{nome: "a", tipo: "liberar"},
				{nome: "b"},
			},
			wantChamados:  []string{"a"},
			wantTipo:      ganchos.Bloquear,
			wantMotivo:    ganchos.MotivoDeFalha,
			wantOrigem:    "a",
			wantTexto:     "oi",
			wantRegistros: 1,
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
				t.Errorf("Registros = %d, want %d: %+v", len(r.Registros), tt.wantRegistros, r.Registros)
			}
			for _, reg := range r.Registros {
				if reg.Evento != ganchos.EventoAntesDeResponder {
					t.Errorf("Registro.Evento = %q, want %q", reg.Evento, ganchos.EventoAntesDeResponder)
				}
			}
		})
	}
}

type modoDeFalha string

const (
	comPanic        modoDeFalha = "panic"
	comPrazoPerdido modoDeFalha = "prazo"
)

// falhando ajusta a ferramenta e depois falha do jeito pedido: o ajuste não pode valer.
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
			return e, ganchos.Decisao{Tipo: ganchos.Ajustar}
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
	}{
		{"panic fechada", comPanic, ganchos.Fechada, false, ganchos.Bloquear, false},
		{"panic aberta", comPanic, ganchos.Aberta, true, ganchos.Deixar, true},
		{"panic fechada se grava com escrita", comPanic, ganchos.FechadaSeGrava, true, ganchos.Bloquear, false},
		{"panic fechada se grava sem escrita", comPanic, ganchos.FechadaSeGrava, false, ganchos.Deixar, true},
		{"panic sem politica fecha", comPanic, "", false, ganchos.Bloquear, false},
		{"prazo fechada", comPrazoPerdido, ganchos.Fechada, false, ganchos.Bloquear, false},
		{"prazo aberta", comPrazoPerdido, ganchos.Aberta, true, ganchos.Deixar, true},
		{"prazo fechada se grava com escrita", comPrazoPerdido, ganchos.FechadaSeGrava, true, ganchos.Bloquear, false},
		{"prazo fechada se grava sem escrita", comPrazoPerdido, ganchos.FechadaSeGrava, false, ganchos.Deixar, true},
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
				t.Errorf("Ferramenta = %q: o ajuste de um gancho que falhou não vale", r.Evento.Ferramenta)
			}
			if len(r.Registros) != 1 {
				t.Fatalf("Registros = %d, want 1", len(r.Registros))
			}
			reg := r.Registros[0]
			if !reg.Falhou || reg.Decisao.Tipo != tt.wantTipo || reg.Decisao.Origem.Gancho != "falho" {
				t.Errorf("Registro = %+v, want falha de %q com %q", reg, "falho", tt.wantTipo)
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
	if r.Decisao.Tipo != ganchos.Bloquear || len(r.Registros) != 1 || !r.Registros[0].Falhou {
		t.Errorf("Resultado = %+v, want falha fechada pelo prazo padrão", r)
	}
	if r.Registros[0].Politica != ganchos.Fechada {
		t.Errorf("Politica = %q, want %q", r.Registros[0].Politica, ganchos.Fechada)
	}
}

func TestAvaliarContextoCancelado(t *testing.T) {
	tests := []struct {
		name          string
		cancelarAntes bool
		wantChamados  []string
	}{
		{"cancelado no meio para antes do proximo", false, []string{"a"}},
		{"cancelado antes nao roda nenhum", true, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tt.cancelarAntes {
				cancel()
			}
			var chamados []string
			anotar := func(nome string) ganchos.Gancho[ganchos.AntesDeResponder] {
				return ganchos.Gancho[ganchos.AntesDeResponder]{
					Nome: nome,
					Avaliar: func(e ganchos.AntesDeResponder) (ganchos.AntesDeResponder, ganchos.Decisao) {
						chamados = append(chamados, nome)
						cancel()
						return e, ganchos.Decisao{Tipo: ganchos.Deixar}
					},
				}
			}

			r := ganchos.Avaliar(ctx, []ganchos.Gancho[ganchos.AntesDeResponder]{anotar("a"), anotar("b")}, ganchos.AntesDeResponder{})

			if !slices.Equal(chamados, tt.wantChamados) {
				t.Errorf("chamados = %v, want %v", chamados, tt.wantChamados)
			}
			if r.Decisao.Tipo != ganchos.Bloquear || r.Decisao.Motivo != ganchos.MotivoDeFalha {
				t.Errorf("Decisao = %+v: avaliação interrompida não deixa passar", r.Decisao)
			}
		})
	}
}
