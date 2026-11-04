package ganchos

import (
	"context"
	"slices"
	"sync"
	"time"

	"go-concierge/internal/ganchos/pii"
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

// Indice é o retrato das regras ativas que o cache entrega. Quem publica um índice novo
// cria outro; nunca mexe no que já entregou.
type Indice struct {
	MensagemRecebida         Nivel[MensagemRecebida]
	AntesDaFerramenta        Nivel[AntesDaFerramenta]
	DepoisDaFerramenta       Nivel[DepoisDaFerramenta]
	AntesDeResponder         Nivel[AntesDeResponder]
	TransbordoOuEncerramento Nivel[TransbordoOuEncerramento]
	Ativas                   map[NomeDoEvento]int
}

// Observacao é o resumo de uma avaliação para log, métrica e auditoria. Sem texto, params
// nem resultado: nada de PII.
type Observacao struct {
	Evento                         NomeDoEvento
	CondominioID, AdministradoraID string
	Duracao                        time.Duration
	Lentos                         int
	Registros                      []Registro
	// Previa marca a avaliação de um turno de prévia: entra em log e métrica, nunca na
	// auditoria, porque nada aconteceu ainda.
	Previa bool
}

type Opcao func(*Conjunto)

// ComObservador recebe a observação de cada avaliação. Com mais de um, todos recebem, na
// ordem das opções (log e métrica, depois a auditoria).
func ComObservador(observar func(context.Context, Observacao)) Opcao {
	return func(c *Conjunto) {
		anterior := c.observar
		if anterior == nil {
			c.observar = observar
			return
		}
		c.observar = func(ctx context.Context, o Observacao) {
			anterior(ctx, o)
			observar(ctx, o)
		}
	}
}

// Conjunto junta os fixos e a fonte do índice de regras. Os fixos não têm setter: só
// NovoConjunto os define, numa cópia própria.
type Conjunto struct {
	fixos    Fixos
	regras   func() *Indice
	observar func(context.Context, Observacao)
}

// NovoConjunto monta o conjunto. regras nil (ou devolvendo nil) = sem regras.
func NovoConjunto(f Fixos, regras func() *Indice, opcoes ...Opcao) *Conjunto {
	c := &Conjunto{
		fixos: Fixos{
			MensagemRecebida:         fixar(f.MensagemRecebida),
			AntesDaFerramenta:        fixar(f.AntesDaFerramenta),
			DepoisDaFerramenta:       fixar(f.DepoisDaFerramenta),
			AntesDeResponder:         fixar(f.AntesDeResponder),
			TransbordoOuEncerramento: fixar(f.TransbordoOuEncerramento),
		},
		regras: regras,
	}
	for _, o := range opcoes {
		o(c)
	}
	return c
}

// Turno é a visão do conjunto para um turno: o índice de regras é lido uma vez, na
// abertura, e vale para todos os eventos do turno. Regra salva no meio só vale no próximo.
type Turno struct {
	conjunto *Conjunto
	indice   Indice
	cofre    *pii.Cofre
	previa   bool
	// confirmadas são as perguntas de regra da resposta anterior que a mensagem atual
	// confirmou; feitas, as que os ganchos fizeram neste turno.
	confirmadas []string
	feitas      *perguntasFeitas
}

// perguntasFeitas junta as perguntas de confirmação de um turno: as chamadas de ferramenta
// do mesmo turno podem rodar em paralelo.
type perguntasFeitas struct {
	mu        sync.Mutex
	perguntas []string
}

func (f *perguntasFeitas) guardar(ps []string) {
	if f == nil || len(ps) == 0 {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, p := range ps {
		if !slices.Contains(f.perguntas, p) {
			f.perguntas = append(f.perguntas, p)
		}
	}
}

func (f *perguntasFeitas) ler() []string {
	if f == nil {
		return nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.perguntas)
}

// Turno abre a visão de um turno, com o cofre de PII que só ele possui.
func (c *Conjunto) Turno() Turno { return c.TurnoAPartirDe(1) }

// TurnoAPartirDe abre o turno com o cofre numerando os marcadores a partir de proximo.
func (c *Conjunto) TurnoAPartirDe(proximo int) Turno {
	return Turno{conjunto: c, indice: c.indice(), cofre: pii.NovoCofreAPartirDe(proximo), feitas: &perguntasFeitas{}}
}

// TurnoDePrevia abre um turno descartável, com cofre próprio, para prever uma decisão antes
// da hora: as observações dele saem com Previa, e a auditoria as ignora.
func (c *Conjunto) TurnoDePrevia() Turno {
	t := c.Turno()
	t.previa = true
	return t
}

// Cofre é o cofre de PII do turno: o mesmo que os eventos recebem.
func (t Turno) Cofre() *pii.Cofre { return t.cofre }

// ComPerguntasConfirmadas devolve o turno que leva, em cada antes_da_ferramenta, as
// perguntas de regra que a mensagem atual confirmou. Quem decide se confirmou é a borda,
// que conhece a mensagem e a resposta anterior.
func (t Turno) ComPerguntasConfirmadas(perguntas []string) Turno {
	t.confirmadas = slices.Clone(perguntas)
	return t
}

// PerguntasFeitas são as perguntas de confirmação que os ganchos fizeram neste turno, sem
// repetição, na ordem em que saíram. Texto fixo da regra: sem PII. Prévia não conta.
func (t Turno) PerguntasFeitas() []string { return t.feitas.ler() }

func (t Turno) MensagemRecebida(ctx context.Context, e MensagemRecebida) Resultado[MensagemRecebida] {
	e.Cofre = t.cofre
	return avaliarNoPonto(ctx, t, ponto[MensagemRecebida]{t.conjunto.fixos.MensagemRecebida, t.indice.MensagemRecebida}, e)
}

func (t Turno) AntesDaFerramenta(ctx context.Context, e AntesDaFerramenta) Resultado[AntesDaFerramenta] {
	e.Cofre = t.cofre
	e.PerguntasConfirmadas = t.confirmadas
	r := avaliarNoPonto(ctx, t, ponto[AntesDaFerramenta]{t.conjunto.fixos.AntesDaFerramenta, t.indice.AntesDaFerramenta}, e)
	if !t.previa {
		t.feitas.guardar(r.Perguntas)
	}
	return r
}

func (t Turno) DepoisDaFerramenta(ctx context.Context, e DepoisDaFerramenta) Resultado[DepoisDaFerramenta] {
	e.Cofre = t.cofre
	return avaliarNoPonto(ctx, t, ponto[DepoisDaFerramenta]{t.conjunto.fixos.DepoisDaFerramenta, t.indice.DepoisDaFerramenta}, e)
}

func (t Turno) AntesDeResponder(ctx context.Context, e AntesDeResponder) Resultado[AntesDeResponder] {
	e.Cofre = t.cofre
	return avaliarNoPonto(ctx, t, ponto[AntesDeResponder]{t.conjunto.fixos.AntesDeResponder, t.indice.AntesDeResponder}, e)
}

// TransbordoOuEncerramento nunca bloqueia. Se o saneamento do motivo falhar, o motivo cru
// não segue: sai MotivoTransbordoSolicitado.
func (t Turno) TransbordoOuEncerramento(ctx context.Context, e TransbordoOuEncerramento) Resultado[TransbordoOuEncerramento] {
	r := avaliarNoPonto(ctx, t, ponto[TransbordoOuEncerramento]{t.conjunto.fixos.TransbordoOuEncerramento, t.indice.TransbordoOuEncerramento}, e)
	if saneamentoDoMotivoFalhou(r.Registros) {
		r.Evento.Motivo = MotivoTransbordoSolicitado
	}
	return r
}

func saneamentoDoMotivoFalhou(registros []Registro) bool {
	return slices.ContainsFunc(registros, func(r Registro) bool {
		return r.Falhou && r.Decisao.Origem.Gancho == GanchoTransbordoSemPII
	})
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

// NomesDosFixos do conjunto que abriu o turno; turno zero não tem fixos.
func (t Turno) NomesDosFixos() map[NomeDoEvento][]string {
	if t.conjunto == nil {
		return nil
	}
	return t.conjunto.NomesDosFixos()
}

// comAlvo é o evento que sabe a que condomínio e administradora se refere.
type comAlvo interface {
	Ocorrencia
	alvo() (condominioID, administradoraID string)
}

// ponto junta os fixos e as regras de um evento no turno.
type ponto[E Ocorrencia] struct {
	fixos  Pontos[E]
	regras Nivel[E]
}

// avaliarNoPonto roda checagem, regras do alvo e saneamento como uma avaliação só. O alvo
// e o Grava() da política saem do evento como ele deixou a checagem: só fixo mexe neles.
func avaliarNoPonto[E comAlvo](ctx context.Context, t Turno, p ponto[E], ev E) Resultado[E] {
	inicio := time.Now()
	a := seguir(ctx, comecar(ev), p.fixos.Checagem)
	a.grava = gravaDe(a.evento)
	condominioID, administradoraID := a.evento.alvo()
	a = seguir(ctx, a, carimbar(p.regras.Para(condominioID, administradoraID), false))
	r := sanear(ctx, a, p.fixos.Saneamento).resultado(time.Since(inicio))
	if t.conjunto.observar != nil {
		t.conjunto.observar(ctx, Observacao{
			Evento: ev.Nome(), CondominioID: condominioID, AdministradoraID: administradoraID,
			Duracao: r.Duracao, Lentos: r.Lentos, Registros: r.Registros, Previa: t.previa,
		})
	}
	return r
}

// sanear roda o saneamento mesmo depois de um Bloquear de checagem ou regra: o evento que
// volta ao chamador nunca sai com PII crua. A decisão continua a que foi tomada.
func sanear[E Ocorrencia](ctx context.Context, a avaliacao[E], gs []Gancho[E]) avaliacao[E] {
	bloqueada := a.encerrada
	a.encerrada = false
	a = seguir(ctx, a, gs)
	a.encerrada = a.encerrada || bloqueada
	return a
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

func fixar[E Ocorrencia](p Pontos[E]) Pontos[E] {
	return Pontos[E]{Checagem: carimbar(p.Checagem, true), Saneamento: carimbar(p.Saneamento, true)}
}

// carimbar copia os ganchos marcando Fixo pela fase: quem monta o gancho não decide isso.
func carimbar[E Ocorrencia](gs []Gancho[E], fixo bool) []Gancho[E] {
	copia := slices.Clone(gs)
	for i := range copia {
		copia[i].Fixo = fixo
	}
	return copia
}

func nomes[E Ocorrencia](p Pontos[E]) []string {
	ns := make([]string, 0, len(p.Checagem)+len(p.Saneamento))
	for _, g := range slices.Concat(p.Checagem, p.Saneamento) {
		ns = append(ns, g.Nome)
	}
	return ns
}
