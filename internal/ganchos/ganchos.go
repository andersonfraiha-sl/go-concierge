// Package ganchos é o núcleo puro dos ganchos: regras fixas de segurança e regras
// declarativas que decidem, em cada ponto do turno, se o evento passa, para, muda ou
// pede confirmação. Nada aqui faz I/O; o relógio só mede o prazo de cada gancho.
package ganchos

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
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
// ou para o Studio, e é obrigatório em Bloquear e Confirmar. Quantidade conta o que o
// gancho mexeu (por exemplo, quantos dados mascarou).
type Decisao struct {
	Tipo       TipoDecisao
	Motivo     string
	Origem     Origem
	Quantidade int
	// MotivoAuditoria substitui Motivo na auditoria quando o Motivo carrega texto do
	// morador ou de payload (o modelo precisa dele, a auditoria não pode guardá-lo).
	MotivoAuditoria string
}

// Politica diz o que vale quando o gancho falha.
type Politica string

const (
	Fechada        Politica = "fechada"
	Aberta         Politica = "aberta"
	FechadaSeGrava Politica = "fechada_se_grava"
)

// Causa de uma falha de gancho.
type Causa string

const (
	CausaPanic           Causa = "panic"
	CausaPrazo           Causa = "timeout"
	CausaDecisaoInvalida Causa = "decisao_invalida"
	CausaCancelado       Causa = "cancelado"
)

const (
	PrazoPadrao   = 2 * time.Millisecond
	MotivoDeFalha = "gancho falhou"
	// MotivoTransbordoSolicitado substitui o motivo do transbordo quando o saneamento
	// dele falha (spec §11).
	MotivoTransbordoSolicitado = "transbordo solicitado"
	// GanchoTransbordoSemPII é o nome do fixo que mascara o motivo do transbordo.
	GanchoTransbordoSemPII = "transbordo_sem_pii"
)

// Gancho avalia um evento. Avaliar é pura: sem I/O, sem relógio, sem estado, sem mudar
// os mapas recebidos. Prazo zero vale PrazoPadrao; Falha vazia vale Fechada. Só um
// gancho Fixo pode Ajustar; no Conjunto, Fixo é carimbado pela fase.
type Gancho[E Ocorrencia] struct {
	Nome string
	// Origem leva o id e a versão da regra; vai em toda decisão do gancho, falha inclusive.
	Origem  Origem
	Fixo    bool
	Prazo   time.Duration
	Falha   Politica
	Avaliar func(E) (E, Decisao)
}

// Registro de um gancho que não deixou passar ou que falhou: vira log, métrica e auditoria.
// Causa e Detalhe só vêm em falha; Detalhe nunca leva dado do evento.
type Registro struct {
	Evento   NomeDoEvento
	Decisao  Decisao
	Falhou   bool
	Causa    Causa
	Detalhe  string
	Politica Politica
	Duracao  time.Duration
}

// Resultado da avaliação: o evento ajustado, a decisão final e o que cada gancho decidiu.
// Lentos conta os fixos que passaram do prazo na primeira execução.
type Resultado[E Ocorrencia] struct {
	Evento    E
	Decisao   Decisao
	Perguntas []string
	Registros []Registro
	Duracao   time.Duration
	Lentos    int
}

// Avaliar roda os ganchos na ordem recebida. Bloquear encerra; ajustar (só fixo) passa o
// evento ajustado ao seguinte; confirmar acumula o motivo em Perguntas. A decisão final
// segue bloquear > confirmar > ajustar > deixar. Com o contexto cancelado, os fixos rodam
// assim mesmo (são puros e a máscara nunca pode ser pulada) e cada regra restante conta
// como falha (CausaCancelado) sob a própria política.
func Avaliar[E Ocorrencia](ctx context.Context, gs []Gancho[E], ev E) Resultado[E] {
	return seguir(ctx, comecar(ev, time.Now), gs).resultado()
}

