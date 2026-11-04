package memoria

import (
	"context"
	"slices"
	"strings"
	"sync"
	"time"

	"go-concierge/internal/escopo"
)

// Fake é o Store dos testes: guarda a memória como o Postgres a guardaria e devolve cópias.
type Fake struct {
	mu       sync.Mutex
	memorias map[string]Memoria // por escopo.Contato.Chave
}

func NovoFake() *Fake { return &Fake{memorias: map[string]Memoria{}} }

func (f *Fake) Ler(_ context.Context, c escopo.Contato) (Memoria, error) {
	if err := validarContato(c); err != nil {
		return Memoria{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	m, ok := f.memorias[c.Chave()]
	if !ok {
		return vazia(c), nil
	}
	return copia(m), nil
}

func (f *Fake) Gravar(_ context.Context, c escopo.Contato, m Memoria) error {
	if err := validarContato(c); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.memorias[c.Chave()] = comoGuardada(c, m, time.Now())
	return nil
}

func (f *Fake) Listar(_ context.Context, administradoraID string) ([]Memoria, error) {
	if err := validarAdministradora(administradoraID); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []Memoria{}
	for chave, m := range f.memorias {
		if strings.HasPrefix(chave, administradoraID+":") {
			out = append(out, copia(m))
		}
	}
	slices.SortFunc(out, maisRecentesPrimeiro)
	return out, nil
}

func copia(m Memoria) Memoria {
	m.Fatos = append([]Fato{}, m.Fatos...)
	return m
}
