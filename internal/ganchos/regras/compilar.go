package regras

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"go-concierge/internal/ganchos"
	"go-concierge/internal/ganchos/fixos"
)

const nomeDoGanchoDeRegra = "regra"

var _ Compilador = Compilar

// Compilar monta o índice das regras ativas. A entrada é ordenada por id para que cada
// nível saia ordenado. Regra que não decodifica, não valida ou não compila volta como
// ErroDeCarga e as outras entram.
func Compilar(rs []RegraGravada) (*ganchos.Indice, []ErroDeCarga) {
	indice := &ganchos.Indice{Ativas: map[ganchos.NomeDoEvento]int{}}
	var erros []ErroDeCarga
	fusos := fusosCarregados{}
	for _, g := range slices.SortedFunc(slices.Values(rs), func(a, b RegraGravada) int { return cmp.Compare(a.ID, b.ID) }) {
		regra, err := carregar(g)
		if err == nil {
			err = incluir(indice, fusos, g, regra)
		}
		if err != nil {
			erros = append(erros, ErroDeCarga{RegraID: g.ID, Err: err})
			continue
		}
		indice.Ativas[regra.Evento]++
	}
	return indice, erros
}

func carregar(g RegraGravada) (Regra, error) {
	regra, err := Decodificar(g.Definicao)
	if err != nil {
		return Regra{}, err
	}
	if ps := ValidarRegra(regra, catalogoDaPropriaRegra(regra)); len(ps) > 0 {
		return Regra{}, fmt.Errorf("regra inválida: %s", resumoDosProblemas(ps))
	}
	return regra, nil
}

// catalogoDaPropriaRegra aceita o que a regra cita: ferramenta, caso e condomínio já foram
// conferidos com o catálogo de verdade na gravação; no carregamento só importa a forma.
func catalogoDaPropriaRegra(r Regra) Catalogo {
	cat := Catalogo{Ferramentas: map[string]bool{}, Casos: map[string]bool{}, AdministradoraDe: map[string]string{}}
	for _, c := range r.Condicoes {
		if c.Ferramenta != nil {
			marcar(cat.Ferramentas, c.Ferramenta.Nomes)
		}
		if c.Caso != nil {
			marcar(cat.Casos, c.Caso.IDs)
		}
	}
	for _, condominio := range r.Alvo.Condominios {
		cat.AdministradoraDe[condominio] = r.Alvo.Administradora
	}
	return cat
}

func marcar(conjunto map[string]bool, itens []string) {
	for _, it := range itens {
		conjunto[it] = true
	}
}

func resumoDosProblemas(ps []Problema) string {
	codigos := make([]string, len(ps))
	for i, p := range ps {
		codigos[i] = p.Campo + " " + p.Codigo
	}
	return strings.Join(codigos, "; ")
}

// incluir põe a regra no nível do evento dela. Só antes da ferramenta fecha na falha de
// escrita; nos outros pontos a falha da regra deixa passar.
func incluir(ix *ganchos.Indice, fusos fusosCarregados, g RegraGravada, r Regra) error {
	loc, err := fusos.de(r.Fuso)
	if err != nil {
		return fmt.Errorf("fuso: %w", err)
	}
	conds, err := compilarCondicoes(r.Condicoes, loc)
	if err != nil {
		return err
	}
	decide := decisorDe(r.Decisao, conds)
	origem := ganchos.Origem{RegraID: g.ID, RegraVersao: g.Versao}
	switch r.Evento {
	case ganchos.EventoMensagemRecebida:
		colocar(&ix.MensagemRecebida, r.Alvo, gancho(decide, origem, ganchos.Aberta, lerMensagem))
	case ganchos.EventoAntesDaFerramenta:
		colocar(&ix.AntesDaFerramenta, r.Alvo, gancho(decide, origem, ganchos.FechadaSeGrava, lerAntesDaFerramenta))
	case ganchos.EventoDepoisDaFerramenta:
		colocar(&ix.DepoisDaFerramenta, r.Alvo, gancho(decide, origem, ganchos.Aberta, lerDepoisDaFerramenta))
	case ganchos.EventoAntesDeResponder:
		colocar(&ix.AntesDeResponder, r.Alvo, gancho(decide, origem, ganchos.Aberta, lerResposta))
	default:
		return errors.New("evento sem índice de regras: " + string(r.Evento))
	}
	return nil
}