// avaliacao é o acumulador da dobra sobre os ganchos. grava é o Grava() que a política
// FechadaSeGrava lê: o do evento de entrada, ou o da saída da checagem no Conjunto.
// relogio mede o prazo de cada gancho e a duração da avaliação.
type avaliacao[E Ocorrencia] struct {
	evento    E
	decisao   Decisao
	perguntas []string
	registros []Registro
	grava     bool
	lentos    int
	encerrada bool
	relogio   func() time.Time
	inicio    time.Time
}

func comecar[E Ocorrencia](ev E, relogio func() time.Time) avaliacao[E] {
	return avaliacao[E]{evento: ev, decisao: Decisao{Tipo: Deixar}, grava: gravaDe(ev), relogio: relogio, inicio: relogio()}
}

func (a avaliacao[E]) resultado() Resultado[E] {
	return Resultado[E]{
		Evento: a.evento, Decisao: a.decisao, Perguntas: a.perguntas, Registros: a.registros,
		Duracao: a.relogio().Sub(a.inicio), Lentos: a.lentos,
	}
}

func seguir[E Ocorrencia](ctx context.Context, a avaliacao[E], gs []Gancho[E]) avaliacao[E] {
	for _, g := range gs {
		if a.encerrada {
			return a
		}
		if ctx.Err() != nil && !g.Fixo {
			a = aplicarFalha(a, g, falha{causa: CausaCancelado})
			continue
		}
		x := executar(g, a.evento, a.relogio)
		if x.lento {
			a.lentos++
		}
		a = aplicar(a, g, x)
	}
	return a
}

// falha é a causa apurada de um gancho que não valeu.
type falha struct {
	causa   Causa
	detalhe string
	duracao time.Duration
}

// execucao é o que um gancho devolveu; falhou.causa vazia = valeu.
type execucao[E Ocorrencia] struct {
	evento  E
	decisao Decisao
	falhou  falha
	duracao time.Duration
	lento   bool
}

// executar roda o gancho. Fixo que passa do prazo roda de novo uma vez (é puro): pausa de
// GC ou de scheduler não deve fechar um turno. Só falha se passar do prazo de novo.
func executar[E Ocorrencia](g Gancho[E], ev E, relogio func() time.Time) execucao[E] {
	x := rodar(g, ev, relogio)
	if x.falhou.causa != CausaPrazo || !g.Fixo {
		return x
	}
	x = rodar(g, ev, relogio)
	x.lento = true
	return x
}

func rodar[E Ocorrencia](g Gancho[E], ev E, relogio func() time.Time) execucao[E] {
	inicio := relogio()
	ajustado, d, detalhe := protegido(g.Avaliar, ev)
	duracao := relogio().Sub(inicio)
	if d.Tipo == "" {
		d.Tipo = Deixar
	}
	x := execucao[E]{evento: ajustado, decisao: d, duracao: duracao}
	switch {
	case detalhe != "":
		x.falhou = falha{causa: CausaPanic, detalhe: detalhe}
	case duracao > prazoDe(g):
		x.falhou = falha{causa: CausaPrazo}
	case !aceita(d, g.Fixo, ev):
		x.falhou = falha{causa: CausaDecisaoInvalida, detalhe: string(d.Tipo)}
	}
	x.falhou.duracao = duracao
	return x
}

// aceita diz se o desfecho vale para este gancho neste evento. Regra (não fixo) nunca
// ajusta: é o que impede uma regra de desfazer o que um fixo de checagem fixou.
//
// Custo conhecido: passar ev (struct grande) como Ocorrencia aloca a cada gancho, a maior
// parte das ~26 alocações de um turno com 20 regras. Só vale otimizar se a medição do
// turno (Task 18) passar a falhar.
func aceita(d Decisao, fixo bool, ev Ocorrencia) bool {
	switch d.Tipo {
	case Deixar:
		return true
	case Ajustar:
		return fixo
	case Confirmar, Bloquear:
		return d.Motivo != "" && podeSegurar(ev)
	default:
		return false
	}
}

// podeSegurar diz se o evento aceita parar ou perguntar: transbordo ou encerramento sempre
// segue (spec §3, §4 e §11).
func podeSegurar(ev Ocorrencia) bool { return ev.Nome() != EventoTransbordoOuEncerramento }

