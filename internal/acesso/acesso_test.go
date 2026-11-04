package acesso

import (
	"errors"
	"slices"
	"testing"

	"go-concierge/internal/escopo"
)

func papeis() map[string]Papel {
	m := map[string]Papel{}
	for _, p := range PapeisIniciais() {
		m[p.ID] = p
	}
	return m
}

func admA() Escopo          { return Escopo{TipoAdministradora, "A"} }
func cond(id string) Escopo { return Escopo{TipoCondominio, id} }

func base(atrib []Atribuicao, conc []Concessao, imp escopo.Implantacao) Acesso {
	return Acesso{
		UsuarioID:        "u1",
		Papeis:           papeis(),
		Atribuicoes:      atrib,
		Concessoes:       conc,
		AdministradoraDe: map[string]string{"c1": "A", "c2": "A", "c9": "B"},
		Implantacao:      imp,
	}
}

func TestPermitido(t *testing.T) {
	gestorA := []Atribuicao{{"gestor_administradora", admA(), ""}}
	operadorA := []Atribuicao{{"operador_administradora", admA(), ""}}
	concGestor := []Concessao{{"A", "gestor_administradora", StudioAvancado}}
	superl := []Atribuicao{{"admin_superlogica", Escopo{Tipo: TipoSuperlogica}, ""}}
	fixaA := escopo.Implantacao{AdministradoraFixa: "A"}

	tests := []struct {
		name string
		a    Acesso
		perm Permissao
		alvo Escopo
		want bool
	}{
		{"gestor herda skills.ler", base(gestorA, nil, escopo.Implantacao{}), SkillsLer, admA(), true},
		{"gestor sem concessao nega studio", base(gestorA, nil, escopo.Implantacao{}), StudioAvancado, admA(), false},
		{"concessao vale na administradora", base(gestorA, concGestor, escopo.Implantacao{}), StudioAvancado, admA(), true},
		{"concessao vale no condominio da administradora", base(gestorA, concGestor, escopo.Implantacao{}), StudioAvancado, cond("c1"), true},
		{"concessao nao vale para operador", base(operadorA, concGestor, escopo.Implantacao{}), StudioAvancado, admA(), false},
		{"concessao ao papel vale para quem herda", base([]Atribuicao{{"construtor_superlogica", admA(), ""}}, []Concessao{{"A", "gestor_administradora", UsuariosEditar}}, escopo.Implantacao{}), UsuariosEditar, admA(), true},
		{"gestor de A negado em condominio de B", base(gestorA, concGestor, escopo.Implantacao{}), SkillsLer, cond("c9"), false},
		{"gestor de A negado em administradora B", base(gestorA, nil, escopo.Implantacao{}), SkillsLer, Escopo{TipoAdministradora, "B"}, false},
		{"atribuicao em c1 vale em c1", base([]Atribuicao{{"gestor_administradora", cond("c1"), "A"}}, nil, escopo.Implantacao{}), SkillsLer, cond("c1"), true},
		{"atribuicao em c1 negada em c2 da mesma A", base([]Atribuicao{{"gestor_administradora", cond("c1"), "A"}}, nil, escopo.Implantacao{}), SkillsLer, cond("c2"), false},
		{"atribuicao em c1 negada na administradora", base([]Atribuicao{{"gestor_administradora", cond("c1"), "A"}}, nil, escopo.Implantacao{}), SkillsLer, admA(), false},
		{"concessao nao vaza para atribuicao fora da administradora", base([]Atribuicao{{"gestor_administradora", cond("c9"), "B"}}, concGestor, escopo.Implantacao{}), StudioAvancado, cond("c9"), false},
		{"admin superlogica permitido em B", base(superl, nil, escopo.Implantacao{}), UsuariosEditar, Escopo{TipoAdministradora, "B"}, true},
		{"admin superlogica herda skills.ler pela cadeia", base(superl, nil, escopo.Implantacao{}), SkillsLer, cond("c9"), true},
		{"admin superlogica negado em B com fixa A", base(superl, nil, fixaA), UsuariosEditar, Escopo{TipoAdministradora, "B"}, false},
		{"admin superlogica negado em condominio de B com fixa A", base(superl, nil, fixaA), SkillsLer, cond("c9"), false},
		{"admin superlogica negado no escopo superlogica com fixa A", base(superl, nil, fixaA), UsuariosEditar, Escopo{Tipo: TipoSuperlogica}, false},
		{"admin superlogica permitido em A com fixa A", base(superl, nil, fixaA), UsuariosEditar, admA(), true},
		{"sem atribuicoes nega", base(nil, nil, escopo.Implantacao{}), SkillsLer, admA(), false},
		{"papel desconhecido nega", base([]Atribuicao{{"fantasma", admA(), ""}}, nil, escopo.Implantacao{}), SkillsLer, admA(), false},
		{"condominio vazio nunca casa", base([]Atribuicao{{"gestor_administradora", cond(""), "A"}}, nil, escopo.Implantacao{}), SkillsLer, cond(""), false},
		{"duas atribuicoes: gestor A permite c1", base([]Atribuicao{{"gestor_administradora", admA(), ""}, {"operador_administradora", Escopo{TipoAdministradora, "B"}, ""}}, concGestor, escopo.Implantacao{}), StudioAvancado, cond("c1"), true},
		{"duas atribuicoes: concessao de A nao vale em c9 de B", base([]Atribuicao{{"gestor_administradora", admA(), ""}, {"operador_administradora", Escopo{TipoAdministradora, "B"}, ""}}, concGestor, escopo.Implantacao{}), StudioAvancado, cond("c9"), false},
		{"atribuicao de condominio sem dona nega", base([]Atribuicao{{"gestor_administradora", cond("c1"), ""}}, nil, escopo.Implantacao{}), SkillsLer, cond("c1"), false},
		{"condominio sem administradora conhecida nega", base(gestorA, nil, escopo.Implantacao{}), SkillsLer, cond("zz"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Permitido(tt.a, tt.perm, tt.alvo); got != tt.want {
				t.Errorf("Permitido(%s, %+v) = %v, want %v", tt.perm, tt.alvo, got, tt.want)
			}
		})
	}
}

