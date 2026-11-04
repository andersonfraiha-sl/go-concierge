package ganchos

import (
	"context"
	"slices"
	"time"
)

// Pontos são os fixos de um evento: checagem roda antes das regras, saneamento depois.
type Pontos[E Ocorrencia] struct{ Checagem, Saneamento []Gancho[E] }

// Fixos de cada evento do turno, montados no código.
type Fixos struct {
	MensagemRecebida         Pontos[MensagemRecebida]
	AntesDaFerramenta        Pontos[AntesDaFerramenta]
	DepoisDaFerramenta       Pontos[DepoisDaFerramenta]
	AntesDeResponder         Pontos[AntesDeResponder]
	TransbordoOuEncerramento Pontos[TransbordoOuEncerramento]
}

// Nivel guarda as regras de um evento por alvo, já ordenadas por id dentro de cada nível.
type Nivel[E Ocorrencia] struct {
	PorCondominio, PorAdministradora map[string][]Gancho[E]
	Globais                          []Gancho[E]
}

// Para devolve as regras do alvo: condomínio, administradora, globais.
func (n Nivel[E]) Para(condominioID, administradoraID string) []Gancho[E] {
	return slices.Concat(n.PorCondominio[condominioID], n.PorAdministradora[administradoraID], n.Globais)
}

// Indice é o retrato das regras ativas que o cache entrega a cada turno.
type Indice struct {
	MensagemRecebida         Nivel[MensagemRecebida]
	AntesDaFerramenta        Nivel[AntesDaFerramenta]
	DepoisDaFerramenta       Nivel[DepoisDaFerramenta]
	AntesDeResponder         Nivel[AntesDeResponder]
	TransbordoOuEncerramento Nivel[TransbordoOuEncerramento]
	Ativas                   map[NomeDoEvento]int
}

type Opcao func(*Conjunto)

// ComObservador recebe os registros de cada avaliação (log, métrica, auditoria).
func ComObservador(observar func(context.Context, []Registro)) Opcao {
	return func(c *Conjunto) { c.observar = observar }
}

// Conjunto junta os fixos e o índice de regras, com um método por evento. Os fixos não
// têm setter: só NovoConjunto os define.
type Conjunto struct {
	fixos    Fixos
	regras   func() *Indice
	observar func(context.Context, []Registro)
}

// NovoConjunto monta o conjunto. regras nil (ou devolvendo nil) = sem regras.
func NovoConjunto(f Fixos, regras func() *Indice, opcoes ...Opcao) *Conjunto {
	c := &Conjunto{fixos: f, regras: regras}
	for _, o := range opcoes {
		o(c)
	}
	return c
}

func (c *Conjunto) MensagemRecebida(ctx context.Context, e MensagemRecebida) Resultado[MensagemRecebida] {
	return avaliarNoPonto(ctx, c, c.fixos.MensagemRecebida, func(i Indice) Nivel[MensagemRecebida] { return i.MensagemRecebida }, e)
}

func (c *Conjunto) AntesDaFerramenta(ctx context.Context, e AntesDaFerramenta) Resultado[AntesDaFerramenta] {
	return avaliarNoPonto(ctx, c, c.fixos.AntesDaFerramenta, func(i Indice) Nivel[AntesDaFerramenta] { return i.AntesDaFerramenta }, e)
}

func (c *Conjunto) DepoisDaFerramenta(ctx context.Context, e DepoisDaFerramenta) Resultado[DepoisDaFerramenta] {
	return avaliarNoPonto(ctx, c, c.fixos.DepoisDaFerramenta, func(i Indice) Nivel[DepoisDaFerramenta] { return i.DepoisDaFerramenta }, e)
}

func (c *Conjunto) AntesDeResponder(ctx context.Context, e AntesDeResponder) Resultado[AntesDeResponder] {
	return avaliarNoPonto(ctx, c, c.fixos.AntesDeResponder, func(i Indice) Nivel[AntesDeResponder] { return i.AntesDeResponder }, e)
}

func (c *Conjunto) TransbordoOuEncerramento(ctx context.Context, e TransbordoOuEncerramento) Resultado[TransbordoOuEncerramento] {
	return avaliarNoPonto(ctx, c, c.fixos.TransbordoOuEncerramento, func(i Indice) Nivel[TransbordoOuEncerramento] { return i.TransbordoOuEncerramento }, e)
}

// NomesDosFixos lista, por evento, os fixos de checagem e depois os de saneamento.
func (c *Conjunto) NomesDosFixos() map[NomeDoEvento][]string {
	return map[NomeDoEvento][]string{
		EventoMensagemRecebida:         nomes(c.fixos.MensagemRecebida),
		EventoAntesDaFerramenta:        nomes(c.fixos.AntesDaFerramenta),
		EventoDepoisDaFerramenta:       nomes(c.fixos.DepoisDaFerramenta),
		EventoAntesDeResponder:         nomes(c.fixos.AntesDeResponder),
		EventoTransbordoOuEncerramento: nomes(c.fixos.TransbordoOuEncerramento),
	}
}

// comAlvo é o evento que sabe a que condomínio e administradora se refere.
type comAlvo interface {
	Ocorrencia
	alvo() (condominioID, administradoraID string)
}

// avaliarNoPonto roda checagem, regras do alvo e saneamento como uma avaliação só. O alvo
// é lido depois da checagem, que pode tê-lo preenchido (escopo da conversa).
func avaliarNoPonto[E comAlvo](ctx context.Context, c *Conjunto, p Pontos[E], nivel func(Indice) Nivel[E], ev E) Resultado[E] {
	inicio := time.Now()
	a := seguir(ctx, comecar(ev), p.Checagem)
	regras := nivel(c.indice()).Para(a.evento.alvo())
	r := seguir(ctx, a, slices.Concat(regras, p.Saneamento)).resultado(time.Since(inicio))
	if c.observar != nil {
		c.observar(ctx, r.Registros)
	}
	return r
}

func (c *Conjunto) indice() Indice {
	if c.regras == nil {
		return Indice{}
	}
	if i := c.regras(); i != nil {
		return *i
	}
	return Indice{}
}

func nomes[E Ocorrencia](p Pontos[E]) []string {
	ns := make([]string, 0, len(p.Checagem)+len(p.Saneamento))
	for _, g := range slices.Concat(p.Checagem, p.Saneamento) {
		ns = append(ns, g.Nome)
	}
	return ns
}
