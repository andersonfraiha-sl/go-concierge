package escopo

import (
	"errors"
	"regexp"
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

// Mascarado é o contato para log e mensagem de erro: o telefone sem o miolo.
func (c Contato) Mascarado() string { return c.AdministradoraID + ":" + MascararTelefone(c.Telefone) }

// MascararTelefone normaliza o telefone e esconde o miolo, para log: 5519****2710.
func MascararTelefone(telefone string) string {
	const visiveis = 4
	t := normalizarTelefone(telefone)
	if len(t) <= 2*visiveis {
		return "****"
	}
	return t[:visiveis] + "****" + t[len(t)-visiveis:]
}

// reAdministradora é o formato de id do Admin; também serve de nome de pasta seguro.
var reAdministradora = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

// AdministradoraValida diz se o id tem o formato do Admin.
func AdministradoraValida(id string) bool { return reAdministradora.MatchString(id) }

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
