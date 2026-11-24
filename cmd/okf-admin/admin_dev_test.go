package main

import (
	"testing"

	"go-concierge/internal/acesso"
	"go-concierge/internal/banco/bancotest"
	"go-concierge/internal/escopo"
)

func TestGarantirAdminDevCriaAcessoGlobalERepeteSemAlterarDados(t *testing.T) {
	db := bancotest.Novo(t)
	repo := acesso.NovoPostgres(db.Ent)
	for range 2 {
		if err := garantirAdminDev(t.Context(), "dev", authModeDev, repo.Atribuir); err != nil {
			t.Fatal(err)
		}
	}
	_, err := db.Pool.Exec(t.Context(), `UPDATE users SET name='Nome preservado' WHERE id=$1`, usuarioSemeadoDev)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Atribuir(t.Context(), acesso.Usuario{ID: "outro-usuario", Nome: "Outro"}, acesso.Atribuicao{PapelID: "gestor_administradora", Escopo: acesso.Escopo{Tipo: acesso.TipoAdministradora, ID: "empresa-a"}}); err != nil {
		t.Fatal(err)
	}
	if err := garantirAdminDev(t.Context(), "dev", authModeDev, repo.Atribuir); err != nil {
		t.Fatal(err)
	}
	a, err := repo.Carregar(t.Context(), usuarioSemeadoDev, escopo.Implantacao{})
	if err != nil {
		t.Fatal(err)
	}
	if a.Nome != "Nome preservado" || len(a.Atribuicoes) != 1 || a.Atribuicoes[0].PapelID != "admin_superlogica" || a.Atribuicoes[0].Escopo.Tipo != acesso.TipoSuperlogica {
		t.Fatalf("acesso DEV inesperado: nome=%q atribuições=%+v", a.Nome, a.Atribuicoes)
	}
	if !acesso.Permitido(a, acesso.UsuariosEditar, acesso.Escopo{Tipo: acesso.TipoSuperlogica}) {
		t.Fatal("admin DEV não recebeu permissão global de usuários")
	}
	var adminUsers, adminAssignments, otherUsers, otherAssignments int
	if err := db.Pool.QueryRow(t.Context(), `SELECT count(*) FROM users WHERE id=$1`, usuarioSemeadoDev).Scan(&adminUsers); err != nil {
		t.Fatal(err)
	}
	if err := db.Pool.QueryRow(t.Context(), `SELECT count(*) FROM role_assignments WHERE user_id=$1`, usuarioSemeadoDev).Scan(&adminAssignments); err != nil {
		t.Fatal(err)
	}
	if err := db.Pool.QueryRow(t.Context(), `SELECT count(*) FROM users WHERE id='outro-usuario'`).Scan(&otherUsers); err != nil {
		t.Fatal(err)
	}
	if err := db.Pool.QueryRow(t.Context(), `SELECT count(*) FROM role_assignments WHERE user_id='outro-usuario'`).Scan(&otherAssignments); err != nil {
		t.Fatal(err)
	}
	if adminUsers != 1 || adminAssignments != 1 || otherUsers != 1 || otherAssignments != 1 {
		t.Fatalf("repetição duplicou ou removeu dados: admin=%d/%d outro=%d/%d", adminUsers, adminAssignments, otherUsers, otherAssignments)
	}
}

func TestGarantirAdminDevNaoSemeiaForaDoModoPermitido(t *testing.T) {
	db := bancotest.Novo(t)
	repo := acesso.NovoPostgres(db.Ent)
	for _, c := range []struct {
		ambiente string
		modo     string
	}{
		{ambiente: "prd", modo: authModeDev},
		{ambiente: "dev", modo: authModeProxy},
		{ambiente: "", modo: authModeProxy},
		{ambiente: "dev", modo: ""},
	} {
		if err := garantirAdminDev(t.Context(), c.ambiente, c.modo, repo.Atribuir); err != nil {
			t.Fatalf("ambiente=%q modo=%q: %v", c.ambiente, c.modo, err)
		}
	}
	var users, assignments int
	if err := db.Pool.QueryRow(t.Context(), `SELECT (SELECT count(*) FROM users),(SELECT count(*) FROM role_assignments)`).Scan(&users, &assignments); err != nil {
		t.Fatal(err)
	}
	if users != 0 || assignments != 0 {
		t.Fatalf("usuário criado fora do contexto DEV autorizado: users=%d role_assignments=%d", users, assignments)
	}
}
