package memoria

import (
	"context"
	"slices"
	"sync"
	"time"

	"go-concierge/internal/escopo"
)

type Fake struct {
	mu       sync.Mutex
	memorias map[string]Memoria
}

func NovoFake() *Fake { return &Fake{memorias: map[string]Memoria{}} }

func (f *Fake) Ler(_ context.Context, c escopo.Contato) (Memoria, error) {
	if err := ValidarContato(c); err != nil {
		return Memoria{}, err
	}
	c = c.Canonico()
	f.mu.Lock()
	defer f.mu.Unlock()
	m, ok := f.memorias[c.ChavePessoa()]
	if !ok {
		return Vazia(c), nil
	}
	out := copia(m)
	out.Fatos = slices.DeleteFunc(out.Fatos, func(f Fato) bool { return !c.Permite(f.CondoID) })
	return out, nil
}

func (f *Fake) Gravar(_ context.Context, c escopo.Contato, m Memoria) error {
	if err := ValidarContato(c); err != nil {
		return err
	}
	c = c.Canonico()
	var err error
	if m, err = Preparar(c, m); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	atual := f.memorias[c.ChavePessoa()]
	preservados := slices.DeleteFunc(slices.Clone(atual.Fatos), func(f Fato) bool { return escopo.MesmoCondominio(f.CondoID, c.CondoID) })
	m = ComoGuardada(c, m, time.Now())
	for _, fato := range m.Fatos {
		preservados = slices.DeleteFunc(preservados, func(f Fato) bool { return chaveContextual(f) == chaveContextual(fato) })
		preservados = append(preservados, fato)
	}
	m.Fatos = append([]Fato{}, preservados...)
	f.memorias[c.ChavePessoa()] = copia(m)
	return nil
}

func (f *Fake) Listar(_ context.Context, companyID string) ([]Memoria, error) {
	if err := ValidarAdministradora(companyID); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []Memoria{}
	for _, m := range f.memorias {
		if m.CompanyID == companyID {
			out = append(out, copia(m))
		}
	}
	slices.SortFunc(out, MaisRecentesPrimeiro)
	return out, nil
}

func copia(m Memoria) Memoria {
	m.Fatos = slices.Clone(m.Fatos)
	for i := range m.Fatos {
		if m.Fatos[i].CondoID != nil {
			n := *m.Fatos[i].CondoID
			m.Fatos[i].CondoID = &n
		}
	}
	return m
}
