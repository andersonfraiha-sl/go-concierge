package acesso_test

import (
	"errors"
	"os"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"go-concierge/internal/acesso"
	"go-concierge/internal/core/banco/bancotest"
	"go-concierge/internal/core/banco/ent"
	acessopg "go-concierge/internal/core/dados/postgres/acesso"
	"go-concierge/internal/escopo"
)

func TestMain(m *testing.M) { codigo := m.Run(); bancotest.Encerrar(); os.Exit(codigo) }

func semear(t *testing.T, cli *ent.Client) {
	t.Helper()
	repo := acessopg.NovoPostgres(cli)
	if err := repo.Atribuir(t.Context(), acesso.Usuario{ID: "u1", Nome: "Ana"}, acesso.Atribuicao{PapelID: "gestor_administradora", Escopo: acesso.Escopo{Tipo: acesso.TipoAdministradora, ID: "Nexus / A"}}); err != nil {
		t.Fatal(err)
	}
	if err := repo.Conceder(t.Context(), acesso.Concessao{AdministradoraID: "Nexus / A", PapelID: "gestor_administradora", Permissao: acesso.StudioAvancado}); err != nil {
		t.Fatal(err)
	}
}

func TestCarregarMontaAcessoDoBanco(t *testing.T) {
	b := bancotest.Novo(t)
	semear(t, b.Ent)
	a, err := acessopg.NovoPostgres(b.Ent).Carregar(t.Context(), "u1", escopo.Implantacao{})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		perm acesso.Permissao
		alvo acesso.Escopo
		want bool
	}{
		{"gestor habilita sua empresa", acesso.CondominiosHabilitar, acesso.Escopo{Tipo: acesso.TipoAdministradora, ID: "Nexus / A"}, true},
		{"gestor herda operador", acesso.SkillsLer, acesso.EscopoCondominio("Nexus / A", "101"), true},
		{"concessão alcança condomínio explícito", acesso.StudioAvancado, acesso.EscopoCondominio("Nexus / A", "101"), true},
		{"mesmo ID ERP em outra empresa é negado", acesso.SkillsLer, acesso.EscopoCondominio("B", "101"), false},
		{"condomínio sem empresa é negado", acesso.SkillsLer, acesso.Escopo{Tipo: acesso.TipoCondominio, ID: "101"}, false},
		{"gestor não herda administrador", acesso.UsuariosEditar, acesso.Escopo{Tipo: acesso.TipoAdministradora, ID: "Nexus / A"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := acesso.Permitido(a, tt.perm, tt.alvo); got != tt.want {
				t.Fatalf("Permitido=%v, want %v", got, tt.want)
			}
		})
	}
}

func TestCarregarNaoDependeDeCadastroLocalDeCondominio(t *testing.T) {
	b := bancotest.Novo(t)
	semear(t, b.Ent)
	a, err := acessopg.NovoPostgres(b.Ent).Carregar(t.Context(), "u1", escopo.Implantacao{})
	if err != nil {
		t.Fatal(err)
	}
	if !acesso.Permitido(a, acesso.SkillsLer, acesso.EscopoCondominio("Nexus / A", "987654")) {
		t.Fatal("empresa autorizada não alcançou alvo ERP explícito")
	}
	if acesso.Permitido(a, acesso.SkillsLer, acesso.EscopoCondominio("B", "987654")) {
		t.Fatal("empresa B herdou atribuição de A")
	}
}

func TestCarregarUsuarioDesconhecido(t *testing.T) {
	b := bancotest.Novo(t)
	_, err := acessopg.NovoPostgres(b.Ent).Carregar(t.Context(), "ninguem", escopo.Implantacao{})
	if !errors.Is(err, acessopg.ErrUsuarioDesconhecido) {
		t.Fatalf("Carregar=%v", err)
	}
}

func TestBancoNovoNaoConcedeOperadoresDeDesenvolvimento(t *testing.T) {
	b := bancotest.Novo(t)
	for _, id := range []string{"admin-superlogica", "construtor-superlogica", "gestor-exemplo", "gestor-avancado"} {
		if _, err := acessopg.NovoPostgres(b.Ent).Carregar(t.Context(), id, escopo.Implantacao{}); !errors.Is(err, acessopg.ErrUsuarioDesconhecido) {
			t.Fatalf("operador %s criado implicitamente: %v", id, err)
		}
	}
}

