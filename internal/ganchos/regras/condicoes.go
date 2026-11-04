package regras

import (
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	"go-concierge/internal/ganchos"
)

// veredito é o resultado de uma condição. Depende é parâmetro ainda desconhecido: na
// prévia a regra deixa passar, e na gravação "depende" vale como "não vale". Nos dois
// casos só Sim dispara a regra.
type veredito int

const (
	nao veredito = iota
	sim
	depende
)

// leitura é o que as condições enxergam do evento, igual para todos os pontos do turno.
type leitura struct {
	Agora            time.Time
	Ferramenta, Caso string
	Params           map[string]any
	Texto            string
	Identidade       ganchos.Identidade
}

type condicaoCompilada func(leitura) veredito

// todas é o "e" das condições: nao vence, depois depende.
func todas(cs []condicaoCompilada, l leitura) veredito {
	resultado := sim
	for _, c := range cs {
		switch c(l) {
		case nao:
			return nao
		case depende:
			resultado = depende
		}
	}
	return resultado
}

func compilarCondicoes(cs []Condicao, loc *time.Location) ([]condicaoCompilada, error) {
	compiladas := make([]condicaoCompilada, 0, len(cs))
	for i, c := range cs {
		cc, err := compilarCondicao(c, loc)
		if err != nil {
			return nil, fmt.Errorf("condicoes[%d]: %w", i, err)
		}
		compiladas = append(compiladas, cc)
	}
	return compiladas, nil
}

func compilarCondicao(c Condicao, loc *time.Location) (condicaoCompilada, error) {
	switch c.Tipo {
	case CondicaoFerramenta:
		return daLista(c.Ferramenta.Nomes, func(l leitura) string { return l.Ferramenta }), nil
	case CondicaoCaso:
		return daLista(c.Caso.IDs, func(l leitura) string { return l.Caso }), nil
	case CondicaoForaDoHorario:
		return foraDoHorario(*c.ForaDoHorario, loc)
	case CondicaoDiaDaSemana:
		return diaDaSemana(*c.DiaDaSemana, loc)
	case CondicaoDiaNaoUtil:
		return diaNaoUtil(*c.DiaNaoUtil, loc), nil
	case CondicaoContatoCadastrado:
		return contatoCadastrado(*c.ContatoCadastrado.Valor), nil
	case CondicaoPapelDoMorador:
		return papelDoMorador(c.PapelDoMorador.Papeis), nil
	case CondicaoTextoContem:
		return textoContem(c.TextoContem.Termos), nil
	}
	return nil, fmt.Errorf("condição desconhecida: %s", c.Tipo)
}

func daLista(valores []string, ler func(leitura) string) condicaoCompilada {
	return func(l leitura) veredito { return deBool(slices.Contains(valores, ler(l))) }
}

func deBool(b bool) veredito {
	if b {
		return sim
	}
	return nao
}

func contatoCadastrado(esperado bool) condicaoCompilada {
	return func(l leitura) veredito { return deBool(l.Identidade.Cadastrado == esperado) }
}

func papelDoMorador(papeis []string) condicaoCompilada {
	return func(l leitura) veredito {
		return deBool(slices.ContainsFunc(l.Identidade.Papeis, func(p string) bool {
			return slices.ContainsFunc(papeis, func(q string) bool { return strings.EqualFold(p, q) })
		}))
	}
}

func textoContem(termos []string) condicaoCompilada {
	normalizados := make([]string, len(termos))
	for i, t := range termos {
		normalizados[i] = normalizar(t)
	}
	return func(l leitura) veredito {
		texto := normalizar(l.Texto)
		return deBool(slices.ContainsFunc(normalizados, func(t string) bool { return strings.Contains(texto, t) }))
	}
}

// normalizar põe em minúscula e tira os acentos. A cadeia tem estado, então nasce a cada chamada.
func normalizar(s string) string {
	minuscula := strings.ToLower(s)
	tirarAcentos := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	semAcento, _, err := transform.String(tirarAcentos, minuscula)
	if err != nil {
		return minuscula
	}
	return semAcento
}

// foraDoHorario: inicio e fim delimitam o horário de funcionamento, no fuso da regra; a
// condição vale fora dele. inicio > fim é um horário que cruza a meia-noite.
func foraDoHorario(h CondHorario, loc *time.Location) (condicaoCompilada, error) {
	inicio, err := minutosDoDia(h.Inicio)
	if err != nil {
		return nil, fmt.Errorf("inicio: %w", err)
	}
	fim, err := minutosDoDia(h.Fim)
	if err != nil {
		return nil, fmt.Errorf("fim: %w", err)
	}
	return comInstante(h.Fonte, loc, func(t time.Time) bool {
		return !dentroDoHorario(t.Hour()*60+t.Minute(), inicio, fim)
	}), nil
}

func minutosDoDia(hhmm string) (int, error) {
	t, err := time.Parse("15:04", hhmm)
	if err != nil {
		return 0, err
	}
	return t.Hour()*60 + t.Minute(), nil
}

func dentroDoHorario(minuto, inicio, fim int) bool {
	if inicio <= fim {
		return minuto >= inicio && minuto < fim
	}
	return minuto >= inicio || minuto < fim
}

var diasPorNome = map[string]time.Weekday{
	"dom": time.Sunday, "seg": time.Monday, "ter": time.Tuesday, "qua": time.Wednesday,
	"qui": time.Thursday, "sex": time.Friday, "sab": time.Saturday,
}

func diaDaSemana(d CondDias, loc *time.Location) (condicaoCompilada, error) {
	dias := make([]time.Weekday, 0, len(d.Dias))
	for _, nome := range d.Dias {
		dia, ok := diasPorNome[nome]
		if !ok {
			return nil, fmt.Errorf("dia da semana inválido: %s", nome)
		}
		dias = append(dias, dia)
	}
	return comInstante(d.Fonte, loc, func(t time.Time) bool { return slices.Contains(dias, t.Weekday()) }), nil
}

func diaNaoUtil(d CondNaoUtil, loc *time.Location) condicaoCompilada {
	return comInstante(d.Fonte, loc, func(t time.Time) bool {
		fimDeSemana := t.Weekday() == time.Saturday || t.Weekday() == time.Sunday
		return fimDeSemana || FeriadoNacional(t) || slices.Contains(d.FeriadosExtras, t.Format("2006-01-02"))
	})
}

// comInstante resolve a fonte no fuso da regra e aplica o teste; sem o instante, depende.
func comInstante(f Fonte, loc *time.Location, teste func(time.Time) bool) condicaoCompilada {
	return func(l leitura) veredito {
		t, ok := instante(f, l, loc)
		if !ok {
			return depende
		}
		return deBool(teste(t))
	}
}

var formatosDeData = []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04", "2006-01-02"}

func instante(f Fonte, l leitura, loc *time.Location) (time.Time, bool) {
	if f.Tipo == FonteAgora {
		return l.Agora.In(loc), true
	}
	texto, ok := l.Params[f.Nome].(string)
	if !ok {
		return time.Time{}, false
	}
	for _, formato := range formatosDeData {
		if t, err := time.ParseInLocation(formato, texto, loc); err == nil {
			return t.In(loc), true
		}
	}
	return time.Time{}, false
}
