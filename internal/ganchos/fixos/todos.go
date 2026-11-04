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

func juntar[E ganchos.Ocorrencia](a, b ganchos.Pontos[E]) ganchos.Pontos[E] {
	return ganchos.Pontos[E]{
		Checagem:   slices.Concat(a.Checagem, b.Checagem),
		Saneamento: slices.Concat(a.Saneamento, b.Saneamento),
	}
}
