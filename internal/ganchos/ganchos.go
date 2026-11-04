// Package ganchos é o núcleo puro dos ganchos: regras fixas de segurança e regras
// declarativas que decidem, em cada ponto do turno, se o evento passa, para, muda ou
// pede confirmação. Nada aqui faz I/O; o relógio só mede o prazo de cada gancho.
package ganchos

import (
	"context"
	"time"
)

// TipoDecisao é o desfecho de um gancho.
type TipoDecisao string

const (
	Deixar    TipoDecisao = "deixar"
	Bloquear  TipoDecisao = "bloquear"
	Ajustar   TipoDecisao = "ajustar"
	Confirmar TipoDecisao = "confirmar"
)

// Origem diz quem decidiu: um fixo (pelo nome) ou uma regra (id e versão).
type Origem struct {
	Gancho, RegraID string
	RegraVersao     int
	Fixo            bool
}

// Decisao de um gancho. A zero value vale Deixar. Motivo é PT-BR simples, para o morador
// ou para o Studio.
type Decisao struct {
	Tipo       TipoDecisao
	Motivo     string
	Origem     Origem
	Quantidade int
}

// Politica diz o que vale quando o gancho falha (panic ou prazo estourado).
type Politica string

const (
	Fechada        Politica = "fechada"
	Aberta         Politica = "aberta"
	FechadaSeGrava Politica = "fechada_se_grava"
)

const (
	PrazoPadrao   = 2 * time.Millisecond
	MotivoDeFalha = "gancho falhou"
)

// Gancho avalia um evento. Avaliar é pura: sem I/O, sem relógio, sem estado, sem mudar
// os mapas recebidos. Prazo zero vale PrazoPadrao; Falha vazia vale Fechada.
type Gancho[E Ocorrencia] struct {
	Nome    string
	Fixo    bool
	Prazo   time.Duration
	Falha   Politica
	Avaliar func(E) (E, Decisao)
}

// Registro de um gancho que não deixou passar ou que falhou: vira log, métrica e auditoria.
type Registro struct {
	Evento   NomeDoEvento
	Decisao  Decisao
	Falhou   bool
	Politica Politica
	Duracao  time.Duration
}

// Resultado da avaliação: o evento ajustado, a decisão final e o que cada gancho decidiu.
type Resultado[E Ocorrencia] struct {
	Evento    E
	Decisao   Decisao
	Perguntas []string
	Registros []Registro
	Duracao   time.Duration
}

// Avaliar roda os ganchos na ordem recebida. Bloquear encerra; ajustar passa o evento
// ajustado ao seguinte; confirmar acumula o motivo em Perguntas. A decisão final segue
// bloquear > confirmar > ajustar > deixar. Contexto cancelado para antes do próximo
// gancho e bloqueia: avaliação interrompida não deixa passar.
func Avaliar[E Ocorrencia](ctx context.Context, gs []Gancho[E], ev E) Resultado[E] {
	inicio := time.Now()
	return seguir(ctx, comecar(ev), gs).resultado(time.Since(inicio))
}

// avaliacao é o acumulador da dobra sobre os ganchos.
type avaliacao[E Ocorrencia] struct {
	evento    E
	decisao   Decisao
	perguntas []string
	registros []Registro
	encerrada bool
}

func (a avaliacao[E]) resultado(duracao time.Duration) Resultado[E] {
	return Resultado[E]{
		Evento: a.evento, Decisao: a.decisao, Perguntas: a.perguntas, Registros: a.registros, Duracao: duracao,
	}
}

func comecar[E Ocorrencia](ev E) avaliacao[E] {
	return avaliacao[E]{evento: ev, decisao: Decisao{Tipo: Deixar}}
}

func seguir[E Ocorrencia](ctx context.Context, a avaliacao[E], gs []Gancho[E]) avaliacao[E] {
	for _, g := range gs {
		if a.encerrada {
			return a
		}
		if ctx.Err() != nil {
			return interromper(a)
		}
		a = aplicar(a, g, executar(g, a.evento))
	}
	return a
}

