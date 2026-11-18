package escopo

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

type Contato struct {
	CompanyID        string `json:"company_id"`
	CondoID          *int   `json:"condo_id"`
	Telefone         string `json:"telefone"`
	AdministradoraID string `json:"-"`
	CondominioID     string `json:"-"`
}

func NovoContato(companyID string, condoID any, telefone string) (Contato, error) {
	c := Contato{CompanyID: strings.TrimSpace(companyID), Telefone: NormalizarTelefone(telefone)}
	switch id := condoID.(type) {
	case nil:
	case *int:
		if id != nil {
			v := *id
			c.CondoID = &v
		}
	case int:
		c.CondoID = &id
	case string:
		if strings.TrimSpace(id) != "" {
			n, err := strconv.Atoi(id)
			if err != nil {
				return Contato{}, fmt.Errorf("condomínio deve ser ID inteiro do ERP: %w", err)
			}
			c.CondoID = &n
		}
	default:
		return Contato{}, errors.New("condomínio deve ser ID inteiro opcional do ERP")
	}
	c = c.Canonico()
	if err := c.Validar(); err != nil {
		return Contato{}, err
	}
	if !EmpresaValida(c.CompanyID) || c.Telefone == "" {
		return Contato{}, errContatoIncompleto
	}
	return c, nil
}

var errContatoIncompleto = errors.New("contato exige empresa e telefone")

func (c Contato) Canonico() Contato {
	if c.CompanyID == "" {
		c.CompanyID = strings.TrimSpace(c.AdministradoraID)
	}
	c.CompanyID = strings.TrimSpace(c.CompanyID)
	c.Telefone = NormalizarTelefone(c.Telefone)
	c.AdministradoraID = c.CompanyID
	if c.CondoID != nil {
		c.CondominioID = strconv.Itoa(*c.CondoID)
	} else if c.CondominioID != "" {
		if n, err := strconv.Atoi(c.CondominioID); err == nil {
			c.CondoID = &n
		}
	}
	return c
}

func (c Contato) Validar() error {
	c = c.Canonico()
	if !EmpresaValida(c.CompanyID) || c.Telefone == "" {
		return errContatoIncompleto
	}
	if !CondominioValido(c.CondoID) {
		return errors.New("condomínio fora do intervalo integer do ERP")
	}
	if c.CondoID == nil && c.CondominioID != "" {
		return errors.New("condomínio deve ser ID inteiro do ERP")
	}
	return nil
}

func (c Contato) ChavePessoa() string {
	c = c.Canonico()
	return fmt.Sprintf("%d:%s:%s", len(c.CompanyID), c.CompanyID, c.Telefone)
}

func (c Contato) Chave() string {
	c = c.Canonico()
	recorte := "*"
	if c.CondoID != nil {
		recorte = strconv.Itoa(*c.CondoID)
	}
	return c.ChavePessoa() + ":" + recorte
}

func (c Contato) Mascarado() string {
	return c.Canonico().CompanyID + ":" + MascararTelefone(c.Telefone)
}

func MesmoCondominio(a, b *int) bool { return a == nil && b == nil || a != nil && b != nil && *a == *b }

func (c Contato) Permite(condoID *int) bool {
	return c.CondoID == nil || MesmoCondominio(c.CondoID, condoID)
}

func MascararTelefone(telefone string) string {
	const visiveis = 4
	t := NormalizarTelefone(telefone)
	if len(t) <= 2*visiveis {
		return "****"
	}
	return t[:visiveis] + "****" + t[len(t)-visiveis:]
}

func EmpresaValida(id string) bool { return strings.TrimSpace(id) != "" }

var reAdministradora = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

func AdministradoraValida(id string) bool { return reAdministradora.MatchString(id) }

func NormalizarTelefone(t string) string {
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

func CondominioValido(id *int) bool {
	return id == nil || int64(*id) >= math.MinInt32 && int64(*id) <= math.MaxInt32
}
