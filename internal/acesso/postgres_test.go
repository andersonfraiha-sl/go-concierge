package acesso_test

import (
	"errors"
	"os"
	"slices"
	"testing"

	"go-concierge/internal/acesso"
	"go-concierge/internal/banco/bancotest"
	"go-concierge/internal/banco/ent"
	"go-concierge/internal/banco/ent/concessao"
	"go-concierge/internal/escopo"
)

func TestMain(m *testing.M) {
	codigo := m.Run()
	bancotest.Encerrar()
	os.Exit(codigo)
}

func semear(t *testing.T, cli *ent.Client) {
	t.Helper()
	ctx := t.Context()
	cli.Administradora.Create().SetID("A").SetNome("Adm A").SaveX(ctx)
	cli.Administradora.Create().SetID("B").SetNome("Adm B").SaveX(ctx)
	cli.Condominio.Create().SetID("c1").SetNome("C1").SetAdministradoraID("A").SaveX(ctx)
	cli.Condominio.Create().SetID("c9").SetNome("C9").SetAdministradoraID("B").SaveX(ctx)
	cli.Condominio.Create().SetID("orfao").SetNome("Sem administradora").SaveX(ctx)
	cli.Usuario.Create().SetID("u1").SetNome("Ana").SaveX(ctx)
	cli.Atribuicao.Create().SetUsuarioID("u1").SetPapelID("gestor_administradora").
		SetEscopoTipo("administradora").SetEscopoID("A").SaveX(ctx)
	cli.Concessao.Create().SetAdministradoraID("A").SetPapelID("gestor_administradora").
		SetPermissao(string(acesso.StudioAvancado)).SaveX(ctx)
}

func TestCarregarMontaAcessoDoBanco(t *testing.T) {
	b := bancotest.Novo(t)
	semear(t, b.Ent)

	a, err := acesso.NovoPostgres(b.Ent).Carregar(t.Context(), "u1", escopo.Implantacao{})
	if err != nil {
		t.Fatalf("Carregar: %v", err)
	}

	admA := acesso.Escopo{Tipo: acesso.TipoAdministradora, ID: "A"}
	tests := []struct {
		name string
		perm acesso.Permissao
		alvo acesso.Escopo
		want bool
	}{
		{"gestor habilita condominios na propria administradora", acesso.CondominiosHabilitar, admA, true},
		{"gestor herda skills.ler do operador", acesso.SkillsLer, admA, true},
		{"concessao vale no condominio da administradora", acesso.StudioAvancado, acesso.Escopo{Tipo: acesso.TipoCondominio, ID: "c1"}, true},
		{"gestor negado em condominio de outra administradora", acesso.SkillsLer, acesso.Escopo{Tipo: acesso.TipoCondominio, ID: "c9"}, false},
		{"gestor negado em condominio sem administradora", acesso.SkillsLer, acesso.Escopo{Tipo: acesso.TipoCondominio, ID: "orfao"}, false},
		{"gestor sem permissao de admin", acesso.UsuariosEditar, admA, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := acesso.Permitido(a, tt.perm, tt.alvo); got != tt.want {
				t.Errorf("Permitido(%s, %+v) = %v, want %v", tt.perm, tt.alvo, got, tt.want)
			}
		})
	}
	if got := a.AdministradoraDe["c9"]; got != "B" {
		t.Errorf("AdministradoraDe[c9] = %q, want B (condomínio fora das atribuições)", got)
	}
}

func TestCarregarSegueAdministradoraAtual(t *testing.T) {
	b := bancotest.Novo(t)
	semear(t, b.Ent)
	b.Ent.Condominio.UpdateOneID("c1").SetAdministradoraID("B").ExecX(t.Context())

	a, err := acesso.NovoPostgres(b.Ent).Carregar(t.Context(), "u1", escopo.Implantacao{})
	if err != nil {
		t.Fatalf("Carregar: %v", err)
	}
	if acesso.Permitido(a, acesso.SkillsLer, acesso.Escopo{Tipo: acesso.TipoCondominio, ID: "c1"}) {
		t.Error("c1 mudou para B e o gestor de A ainda o enxerga")
	}
}

