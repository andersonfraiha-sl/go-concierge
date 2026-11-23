package main

import (
	"context"
	"slices"
	"testing"

	"go-concierge/internal/acesso"
	"go-concierge/internal/admin"
)

type rbacGravado struct {
	concessoes  []acesso.Concessao
	atribuicoes map[string]acesso.Atribuicao
}

func (r *rbacGravado) acessoDev() acessoDev {
	r.atribuicoes = map[string]acesso.Atribuicao{}
	return acessoDev{
		conceder: func(_ context.Context, c acesso.Concessao) error {
			r.concessoes = append(r.concessoes, c)
			return nil
		},
		atribuir: func(_ context.Context, u acesso.Usuario, at acesso.Atribuicao) error {
			r.atribuicoes[u.ID] = at
			return nil
		},
	}
}

func TestSemearAcessoDev(t *testing.T) {
	repo := admin.NovaEmMemoria()
	var rbac rbacGravado
	deps := rbac.acessoDev()
	for range 2 {
		if err := semearAcessoDev(t.Context(), repo, deps, "cond-001"); err != nil {
			t.Fatal(err)
		}
	}
	m, err := repo.Ler(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, 0, len(m.Administradoras))
	for _, a := range m.Administradoras {
		ids = append(ids, a.ID)
	}
	slices.Sort(ids)
	if !slices.Equal(ids, []string{"adm-avancado", "adm-um-condominio"}) {
		t.Fatalf("administradoras %v, quer adm-avancado e adm-um-condominio", ids)
	}
	if len(m.Condominios) != 1 || m.Condominios[0].Administradora != "adm-um-condominio" {
		t.Fatalf("condomínios %+v, quer só o único da adm-um-condominio", m.Condominios)
	}
	quer := acesso.Concessao{AdministradoraID: "adm-avancado", PapelID: "gestor_administradora", Permissao: acesso.StudioAvancado}
	if !slices.Contains(rbac.concessoes, quer) {
		t.Fatalf("concessões %+v, quer %+v", rbac.concessoes, quer)
	}
}

func TestSemearAcessoDevRestringeOsLogins(t *testing.T) {
	var rbac rbacGravado
	if err := semearAcessoDev(t.Context(), admin.NovaEmMemoria(), rbac.acessoDev(), "cond-001"); err != nil {
		t.Fatal(err)
	}
	casos := map[string]acesso.Atribuicao{
		"gestor-condominio":    {PapelID: "gestor_administradora", Escopo: acesso.Escopo{Tipo: acesso.TipoCondominio, ID: "cond-001"}, AdministradoraID: "adm-exemplo"},
		"gestor-um-condominio": {PapelID: "gestor_administradora", Escopo: acesso.Escopo{Tipo: acesso.TipoAdministradora, ID: "adm-um-condominio"}},
	}
	for id, quer := range casos {
		if got := rbac.atribuicoes[id]; got != quer {
			t.Errorf("%s: atribuição %+v, quer %+v", id, got, quer)
		}
		if !semeadoDev(id) {
			t.Errorf("%s fora dos usuários semeados de dev: entraria fora de AUTH_MODE=dev", id)
		}
	}
}
