package copiloto

import (
	"reflect"
	"slices"
	"testing"

	"go-concierge/internal/okf"
)

// permissoesDoGestor espelha euDev("gestor") de cmd/okf-admin/studio.go.
var permissoesDoGestor = []string{
	"skills.ler", "conversas.ver", "conversas.atender", "pedidos.ver", "memoria.ver",
	"condominios.habilitar", "skills.editar_textos", "pedidos.editar", "memoria.esquecer",
}

var todasAsPermissoes = []string{
	"skills.editar_textos", "skills.editar_fluxo", "skills.criar", "skills.excluir",
	"condominios.habilitar", "studio.avancado",
}

func ptr[T any](v T) *T { return &v }

func TestPermissaoDeCadaOperacao(t *testing.T) {
	casos := []struct {
		tipo TipoDeOperacao
		quer string
	}{
		{PreencherTextos, "skills.editar_textos"},
		{TrocarFluxo, "skills.editar_fluxo"},
		{CriarSkill, "skills.criar"},
		{ExcluirSkill, "skills.excluir"},
		{HabilitarSkill, "condominios.habilitar"},
		{Orientar, "condominios.habilitar"},
		{Ajustar, "condominios.habilitar"},
		{EditarPersona, "skills.editar_textos"},
		{EditarArquivo, "studio.avancado"},
		{TipoDeOperacao("inventada"), ""},
	}
	for _, c := range casos {
		t.Run(string(c.tipo), func(t *testing.T) {
			if got := PermissaoDe(c.tipo); got != c.quer {
				t.Errorf("PermissaoDe(%q) = %q, quer %q", c.tipo, got, c.quer)
			}
		})
	}
}

func TestValidarForma(t *testing.T) {
	casos := []struct {
		nome    string
		op      Operacao
		querErr bool
	}{
		{"habilitar sem habilitado", Operacao{Tipo: HabilitarSkill, Skill: "boleto", Condominio: "c1"}, true},
		{"habilitar completo", Operacao{Tipo: HabilitarSkill, Skill: "boleto", Condominio: "c1", Habilitado: ptr(false)}, false},
		{"orientar sem condominio", Operacao{Tipo: Orientar, Texto: "Seja breve."}, true},
		{"orientar geral sem skill", Operacao{Tipo: Orientar, Condominio: "c1", Texto: "Seja breve."}, false},
		{"editar arquivo sem caminho", Operacao{Tipo: EditarArquivo, Conteudo: "x"}, true},
		{"editar arquivo completo", Operacao{Tipo: EditarArquivo, Caminho: "persona.md", Conteudo: "x"}, false},
		{"preencher textos sem textos", Operacao{Tipo: PreencherTextos, Skill: "boleto"}, true},
		{"preencher textos completo", Operacao{Tipo: PreencherTextos, Skill: "boleto", Textos: &TextosParciais{Resumo: ptr("Envia a segunda via.")}}, false},
		{"trocar fluxo sem fluxo", Operacao{Tipo: TrocarFluxo, Skill: "boleto"}, true},
		{"criar skill sem titulo", Operacao{Tipo: CriarSkill}, true},
		{"excluir skill sem skill", Operacao{Tipo: ExcluirSkill}, true},
		{"ajustar sem campos", Operacao{Tipo: Ajustar, Condominio: "c1", Recurso: "boletos.listar"}, true},
		{"ajustar completo", Operacao{Tipo: Ajustar, Condominio: "c1", Recurso: "boletos.listar", Campos: map[string]any{"dias": 5}}, false},
		{"editar persona sem texto", Operacao{Tipo: EditarPersona, Agente: "concierge"}, true},
		{"tipo desconhecido", Operacao{Tipo: "inventada"}, true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if err := ValidarForma(c.op); (err != nil) != c.querErr {
				t.Errorf("ValidarForma(%+v) = %v, querErr %v", c.op, err, c.querErr)
			}
		})
	}
}

func fluxoValido() *okf.Fluxo {
	return &okf.Fluxo{Blocos: []okf.Bloco{{ID: "b1", Tipo: okf.BlocoResponder, Texto: "Pronto."}}}
}