func TestCarregarUsuarioDesconhecido(t *testing.T) {
	b := bancotest.Novo(t)

	_, err := acesso.NovoPostgres(b.Ent).Carregar(t.Context(), "ninguem", escopo.Implantacao{})
	if !errors.Is(err, acesso.ErrUsuarioDesconhecido) {
		t.Fatalf("Carregar = %v, want ErrUsuarioDesconhecido", err)
	}
}

func TestUsuarioDevSemeadoEAdminDaSuperlogica(t *testing.T) {
	b := bancotest.Novo(t)

	a, err := acesso.NovoPostgres(b.Ent).Carregar(t.Context(), "admin-superlogica", escopo.Implantacao{})
	if err != nil {
		t.Fatal(err)
	}
	if !acesso.Permitido(a, acesso.AdministradorasEditar, acesso.Escopo{Tipo: acesso.TipoSuperlogica}) {
		t.Fatalf("usuário de dev sem admin na Superlógica: %+v", a.Atribuicoes)
	}
}

func TestUsuariosDevDoStudioSemeados(t *testing.T) {
	b := bancotest.Novo(t)
	casos := map[string]string{
		"construtor-superlogica": "construtor_superlogica",
		"gestor-exemplo":         "gestor_administradora",
		"gestor-avancado":        "gestor_administradora",
	}
	for id, papel := range casos {
		a, err := acesso.NovoPostgres(b.Ent).Carregar(t.Context(), id, escopo.Implantacao{})
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		if len(a.Atribuicoes) != 1 || a.Atribuicoes[0].PapelID != papel {
			t.Fatalf("%s: atribuições %+v, quer o papel %s", id, a.Atribuicoes, papel)
		}
	}
}

func TestConcederEIdempotente(t *testing.T) {
	b := bancotest.Novo(t)
	semear(t, b.Ent)
	pg := acesso.NovoPostgres(b.Ent)
	c := acesso.Concessao{AdministradoraID: "B", PapelID: "gestor_administradora", Permissao: acesso.StudioAvancado}
	for range 2 {
		if err := pg.Conceder(t.Context(), c); err != nil {
			t.Fatal(err)
		}
	}
	if n := b.Ent.Concessao.Query().Where(concessao.AdministradoraID("B")).CountX(t.Context()); n != 1 {
		t.Fatalf("%d linhas em concessoes, quer 1", n)
	}
	b.Ent.Usuario.Create().SetID("u2").SetNome("Bia").SaveX(t.Context())
	b.Ent.Atribuicao.Create().SetUsuarioID("u2").SetPapelID("gestor_administradora").
		SetEscopoTipo("administradora").SetEscopoID("B").SaveX(t.Context())
	a, err := pg.Carregar(t.Context(), "u2", escopo.Implantacao{})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(acesso.PermissoesEmAlgum(a), acesso.StudioAvancado) {
		t.Fatalf("permissões %v sem %s", acesso.PermissoesEmAlgum(a), acesso.StudioAvancado)
	}
}

func TestPapeisSemeadosIguaisAosIniciais(t *testing.T) {
	b := bancotest.Novo(t)
	semear(t, b.Ent)

	a, err := acesso.NovoPostgres(b.Ent).Carregar(t.Context(), "u1", escopo.Implantacao{})
	if err != nil {
		t.Fatalf("Carregar: %v", err)
	}
	for _, want := range acesso.PapeisIniciais() {
		got, ok := a.Papeis[want.ID]
		if !ok {
			t.Errorf("papel %s ausente no banco", want.ID)
			continue
		}
		slices.Sort(got.Permissoes)
		slices.Sort(want.Permissoes)
		if got.HerdaDe != want.HerdaDe || !slices.Equal(got.Permissoes, want.Permissoes) {
			t.Errorf("papel %s: banco %+v, PapeisIniciais %+v", want.ID, got, want)
		}
	}
	if len(a.Papeis) != len(acesso.PapeisIniciais()) {
		t.Errorf("banco tem %d papéis, PapeisIniciais tem %d", len(a.Papeis), len(acesso.PapeisIniciais()))
	}
}

