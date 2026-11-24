package memoria

import (
	"slices"

	"go-concierge/internal/escopo"
)

func ParaTransporte(c escopo.Contato, m Memoria) (Memoria, error) {
	if err := c.Validar(); err != nil {
		return Memoria{}, err
	}
	c = c.Canonico()
	m.Fatos = slices.Clone(m.Fatos)
	for i := range m.Fatos {
		f := &m.Fatos[i]
		if f.CondoID != nil || f.CondominioID == "" {
			continue
		}
		alvo, err := escopo.NovoContato(c.CompanyID, f.CondominioID, c.Telefone)
		if err != nil {
			return Memoria{}, err
		}
		f.CondoID = alvo.CondoID
	}
	return m, nil
}
