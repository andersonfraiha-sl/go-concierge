package memoria

import (
	"context"
	"slices"
	"strings"
	"sync"

	"go-concierge/internal/escopo"
)

// Fake é o Store dos testes: guarda só os fatos por contato, como a tabela, e devolve cópias.
type Fake struct {
	mu    sync.Mutex
	fatos map[string][]Fato // por escopo.Contato.Chave
}

func NovoFake() *Fake { return &Fake{fatos: map[string][]Fato{}} }

func (f *Fake) Ler(_ context.Context, c escopo.Contato) (Memoria, error) {
	if err := validarContato(c); err != nil {
		return Memoria{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return montar(c, append([]Fato{}, f.fatos[c.Chave()]...)), nil
}

func (f *Fake) Gravar(_ context.Context, c escopo.Contato, m Memoria) error {
	if err := validarContato(c); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(m.Fatos) == 0 {
		delete(f.fatos, c.Chave())
		return nil
	}
	f.fatos[c.Chave()] = slices.Clone(m.Fatos)
	return nil
}

func (f *Fake) Listar(_ context.Context, administradoraID string) ([]Memoria, error) {
	if err := validarAdministradora(administradoraID); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []Memoria{}
	for chave, fatos := range f.fatos {
		telefone, ok := strings.CutPrefix(chave, administradoraID+":")
		if !ok {
			continue
		}
		c := escopo.Contato{AdministradoraID: administradoraID, Telefone: telefone}
		out = append(out, montar(c, slices.Clone(fatos)))
	}
	slices.SortFunc(out, maisRecentesPrimeiro)
	return out, nil
}