func TestFiltrarPorPermissaoEEscopo(t *testing.T) {
	gestor := Quem{Permissoes: permissoesDoGestor, Administradora: "adm-exemplo"}
	dele := func(id string) bool { return id == "c-dele" }
	casos := []struct {
		nome   string
		ctx    Contexto
		op     Operacao
		motivo string // "" = aceita
	}{
		{"habilita em condominio dele", Contexto{Tela: TelaCondominios},
			Operacao{Tipo: HabilitarSkill, Skill: "boleto", Condominio: "c-dele", Habilitado: ptr(true)}, ""},
		{"habilita em condominio de outra administradora", Contexto{Tela: TelaCondominios},
			Operacao{Tipo: HabilitarSkill, Skill: "boleto", Condominio: "c-outra", Habilitado: ptr(true)}, motivoForaDoEscopo},
		{"troca fluxo sem permissao", Contexto{Tela: TelaFluxo, Skill: "boleto"},
			Operacao{Tipo: TrocarFluxo, Skill: "boleto", Fluxo: fluxoValido()}, motivoSemPermissao},
		{"edita arquivo sem permissao", Contexto{Tela: TelaAvancado},
			Operacao{Tipo: EditarArquivo, Caminho: "persona.md", Conteudo: "x"}, motivoSemPermissao},
		{"preenche textos do catalogo sendo da administradora", Contexto{Tela: TelaSkill, Skill: "boleto"},
			Operacao{Tipo: PreencherTextos, Skill: "boleto", Textos: &TextosParciais{Resumo: ptr("r")}}, motivoSoSuperlogica},
		{"orienta outro condominio na ficha aberta", Contexto{Tela: TelaCondominio, Condominio: "c-dele"},
			Operacao{Tipo: Orientar, Condominio: "c-dele-2", Texto: "t"}, motivoForaDoEscopo},
		{"orienta a ficha aberta", Contexto{Tela: TelaCondominio, Condominio: "c-dele"},
			Operacao{Tipo: Orientar, Condominio: "c-dele", Texto: "t"}, ""},
		{"habilita incompleto", Contexto{Tela: TelaCondominios},
			Operacao{Tipo: HabilitarSkill, Skill: "boleto", Condominio: "c-dele"}, motivoIncompleta},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			aceitas, recusas := Filtrar([]Operacao{c.op}, c.ctx, gestor, dele)
			if c.motivo == "" {
				if len(aceitas) != 1 || len(recusas) != 0 {
					t.Fatalf("quer aceita, veio aceitas=%v recusas=%v", aceitas, recusas)
				}
				return
			}
			quer := []Recusa{{Indice: 0, Motivo: c.motivo}}
			if len(aceitas) != 0 || !reflect.DeepEqual(recusas, quer) {
				t.Fatalf("quer recusa %q, veio aceitas=%v recusas=%v", c.motivo, aceitas, recusas)
			}
		})
	}
}

func TestFiltrarCatalogoParaSuperlogica(t *testing.T) {
	superlogica := Quem{Permissoes: todasAsPermissoes}
	ops := []Operacao{
		{Tipo: TrocarFluxo, Skill: "boleto", Fluxo: fluxoValido()},
		{Tipo: PreencherTextos, Skill: "boleto", Textos: &TextosParciais{Titulo: ptr("Boleto")}},
	}
	aceitas, recusas := Filtrar(ops, Contexto{Tela: TelaFluxo, Skill: "boleto"}, superlogica, func(string) bool { return true })
	if len(aceitas) != 2 || len(recusas) != 0 {
		t.Fatalf("aceitas=%v recusas=%v", aceitas, recusas)
	}
}

func TestFiltrarForaDaTela(t *testing.T) {
	superlogica := Quem{Permissoes: todasAsPermissoes}
	ops := []Operacao{
		{Tipo: CriarSkill, Titulo: "Nova"},
		{Tipo: PreencherTextos, Skill: "boleto", Textos: &TextosParciais{Titulo: ptr("Boleto")}},
		{Tipo: EditarArquivo, Caminho: "persona.md"},
	}
	aceitas, recusas := Filtrar(ops, Contexto{Tela: TelaSkill, Skill: "boleto"}, superlogica, func(string) bool { return true })
	if len(aceitas) != 1 || aceitas[0].Tipo != PreencherTextos {
		t.Errorf("aceitas = %v, quer só preencher_textos", aceitas)
	}
	quer := []Recusa{{Indice: 0, Motivo: motivoForaDaTela}, {Indice: 2, Motivo: motivoForaDaTela}}
	if !reflect.DeepEqual(recusas, quer) {
		t.Errorf("recusas = %v, quer %v", recusas, quer)
	}
}

func TestFiltrarNaFerramentasRecusaTudo(t *testing.T) {
	aceitas, recusas := Filtrar([]Operacao{{Tipo: CriarSkill, Titulo: "x"}}, Contexto{Tela: TelaFerramentas}, Quem{Permissoes: todasAsPermissoes}, func(string) bool { return true })
	if len(aceitas) != 0 || len(recusas) != 1 {
		t.Errorf("aceitas=%v recusas=%v", aceitas, recusas)
	}
}

func TestValidarRecusaFluxoSemConfirmar(t *testing.T) {
	// O modelo marca a ferramenta como busca; o servidor reclassifica pelo nome e acha a gravação.
	proposto := okf.Fluxo{Blocos: []okf.Bloco{
		{ID: "b1", Tipo: okf.BlocoBuscar, Recurso: "boletos.emitir-segunda-via"},
		{ID: "b2", Tipo: okf.BlocoResponder, Texto: "Enviei."},
	}}
	problemas := ValidarFluxoProposto(proposto, okf.EscritaPeloNome)
	semConfirmar := slices.ContainsFunc(problemas, func(p okf.ProblemaFluxo) bool {
		return p.Codigo == "GRAVAR_SEM_CONFIRMAR" && p.Nivel == "erro" && p.BlocoID == "b1"
	})
	if !semConfirmar {
		t.Errorf("problemas = %v, quer GRAVAR_SEM_CONFIRMAR de nível erro em b1", problemas)
	}
}

func TestValidarFluxoPropostoAceitaComConfirmar(t *testing.T) {
	proposto := okf.Fluxo{Blocos: []okf.Bloco{
		{ID: "b1", Tipo: okf.BlocoConfirmar, Texto: "Posso emitir?"},
		{ID: "b2", Tipo: okf.BlocoBuscar, Recurso: "boletos.emitir-segunda-via"},
		{ID: "b3", Tipo: okf.BlocoResponder, Texto: "Enviei."},
	}}
	if problemas := ValidarFluxoProposto(proposto, okf.EscritaPeloNome); len(problemas) != 0 {
		t.Errorf("problemas = %v, quer nenhum", problemas)
	}
}
