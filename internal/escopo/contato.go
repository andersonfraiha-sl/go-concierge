package escopo

import (
	"errors"
	"strings"
)

// Contato é o morador visto por uma administradora: memória e pedidos são dele dentro
// dela (a mesma pessoa em duas administradoras são dois contatos). O condomínio é o da
// conversa que o criou.
type Contato struct{ AdministradoraID, CondominioID, Telefone string }

var errContatoIncompleto = errors.New("contato exige administradora, condomínio e telefone")

// NovoContato normaliza o telefone (só dígitos; com 10 ou 11, sem DDI, ganha o 55).
func NovoContato(administradoraID, condominioID, telefone string) (Contato, error) {
	c := Contato{
		AdministradoraID: strings.TrimSpace(administradoraID),
		CondominioID:     strings.TrimSpace(condominioID),
		Telefone:         normalizarTelefone(telefone),
	}
	if c.AdministradoraID == "" || c.CondominioID == "" || c.Telefone == "" {
		return Contato{}, errContatoIncompleto
	}
	return c, nil
}

// Chave identifica o contato no armazenamento: o condomínio não entra.
func (c Contato) Chave() string { return c.AdministradoraID + ":" + c.Telefone }

func normalizarTelefone(t string) string {
	digitos := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, t)
	if len(digitos) == 10 || len(digitos) == 11 {
		return "55" + digitos
	}
	return digitos
}
