// Package ganchostest monta ganchos e conjuntos roteirizados para testes.
package ganchostest

import (
	"slices"
	"sync"

	"go-concierge/internal/ganchos"
)

// Roteiro diz o que o gancho decide e anota cada evento que ele recebeu. É seguro entre
// turnos concorrentes: com turnos ainda rodando, leia por Eventos, não por Recebidos.
type Roteiro[E ganchos.Ocorrencia] struct {
	mu        sync.Mutex
	Recebidos []E
	Decisao   ganchos.Decisao
}

// Eventos devolve uma cópia dos eventos recebidos até agora.
func (r *Roteiro[E]) Eventos() []E {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.Recebidos)
}

func (r *Roteiro[E]) receber(e E) ganchos.Decisao {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Recebidos = append(r.Recebidos, e)
	return r.Decisao
}

// Gancho devolve um gancho que anota o evento em r e decide r.Decisao, sem mudar o evento.
func Gancho[E ganchos.Ocorrencia](nome string, r *Roteiro[E]) ganchos.Gancho[E] {
	return ganchos.Gancho[E]{
		Nome:    nome,
		Avaliar: func(e E) (E, ganchos.Decisao) { return e, r.receber(e) },
	}
}

// Conjunto monta um conjunto com fixos f e o índice fixo idx (nil = sem regras).
func Conjunto(f ganchos.Fixos, idx *ganchos.Indice) *ganchos.Conjunto {
	return ganchos.NovoConjunto(f, func() *ganchos.Indice { return idx })
}
