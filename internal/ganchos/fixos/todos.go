package fixos

import (
	"slices"

	"go-concierge/internal/ganchos"
)

// Todos junta checagem e saneamento: os fixos de uma borda que recebeu Conjunto nil.
func Todos() ganchos.Fixos {
	c, s := Checagem(), Saneamento()
	return ganchos.Fixos{
		MensagemRecebida:         juntar(c.MensagemRecebida, s.MensagemRecebida),
		AntesDaFerramenta:        juntar(c.AntesDaFerramenta, s.AntesDaFerramenta),
		DepoisDaFerramenta:       juntar(c.DepoisDaFerramenta, s.DepoisDaFerramenta),
		AntesDeResponder:         juntar(c.AntesDeResponder, s.AntesDeResponder),
		TransbordoOuEncerramento: juntar(c.TransbordoOuEncerramento, s.TransbordoOuEncerramento),
	}
}

// Ausentes lista, como evento/nome, os fixos de Todos que tem não traz: um conjunto
// injetado sem eles é recusado, porque fixo nunca some por configuração.
func Ausentes(tem map[ganchos.NomeDoEvento][]string) []string {
	var faltam []string
	for evento, nomes := range ganchos.NovoConjunto(Todos(), nil).NomesDosFixos() {
		for _, nome := range nomes {
			if !slices.Contains(tem[evento], nome) {
				faltam = append(faltam, string(evento)+"/"+nome)
			}
		}
	}
	slices.Sort(faltam)
	return faltam
}

func juntar[E ganchos.Ocorrencia](a, b ganchos.Pontos[E]) ganchos.Pontos[E] {
	return ganchos.Pontos[E]{
		Checagem:   slices.Concat(a.Checagem, b.Checagem),
		Saneamento: slices.Concat(a.Saneamento, b.Saneamento),
	}
}