func TestPermitidoEmAlgum(t *testing.T) {
	gestorB := []Atribuicao{{"gestor_administradora", Escopo{TipoAdministradora, "B"}, ""}}
	tests := []struct {
		name string
		a    Acesso
		perm Permissao
		want bool
	}{
		{"atribuicao com a permissao", base(gestorB, nil, escopo.Implantacao{}), SkillsLer, true},
		{"sem a permissao", base(gestorB, nil, escopo.Implantacao{}), StudioAvancado, false},
		{"concessao conta", base(gestorB, []Concessao{{"B", "gestor_administradora", StudioAvancado}}, escopo.Implantacao{}), StudioAvancado, true},
		{"atribuicao de outra administradora ignorada com fixa", base(gestorB, nil, escopo.Implantacao{AdministradoraFixa: "A"}), SkillsLer, false},
		{"sem atribuicoes", base(nil, nil, escopo.Implantacao{}), SkillsLer, false},
		{"atribuicao superlogica vale com fixa A", base([]Atribuicao{{"admin_superlogica", Escopo{Tipo: TipoSuperlogica}, ""}}, nil, escopo.Implantacao{AdministradoraFixa: "A"}), UsuariosEditar, true},
		{"atribuicao de tipo vazio ignorada", base([]Atribuicao{{"admin_superlogica", Escopo{}, ""}}, nil, escopo.Implantacao{}), SkillsLer, false},
		{"atribuicao de tipo desconhecido ignorada", base([]Atribuicao{{"admin_superlogica", Escopo{Tipo: "x", ID: "A"}, ""}}, nil, escopo.Implantacao{}), SkillsLer, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PermitidoEmAlgum(tt.a, tt.perm); got != tt.want {
				t.Errorf("PermitidoEmAlgum(%s) = %v, want %v", tt.perm, got, tt.want)
			}
		})
	}
}

