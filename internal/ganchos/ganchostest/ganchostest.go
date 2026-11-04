// Package ganchostest monta ganchos e conjuntos roteirizados para testes.
package ganchostest

import "go-concierge/internal/ganchos"

// Roteiro diz o que o gancho decide e anota cada evento que ele recebeu.
type Roteiro[E ganchos.Ocorrencia] struct {
	Recebidos []E
	Decisao   ganchos.Decisao
}

// Gancho devolve um gancho que anota o evento em r e decide r.Decisao, sem mudar o evento.
func Gancho[E ganchos.Ocorrencia](nome string, r *Roteiro[E]) ganchos.Gancho[E] {
	return ganchos.Gancho[E]{
		Nome: nome,
		Avaliar: func(e E) (E, ganchos.Decisao) {
			r.Recebidos = append(r.Recebidos, e)
			return e, r.Decisao
		},
	}
}

// Conjunto monta um conjunto com fixos f e o índice fixo idx (nil = sem regras).
func Conjunto(f ganchos.Fixos, idx *ganchos.Indice) *ganchos.Conjunto {
	return ganchos.NovoConjunto(f, func() *ganchos.Indice { return idx })
}