func interromper[E Ocorrencia](a avaliacao[E]) avaliacao[E] {
	a.decisao = Decisao{Tipo: Bloquear, Motivo: MotivoDeFalha}
	a.encerrada = true
	return a
}

// execucao é o que um gancho devolveu, já com a falha apurada.
type execucao[E Ocorrencia] struct {
	evento  E
	decisao Decisao
	falhou  bool
	duracao time.Duration
}

func executar[E Ocorrencia](g Gancho[E], ev E) execucao[E] {
	inicio := time.Now()
	ajustado, d, ok := protegido(g.Avaliar, ev)
	duracao := time.Since(inicio)
	if d.Tipo == "" {
		d.Tipo = Deixar
	}
	_, conhecida := prioridade(d.Tipo)
	falhou := !ok || !conhecida || duracao > prazoDe(g)
	return execucao[E]{evento: ajustado, decisao: d, falhou: falhou, duracao: duracao}
}

// protegido recupera o panic do gancho: um gancho quebrado é uma falha, não um turno morto.
func protegido[E Ocorrencia](f func(E) (E, Decisao), ev E) (ajustado E, d Decisao, ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	ajustado, d = f(ev)
	return ajustado, d, true
}

func prazoDe[E Ocorrencia](g Gancho[E]) time.Duration {
	if g.Prazo > 0 {
		return g.Prazo
	}
	return PrazoPadrao
}

func aplicar[E Ocorrencia](a avaliacao[E], g Gancho[E], x execucao[E]) avaliacao[E] {
	if x.falhou {
		return aplicarFalha(a, g, x.duracao)
	}
	d := comOrigem(x.decisao, g)
	switch d.Tipo {
	case Deixar:
		return a
	case Bloquear:
		a.encerrada = true
	case Ajustar:
		a.evento = x.evento
	case Confirmar:
		a.perguntas = append(a.perguntas, d.Motivo)
	}
	a.decisao = maisForte(a.decisao, d)
	a.registros = append(a.registros, Registro{
		Evento: a.evento.Nome(), Decisao: d, Politica: politicaDe(g), Duracao: x.duracao,
	})
	return a
}

// aplicarFalha descarta o que o gancho devolveu e aplica a política dele.
func aplicarFalha[E Ocorrencia](a avaliacao[E], g Gancho[E], duracao time.Duration) avaliacao[E] {
	d := comOrigem(Decisao{Tipo: Deixar}, g)
	if fechaNaFalha(politicaDe(g), a.evento) {
		d.Tipo, d.Motivo = Bloquear, MotivoDeFalha
		a.decisao = d
		a.encerrada = true
	}
	a.registros = append(a.registros, Registro{
		Evento: a.evento.Nome(), Decisao: d, Falhou: true, Politica: politicaDe(g), Duracao: duracao,
	})
	return a
}

func comOrigem[E Ocorrencia](d Decisao, g Gancho[E]) Decisao {
	d.Origem.Gancho, d.Origem.Fixo = g.Nome, g.Fixo
	return d
}

func politicaDe[E Ocorrencia](g Gancho[E]) Politica {
	if g.Falha == "" {
		return Fechada
	}
	return g.Falha
}

func fechaNaFalha(p Politica, ev Ocorrencia) bool {
	switch p {
	case Aberta:
		return false
	case FechadaSeGrava:
		gravavel, ok := ev.(interface{ Grava() bool })
		return ok && gravavel.Grava()
	default:
		return true
	}
}

// maisForte fica com a decisão de maior prioridade; no empate, com a primeira.
func maisForte(atual, nova Decisao) Decisao {
	pa, _ := prioridade(atual.Tipo)
	pn, _ := prioridade(nova.Tipo)
	if pn > pa {
		return nova
	}
	return atual
}

func prioridade(t TipoDecisao) (int, bool) {
	switch t {
	case Deixar:
		return 0, true
	case Ajustar:
		return 1, true
	case Confirmar:
		return 2, true
	case Bloquear:
		return 3, true
	default:
		return 0, false
	}
}
