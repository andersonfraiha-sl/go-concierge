package admin

import (
	"errors"
	"testing"

	"go-concierge/internal/escopo"
)

func TestResolverCondominio(t *testing.T) {
	m := Mapeamento{Condominios: []Condominio{
		{ID: "c-a", Nome: "A", Administradora: "A", Account: "acc-a", CompanyID: "100", Tenant: "t-a"},
		{ID: "c-b", Nome: "B", Administradora: "B", Account: "acc-b", CompanyID: "200", Tenant: "t-b"},
		{ID: "c-orfao", Nome: "Órfão", Account: "acc-o", CompanyID: "300"},
		{ID: "c-sem-account", Nome: "Sem account", Administradora: "B", CompanyID: "400"},
		{ID: "c-sem-company", Nome: "Sem company", Administradora: "B", Account: "acc-s", Tenant: "t-sem-company"},
		// mesmo tenant em duas administradoras: ambíguo
		{ID: "c-amb-a", Nome: "Amb", Administradora: "A", Account: "acc", CompanyID: "501", Tenant: "t-amb"},
		{ID: "c-amb-b", Nome: "Amb", Administradora: "B", Account: "acc", CompanyID: "502", Tenant: "t-amb"},
		// mesmo tenant na mesma administradora: vale o primeiro por nome e id (fora de ordem aqui)
		{ID: "c-z", Nome: "Zeta", Administradora: "A", Account: "acc", CompanyID: "601", Tenant: "t-mesma"},
		{ID: "c-m2", Nome: "Mesmo", Administradora: "A", Account: "acc", CompanyID: "602", Tenant: "t-mesma"},
		{ID: "c-m1", Nome: "Mesmo", Administradora: "A", Account: "acc", CompanyID: "603", Tenant: "t-mesma"},
	}}
	tests := []struct {
		name     string
		imp      escopo.Implantacao
		company  string
		tenant   string
		wantID   string
		wantErro error
	}{
		{"company conhecida", escopo.Implantacao{}, "200", "", "c-b", nil},
		{"company vence o tenant", escopo.Implantacao{}, "200", "t-a", "c-b", nil},
		{"tenant conhecido sem company", escopo.Implantacao{}, "", "t-a", "c-a", nil},
		{"company desconhecida", escopo.Implantacao{}, "999", "", "", ErrCondominioDesconhecido},
		{"nada informado", escopo.Implantacao{}, "", "", "", ErrCondominioDesconhecido},
		{"condominio sem administradora", escopo.Implantacao{}, "300", "", "", ErrCondominioIncompleto},
		{"condominio sem account", escopo.Implantacao{}, "400", "", "", ErrCondominioIncompleto},
		{"condominio sem company", escopo.Implantacao{}, "", "t-sem-company", "", ErrCondominioIncompleto},
		{"tenant em duas administradoras", escopo.Implantacao{}, "", "t-amb", "", ErrCondominioAmbiguo},
		{"tenant repetido na mesma administradora", escopo.Implantacao{}, "", "t-mesma", "c-m1", nil},
		{"condominio de B com fixa A", escopo.Implantacao{AdministradoraFixa: "A"}, "200", "", "", ErrForaDaImplantacao},
		{"condominio de A com fixa A", escopo.Implantacao{AdministradoraFixa: "A"}, "100", "", "c-a", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := ResolverCondominio(m, tt.imp, tt.company, tt.tenant)
			if !errors.Is(err, tt.wantErro) {
				t.Fatalf("erro = %v, want %v", err, tt.wantErro)
			}
			if c.ID != tt.wantID {
				t.Errorf("condomínio = %q, want %q", c.ID, tt.wantID)
			}
		})
	}
}