func TestConcederEIdempotente(t *testing.T) {
	b := bancotest.Novo(t)
	semear(t, b.Ent)
	repo := acessopg.NovoPostgres(b.Ent)
	grant := acesso.Concessao{AdministradoraID: "B", PapelID: "gestor_administradora", Permissao: acesso.StudioAvancado}
	for range 2 {
		if err := repo.Conceder(t.Context(), grant); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := b.Pool.QueryRow(t.Context(), `SELECT count(*) FROM company_role_grants WHERE company_id='B'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("concessões=%d (%v)", count, err)
	}
	if err := repo.Atribuir(t.Context(), acesso.Usuario{ID: "u2", Nome: "Bia"}, acesso.Atribuicao{PapelID: "gestor_administradora", Escopo: acesso.Escopo{Tipo: acesso.TipoAdministradora, ID: "B"}}); err != nil {
		t.Fatal(err)
	}
	a, err := repo.Carregar(t.Context(), "u2", escopo.Implantacao{})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(acesso.PermissoesEmAlgum(a), acesso.StudioAvancado) {
		t.Fatal("concessão empresarial não foi carregada")
	}
	if acesso.Permitido(a, acesso.StudioAvancado, acesso.EscopoCondominio("Nexus / A", "101")) {
		t.Fatal("concessão de B ampliou A")
	}
}

func TestPapeisSemeadosIguaisAosIniciais(t *testing.T) {
	b := bancotest.Novo(t)
	semear(t, b.Ent)
	a, err := acessopg.NovoPostgres(b.Ent).Carregar(t.Context(), "u1", escopo.Implantacao{})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range acesso.PapeisIniciais() {
		got, ok := a.Papeis[want.ID]
		if !ok {
			t.Fatalf("papel %s ausente", want.ID)
		}
		slices.Sort(got.Permissoes)
		slices.Sort(want.Permissoes)
		if got.HerdaDe != want.HerdaDe || !slices.Equal(got.Permissoes, want.Permissoes) {
			t.Fatalf("papel %s: %+v, want %+v", want.ID, got, want)
		}
	}
	if len(a.Papeis) != len(acesso.PapeisIniciais()) {
		t.Fatalf("papéis inesperados=%+v", a.Papeis)
	}
}

func TestBancoRecusaEscopoInvalido(t *testing.T) {
	b := bancotest.Novo(t)
	semear(t, b.Ent)
	tests := []struct {
		name, scope string
		company     any
		condo       any
	}{
		{"tipo desconhecido", "PLANET", nil, nil}, {"empresa ausente", "COMPANY", nil, nil}, {"empresa vazia", "COMPANY", " ", nil},
		{"plataforma com empresa", "PLATFORM", "A", nil}, {"plataforma com condomínio", "PLATFORM", nil, 101},
		{"condomínio ausente", "CONDO", "A", nil}, {"empresa condominial ausente", "CONDO", nil, 101}, {"empresa com condomínio", "COMPANY", "A", 101},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := b.Pool.Exec(t.Context(), `INSERT INTO role_assignments(user_id,role_id,scope_type,company_id,condo_id) VALUES('u1','gestor_administradora',$1,$2,$3)`, tt.scope, tt.company, tt.condo)
			var pgerr *pgconn.PgError
			if !errors.As(err, &pgerr) || pgerr.Code != "23514" {
				t.Fatalf("atribuição inválida: %v", err)
			}
		})
	}
}

func TestAtribuicaoCondominialNaoAtravessaEmpresaComMesmoID(t *testing.T) {
	b := bancotest.Novo(t)
	semear(t, b.Ent)
	repo := acessopg.NovoPostgres(b.Ent)
	at := acesso.Atribuicao{PapelID: "gestor_administradora", Escopo: acesso.EscopoCondominio("Nexus / A", "101"), AdministradoraID: "Nexus / A"}
	if err := repo.Atribuir(t.Context(), acesso.Usuario{ID: "u2", Nome: "Bia"}, at); err != nil {
		t.Fatal(err)
	}
	a, err := repo.Carregar(t.Context(), "u2", escopo.Implantacao{})
	if err != nil {
		t.Fatal(err)
	}
	if !acesso.Permitido(a, acesso.SkillsLer, at.Escopo) {
		t.Fatal("alvo próprio negado")
	}
	if acesso.Permitido(a, acesso.SkillsLer, acesso.EscopoCondominio("B", "101")) {
		t.Fatal("mesmo número em B herdou autorização")
	}
}

func TestPapeisIrmaosHerdamDoMesmoPai(t *testing.T) {
	b := bancotest.Novo(t)
	semear(t, b.Ent)
	for _, id := range []string{"auditor", "suporte"} {
		if _, err := b.Pool.Exec(t.Context(), `INSERT INTO roles(id,inherits_from) VALUES($1,'operador_administradora')`, id); err != nil {
			t.Fatal(err)
		}
	}
	a, err := acessopg.NovoPostgres(b.Ent).Carregar(t.Context(), "u1", escopo.Implantacao{})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"auditor", "suporte"} {
		if a.Papeis[id].HerdaDe != "operador_administradora" {
			t.Fatalf("pai de %s perdido", id)
		}
	}
}

func TestCarregarFalhaFechadoComCicloDeHeranca(t *testing.T) {
	b := bancotest.Novo(t)
	semear(t, b.Ent)
	if _, err := b.Pool.Exec(t.Context(), `UPDATE roles SET inherits_from='admin_superlogica' WHERE id='operador_administradora'`); err != nil {
		t.Fatal(err)
	}
	if _, err := acessopg.NovoPostgres(b.Ent).Carregar(t.Context(), "u1", escopo.Implantacao{}); !errors.Is(err, acesso.ErrCicloHeranca) {
		t.Fatalf("Carregar=%v", err)
	}
}

func TestCarregarTrazNome(t *testing.T) {
	b := bancotest.Novo(t)
	semear(t, b.Ent)
	a, err := acessopg.NovoPostgres(b.Ent).Carregar(t.Context(), "u1", escopo.Implantacao{})
	if err != nil || a.Nome != "Ana" {
		t.Fatalf("Nome=%q (%v)", a.Nome, err)
	}
}

func TestAtribuirRestringeAoCondominioEIdempotente(t *testing.T) {
	b := bancotest.Novo(t)
	semear(t, b.Ent)
	repo := acessopg.NovoPostgres(b.Ent)
	user := acesso.Usuario{ID: "sindico", Nome: "Síndico Alpha"}
	at := acesso.Atribuicao{PapelID: "gestor_administradora", Escopo: acesso.EscopoCondominio("Nexus / A", "101"), AdministradoraID: "Nexus / A"}
	for range 2 {
		if err := repo.Atribuir(t.Context(), user, at); err != nil {
			t.Fatal(err)
		}
	}
	a, err := repo.Carregar(t.Context(), user.ID, escopo.Implantacao{})
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Atribuicoes) != 1 {
		t.Fatalf("atribuições=%+v", a.Atribuicoes)
	}
	for _, alvo := range []acesso.Escopo{acesso.EscopoCondominio("Nexus / A", "202"), acesso.EscopoCondominio("B", "101"), {Tipo: acesso.TipoAdministradora, ID: "Nexus / A"}} {
		if acesso.Permitido(a, acesso.CondominiosHabilitar, alvo) {
			t.Fatalf("atribuição restrita ampliou %+v", alvo)
		}
	}
	if !acesso.Permitido(a, acesso.CondominiosHabilitar, at.Escopo) {
		t.Fatal("alvo próprio negado")
	}
}

func TestAtribuirRecusaCondominioSemEmpresaExplicita(t *testing.T) {
	b := bancotest.Novo(t)
	repo := acessopg.NovoPostgres(b.Ent)
	for _, at := range []acesso.Atribuicao{
		{PapelID: "gestor_administradora", Escopo: acesso.Escopo{Tipo: acesso.TipoCondominio, ID: "101"}, AdministradoraID: "A"},
		{PapelID: "gestor_administradora", Escopo: acesso.EscopoCondominio("A", "101"), AdministradoraID: "B"},
	} {
		if err := repo.Atribuir(t.Context(), acesso.Usuario{ID: "nao-criar", Nome: "Inválido"}, at); err == nil {
			t.Fatal("atribuição inválida aceita")
		}
	}
	if _, err := repo.Carregar(t.Context(), "nao-criar", escopo.Implantacao{}); !errors.Is(err, acessopg.ErrUsuarioDesconhecido) {
		t.Fatalf("falha criou usuário parcial: %v", err)
	}
}