// decisorDe devolve a decisão da regra para uma leitura: Deixar se alguma condição não vale
// ou depende, a decisão gravada se todas valem. Pedir confirmação deixa passar quando a
// mensagem atual já é o sim do morador, pelo mesmo critério do escrita_confirmada.
func decisorDe(d DecisaoDaRegra, conds []condicaoCompilada) func(leitura) ganchos.Decisao {
	decisao := decisaoDe(d)
	return func(l leitura) ganchos.Decisao {
		if todas(conds, l) != sim {
			return ganchos.Decisao{}
		}
		if decisao.Tipo == ganchos.Confirmar && fixos.Confirmado(l.Confirmacao) {
			return ganchos.Decisao{}
		}
		return decisao
	}
}

// decisaoDe: regra só bloqueia ou pede confirmação; o texto é fixo, escrito na gravação.
func decisaoDe(d DecisaoDaRegra) ganchos.Decisao {
	if d.Tipo == ganchos.Confirmar {
		return ganchos.Decisao{Tipo: ganchos.Confirmar, Motivo: d.Pergunta}
	}
	return ganchos.Decisao{Tipo: ganchos.Bloquear, Motivo: d.Mensagem}
}

func gancho[E ganchos.Ocorrencia](decide func(leitura) ganchos.Decisao, origem ganchos.Origem, falha ganchos.Politica, ler func(E) leitura) ganchos.Gancho[E] {
	return ganchos.Gancho[E]{
		Nome: nomeDoGanchoDeRegra, Origem: origem, Prazo: ganchos.PrazoPadrao, Falha: falha,
		Avaliar: func(e E) (E, ganchos.Decisao) { return e, decide(ler(e)) },
	}
}

// colocar põe o gancho em cada condomínio do alvo, senão na administradora, senão nos globais.
func colocar[E ganchos.Ocorrencia](n *ganchos.Nivel[E], alvo Alvo, g ganchos.Gancho[E]) {
	switch {
	case len(alvo.Condominios) > 0:
		if n.PorCondominio == nil {
			n.PorCondominio = map[string][]ganchos.Gancho[E]{}
		}
		for _, c := range alvo.Condominios {
			n.PorCondominio[c] = append(n.PorCondominio[c], g)
		}
	case alvo.Administradora != "":
		if n.PorAdministradora == nil {
			n.PorAdministradora = map[string][]ganchos.Gancho[E]{}
		}
		n.PorAdministradora[alvo.Administradora] = append(n.PorAdministradora[alvo.Administradora], g)
	default:
		n.Globais = append(n.Globais, g)
	}
}

func lerMensagem(e ganchos.MensagemRecebida) leitura {
	return leitura{Agora: e.Agora, Texto: e.Texto, Identidade: e.Identidade}
}

func lerAntesDaFerramenta(e ganchos.AntesDaFerramenta) leitura {
	return leitura{Agora: e.Agora, Ferramenta: e.Ferramenta, Caso: e.Caso, Params: e.Params, Identidade: e.Identidade, Confirmacao: e.Confirmacao}
}

func lerDepoisDaFerramenta(e ganchos.DepoisDaFerramenta) leitura {
	return leitura{Agora: e.Agora, Ferramenta: e.Ferramenta, Caso: e.Caso, Params: e.Params, Identidade: e.Identidade}
}

func lerResposta(e ganchos.AntesDeResponder) leitura {
	return leitura{Agora: e.Agora, Texto: e.Texto, Identidade: e.Identidade}
}

// fusosCarregados evita reler o banco de zonas a cada regra: 10 mil regras dividem poucos fusos.
type fusosCarregados map[string]*time.Location

func (f fusosCarregados) de(nome string) (*time.Location, error) {
	if loc, ok := f[nome]; ok {
		return loc, nil
	}
	loc, err := time.LoadLocation(nome)
	if err != nil {
		return nil, err
	}
	f[nome] = loc
	return loc, nil
}