// protegido recupera o panic do gancho: um gancho quebrado é uma falha, não um turno morto.
// detalhe vazio = não houve panic.
func protegido[E Ocorrencia](f func(E) (E, Decisao), ev E) (ajustado E, d Decisao, detalhe string) {
	defer func() {
		if v := recover(); v != nil {
			detalhe = detalheDoPanic(v)
		}
	}()
	ajustado, d = f(ev)
	return ajustado, d, ""
}

// detalheDoPanic descreve o panic sem o valor dele (que pode trazer dado do morador): a
// mensagem do runtime quando é runtime.Error, senão só o tipo; mais onde panicou.
func detalheDoPanic(v any) string {
	descricao := fmt.Sprintf("%T", v)
	var re runtime.Error
	if err, ok := v.(error); ok && errors.As(err, &re) {
		descricao = re.Error()
	}
	return descricao + " em " + localDoPanic()
}

// localDoPanic é o primeiro quadro fora do runtime acima do recover: quem panicou.
func localDoPanic() string {
	pcs := make([]uintptr, 32)
	quadros := runtime.CallersFrames(pcs[:runtime.Callers(4, pcs)])
	for {
		q, mais := quadros.Next()
		if !strings.HasPrefix(q.Function, "runtime.") {
			return fmt.Sprintf("%s (%s:%d)", q.Function, filepath.Base(q.File), q.Line)
		}
		if !mais {
			return "local desconhecido"
		}
	}
}

func prazoDe[E Ocorrencia](g Gancho[E]) time.Duration {
	if g.Prazo > 0 {
		return g.Prazo
	}
	return PrazoPadrao
}

func aplicar[E Ocorrencia](a avaliacao[E], g Gancho[E], x execucao[E]) avaliacao[E] {
	if x.falhou.causa != "" {
		return aplicarFalha(a, g, x.falhou)
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
func aplicarFalha[E Ocorrencia](a avaliacao[E], g Gancho[E], f falha) avaliacao[E] {
	d := comOrigem(Decisao{Tipo: Deixar}, g)
	if fechaNaFalha(politicaDe(g), a) {
		d.Tipo, d.Motivo = Bloquear, MotivoDeFalha
		a.decisao = maisForte(a.decisao, d) // vale o motivo do primeiro que bloqueou
		a.encerrada = true
	}
	a.registros = append(a.registros, Registro{
		Evento: a.evento.Nome(), Decisao: d, Falhou: true, Causa: f.causa, Detalhe: f.detalhe,
		Politica: politicaDe(g), Duracao: f.duracao,
	})
	return a
}

func comOrigem[E Ocorrencia](d Decisao, g Gancho[E]) Decisao {
	d.Origem = g.Origem
	d.Origem.Gancho, d.Origem.Fixo = g.Nome, g.Fixo
	return d
}

func politicaDe[E Ocorrencia](g Gancho[E]) Politica {
	if g.Falha == "" {
		return Fechada
	}
	return g.Falha
}

func fechaNaFalha[E Ocorrencia](p Politica, a avaliacao[E]) bool {
	if !podeSegurar(a.evento) {
		return false
	}
	switch p {
	case Aberta:
		return false
	case FechadaSeGrava:
		return a.grava
	default:
		return true
	}
}

func gravaDe(ev Ocorrencia) bool {
	gravavel, ok := ev.(interface{ Grava() bool })
	return ok && gravavel.Grava()
}

// maisForte fica com a decisão de maior prioridade; no empate, com a primeira.
func maisForte(atual, nova Decisao) Decisao {
	if prioridade(nova.Tipo) > prioridade(atual.Tipo) {
		return nova
	}
	return atual
}

// prioridade só recebe tipos que passaram por aceita.
func prioridade(t TipoDecisao) int {
	switch t {
	case Ajustar:
		return 1
	case Confirmar:
		return 2
	case Bloquear:
		return 3
	default:
		return 0
	}
}