func TestValidarHerancaCiclo(t *testing.T) {
	tests := []struct {
		name   string
		papeis []Papel
		want   error
	}{
		{"ciclo A-B-A", []Papel{{ID: "A", HerdaDe: "B"}, {ID: "B", HerdaDe: "A"}}, ErrCicloHeranca},
		{"auto heranca", []Papel{{ID: "A", HerdaDe: "A"}}, ErrCicloHeranca},
		{"cadeia valida", []Papel{{ID: "A", HerdaDe: "B"}, {ID: "B"}}, nil},
		{"vazio", nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidarHeranca(tt.papeis); !errors.Is(err, tt.want) {
				t.Errorf("ValidarHeranca = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestPapeisIniciaisSemCiclo(t *testing.T) {
	if err := ValidarHeranca(PapeisIniciais()); err != nil {
		t.Fatalf("papéis iniciais: %v", err)
	}
	if got := len(PapeisIniciais()); got != 4 {
		t.Errorf("len(PapeisIniciais) = %d, want 4", got)
	}
}

func TestAtribuicaoDeCondominioPerdeValidadeQuandoCondominioMuda(t *testing.T) {
	a := base([]Atribuicao{{"gestor_administradora", cond("c1"), "A"}}, nil, escopo.Implantacao{})
	if !Permitido(a, SkillsLer, cond("c1")) {
		t.Fatal("antes da mudança o gestor de c1 deveria ler")
	}
	a.AdministradoraDe["c1"] = "B"
	if Permitido(a, SkillsLer, cond("c1")) {
		t.Error("Permitido em c1 após mudar para B, want negado")
	}
	if PermitidoEmAlgum(a, SkillsLer) {
		t.Error("PermitidoEmAlgum com atribuição obsoleta, want negado")
	}
	a.Concessoes = []Concessao{{"B", "gestor_administradora", StudioAvancado}}
	if Permitido(a, StudioAvancado, cond("c1")) {
		t.Error("concessão da nova administradora vazou para atribuição obsoleta")
	}
}

func naAdm(papel, adm string) []Atribuicao {
	return []Atribuicao{{PapelID: papel, Escopo: Escopo{TipoAdministradora, adm}}}
}

func TestPermissoesEmAlgum(t *testing.T) {
	casos := []struct {
		nome   string
		ats    []Atribuicao
		conc   []Concessao
		tem    []Permissao
		naoTem []Permissao
	}{
		{"operador", naAdm("operador_administradora", "A"), nil, []Permissao{SkillsLer}, []Permissao{CondominiosHabilitar, StudioAvancado}},
		{"gestor sem concessão", naAdm("gestor_administradora", "A"), nil, []Permissao{SkillsEditarTextos}, []Permissao{StudioAvancado}},
		{"gestor com concessão", naAdm("gestor_administradora", "A"), []Concessao{{AdministradoraID: "A", PapelID: "gestor_administradora", Permissao: StudioAvancado}}, []Permissao{StudioAvancado}, nil},
		{"sem papel", nil, nil, nil, []Permissao{SkillsLer}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			got := PermissoesEmAlgum(base(c.ats, c.conc, escopo.Implantacao{}))
			if got == nil {
				t.Fatal("PermissoesEmAlgum = nil")
			}
			for _, p := range c.tem {
				if !slices.Contains(got, p) {
					t.Errorf("falta %s em %v", p, got)
				}
			}
			for _, p := range c.naoTem {
				if slices.Contains(got, p) {
					t.Errorf("sobra %s em %v", p, got)
				}
			}
		})
	}
}

func TestPapelPrincipal(t *testing.T) {
	casos := []struct {
		nome string
		ats  []Atribuicao
		quer string
	}{
		{"admin vence operador", []Atribuicao{{PapelID: "operador_administradora", Escopo: admA()}, {PapelID: "admin_superlogica", Escopo: Escopo{Tipo: TipoSuperlogica}}}, "admin_superlogica"},
		{"só gestor", naAdm("gestor_administradora", "A"), "gestor_administradora"},
		{"sem atribuição", nil, ""},
		{"condomínio com dona mudada é ignorado", []Atribuicao{{PapelID: "gestor_administradora", Escopo: cond("c1"), AdministradoraID: "B"}}, ""},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if got := PapelPrincipal(base(c.ats, nil, escopo.Implantacao{})); got != c.quer {
				t.Fatalf("PapelPrincipal = %q, quer %q", got, c.quer)
			}
		})
	}
}