func TestBancoRecusaEscopoInvalido(t *testing.T) {
	b := bancotest.Novo(t)
	semear(t, b.Ent)

	tests := []struct{ name, tipo, id, dona string }{
		{"tipo desconhecido", "planeta", "x", ""},
		{"administradora sem id", "administradora", "", ""},
		{"superlogica com id", "superlogica", "x", ""},
		{"condominio sem id", "condominio", "", "A"},
		{"condominio sem administradora dona", "condominio", "c1", ""},
		{"administradora com dona", "administradora", "A", "A"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := b.Ent.Atribuicao.Create().SetUsuarioID("u1").SetPapelID("gestor_administradora").
				SetEscopoTipo(tt.tipo).SetEscopoID(tt.id)
			if tt.dona != "" {
				c.SetAdministradoraID(tt.dona)
			}
			if err := c.Exec(t.Context()); !ent.IsConstraintError(err) {
				t.Errorf("atribuição %q/%q/%q: erro %v, want violação de constraint", tt.tipo, tt.id, tt.dona, err)
			}
		})
	}
}

func TestCarregarNegaAtribuicaoDeCondominioQueMudouDeAdministradora(t *testing.T) {
	b := bancotest.Novo(t)
	semear(t, b.Ent)
	b.Ent.Usuario.Create().SetID("u2").SetNome("Bia").SaveX(t.Context())
	b.Ent.Atribuicao.Create().SetUsuarioID("u2").SetPapelID("gestor_administradora").
		SetEscopoTipo("condominio").SetEscopoID("c1").SetAdministradoraID("A").SaveX(t.Context())
	c1 := acesso.Escopo{Tipo: acesso.TipoCondominio, ID: "c1"}
	pg := acesso.NovoPostgres(b.Ent)

	antes, err := pg.Carregar(t.Context(), "u2", escopo.Implantacao{})
	if err != nil {
		t.Fatalf("Carregar antes: %v", err)
	}
	if !acesso.Permitido(antes, acesso.SkillsLer, c1) {
		t.Fatal("u2 deveria ler c1 enquanto c1 é de A")
	}

	b.Ent.Condominio.UpdateOneID("c1").SetAdministradoraID("B").ExecX(t.Context())
	depois, err := pg.Carregar(t.Context(), "u2", escopo.Implantacao{})
	if err != nil {
		t.Fatalf("Carregar depois: %v", err)
	}
	if acesso.Permitido(depois, acesso.SkillsLer, c1) {
		t.Error("u2 ainda lê c1 depois de c1 mudar para B")
	}
}

func TestPapeisIrmaosHerdamDoMesmoPai(t *testing.T) {
	b := bancotest.Novo(t)
	semear(t, b.Ent)
	for _, id := range []string{"auditor", "suporte"} {
		b.Ent.Papel.Create().SetID(id).SetHerdaDe("operador_administradora").SaveX(t.Context())
	}

	a, err := acesso.NovoPostgres(b.Ent).Carregar(t.Context(), "u1", escopo.Implantacao{})
	if err != nil {
		t.Fatalf("Carregar: %v", err)
	}
	if a.Papeis["auditor"].HerdaDe != "operador_administradora" || a.Papeis["suporte"].HerdaDe != "operador_administradora" {
		t.Errorf("irmãos perderam o pai: %+v %+v", a.Papeis["auditor"], a.Papeis["suporte"])
	}
}

func TestCarregarFalhaFechadoComCicloDeHeranca(t *testing.T) {
	b := bancotest.Novo(t)
	semear(t, b.Ent)
	b.Ent.Papel.UpdateOneID("operador_administradora").SetHerdaDe("admin_superlogica").ExecX(t.Context())

	_, err := acesso.NovoPostgres(b.Ent).Carregar(t.Context(), "u1", escopo.Implantacao{})
	if !errors.Is(err, acesso.ErrCicloHeranca) {
		t.Fatalf("Carregar = %v, want ErrCicloHeranca", err)
	}
}

func TestCarregarTrazNome(t *testing.T) {
	b := bancotest.Novo(t)
	semear(t, b.Ent)
	a, err := acesso.NovoPostgres(b.Ent).Carregar(t.Context(), "u1", escopo.Implantacao{})
	if err != nil {
		t.Fatalf("Carregar: %v", err)
	}
	if a.Nome != "Ana" {
		t.Fatalf("Nome = %q, quer Ana", a.Nome)
	}
}
