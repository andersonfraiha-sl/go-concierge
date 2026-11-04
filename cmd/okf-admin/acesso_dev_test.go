package main

import (
	"context"
	"slices"
	"testing"

	"go-concierge/internal/acesso"
	"go-concierge/internal/admin"
)

func TestSemearAcessoDev(t *testing.T) {
	repo := admin.NovaEmMemoria()
	var recebidas []acesso.Concessao
	conceder := func(_ context.Context, c acesso.Concessao) error {
		recebidas = append(recebidas, c)
		return nil
	}
	for range 2 {
		if err := semearAcessoDev(t.Context(), repo, conceder); err != nil {
			t.Fatal(err)
		}
	}
	m, err := repo.Ler(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if n := len(m.Administradoras); n != 1 || m.Administradoras[0].ID != "adm-avancado" {
		t.Fatalf("administradoras %+v, quer só adm-avancado", m.Administradoras)
	}
	quer := acesso.Concessao{AdministradoraID: "adm-avancado", PapelID: "gestor_administradora", Permissao: acesso.StudioAvancado}
	if !slices.Contains(recebidas, quer) {
		t.Fatalf("concessões %+v, quer %+v", recebidas, quer)
	}
}
