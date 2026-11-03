package admin

import (
	"errors"
	"testing"

	"go-concierge/internal/escopo"
)

func TestResolverCondominio(t *testing.T) {
	m := Mapeamento{Condominios: []Condominio{
		{ID: "c-a", Administradora: "A", CompanyID: "100", Tenant: "t-a"},
		{ID: "c-b", Administradora: "B", CompanyID: "200", Tenant: "t-b"},
		{ID: "c-orfao", CompanyID: "300"},
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
		{"condominio sem administradora", escopo.Implantacao{}, "300", "", "", ErrCondominioDesconhecido},
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
