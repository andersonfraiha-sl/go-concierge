package harness

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"go-concierge/internal/okf"
)

const principalEmArquivo = "# Principal em arquivo\n\n1. Siga o arquivo.\n"

// catalogoDeSkills é um operador com duas skills: `mudanca`, que tem um procedimento extra
// além do principal.md, e `outra`.
func catalogoDeSkills(t *testing.T) (*okf.Catalogo, *okf.Agent) {
	t.Helper()
	root := t.TempDir()
	escreve(t, root, "agentes/chefe/agent.yaml", "nome: chefe\ntipo: operador\ndescricao: chefe das skills\n")
	escreve(t, root, "agentes/chefe/workflows/principal.md", "# Workflow\n\n1. Delegue.\n")
	escreve(t, root, "agentes/chefe/agentes/mudanca/agent.yaml", "nome: mudanca\ndescricao: mudança em arquivo\n")
	escreve(t, root, "agentes/chefe/agentes/mudanca/workflows/principal.md", principalEmArquivo)
	escreve(t, root, "agentes/chefe/agentes/mudanca/workflows/extra.md", "# Procedimento extra\n\nPasso a passo extra.\n")
	escreve(t, root, "agentes/chefe/agentes/outra/agent.yaml", "nome: outra\ndescricao: outra skill\n")
	escreve(t, root, "agentes/chefe/agentes/outra/workflows/principal.md", "# Workflow\n\n1. Outra.\n")
	cat, err := okf.NewCatalogo(root)
	if err != nil {
		t.Fatal(err)
	}
	a, err := cat.Load(filepath.Join(root, "agentes", "chefe"))
	if err != nil {
		t.Fatal(err)
	}
	return cat, a
}

func efetivaDeMudanca() okf.SkillEfetiva {
	return okf.SkillEfetiva{
		Caso:      &okf.Caso{ID: "agendar_mudanca", Name: "Mudança no Solar", Description: "Agenda a mudança."},
		Descricao: "mudança efetiva",
		Principal: "# Principal efetivo\n\n1. Siga a camada.\n",
	}
}

// skillsDe devolve a consulta com as skills dadas; o resto não tem versão efetiva.
func skillsDe(efetivas map[string]okf.SkillEfetiva) func(string) (okf.SkillEfetiva, bool) {
	return func(nome string) (okf.SkillEfetiva, bool) {
		e, ok := efetivas[nome]
		return e, ok
	}
}

func construirComSkills(t *testing.T, cfg Config) (*Agente, *okf.Agent) {
	t.Helper()
	cat, a := catalogoDeSkills(t)
	cfg.Mock, cfg.Env = true, semEnv
	ag, err := Build(context.Background(), cat, a, cfg)
	if err != nil {
		t.Fatal(err)
	}
	return ag, a
}

func TestBuildUsaOCasoEfetivo(t *testing.T) {
	efetivas := map[string]okf.SkillEfetiva{"mudanca": efetivaDeMudanca(), "outra": {Descricao: "outra efetiva"}}

	ag, original := construirComSkills(t, Config{Skills: skillsDe(efetivas)})

	sub, ok := ag.Subagentes["mudanca"]
	if !ok {
		t.Fatal("skill com versão efetiva deveria ser montada")
	}
	if sub.Def.Caso == nil || sub.Def.Caso.Name != "Mudança no Solar" || sub.Def.Descricao != "mudança efetiva" {
		t.Errorf("caso e descrição do subagente = %+v / %q", sub.Def.Caso, sub.Def.Descricao)
	}
	for _, quer := range []string{"Mudança no Solar", "mudança efetiva", "Siga a camada."} {
		if !strings.Contains(sub.Instrucao, quer) {
			t.Errorf("a projeção deveria citar %q:\n%s", quer, sub.Instrucao)
		}
	}
	if strings.Contains(sub.Instrucao, "Siga o arquivo.") {
		t.Error("a projeção não deveria trazer o principal.md do arquivo")
	}
	for _, s := range original.Subagentes {
		if s.Nome == "mudanca" && (s.Caso != nil || s.Descricao != "mudança em arquivo") {
			t.Errorf("o agente do catálogo foi alterado: %+v", s)
		}
	}
}

func TestBuildExcluiSkillSemVersaoEfetiva(t *testing.T) {
	efetivas := map[string]okf.SkillEfetiva{"mudanca": efetivaDeMudanca()}

	ag, _ := construirComSkills(t, Config{Skills: skillsDe(efetivas)})

	if _, tem := ag.Subagentes["outra"]; tem {
		t.Fatal("skill sem versão efetiva não pode ser oferecida")
	}
	if strings.Contains(ag.Instrucao, "`outra`") {
		t.Errorf("a instrução ainda cita a skill excluída:\n%s", ag.Instrucao)
	}
	if _, tem := ag.Subagentes["mudanca"]; !tem {
		t.Fatal("as demais continuam")
	}
}

func TestAbrirWorkflowDevolveOPrincipalEfetivo(t *testing.T) {
	efetivas := map[string]okf.SkillEfetiva{"mudanca": efetivaDeMudanca(), "outra": {}}
	ag, _ := construirComSkills(t, Config{Skills: skillsDe(efetivas)})
	sub := ag.Subagentes["mudanca"]

	principal := chama(t, sub, "abrir_workflow", map[string]any{"nome": "principal.md"})
	extra := chama(t, sub, "abrir_workflow", map[string]any{"nome": "extra"})

	if principal["conteudo"] != efetivaDeMudanca().Principal {
		t.Errorf("principal = %v, quer o efetivo", principal["conteudo"])
	}
	if !strings.Contains(extra["conteudo"].(string), "Passo a passo extra") {
		t.Errorf("os outros procedimentos continuam nos arquivos: %v", extra)
	}
}

func TestOrientacaoEfetivaVenceADoMapeamento(t *testing.T) {
	efetiva := efetivaDeMudanca()
	efetiva.Orientacao = "Orientação da camada."
	cfg := Config{
		Skills:      skillsDe(map[string]okf.SkillEfetiva{"mudanca": efetiva, "outra": {}}),
		Orientacoes: map[string]string{"mudanca": "Orientação do mapeamento."},
	}

	ag, _ := construirComSkills(t, cfg)

	instrucao := ag.Subagentes["mudanca"].Instrucao
	if !strings.Contains(instrucao, "Orientação da camada.") || strings.Contains(instrucao, "Orientação do mapeamento.") {
		t.Errorf("a orientação efetiva deveria vencer:\n%s", instrucao)
	}
}

func TestSemOrientacaoEfetivaUsaADoMapeamento(t *testing.T) {
	cfg := Config{
		Skills:      skillsDe(map[string]okf.SkillEfetiva{"mudanca": efetivaDeMudanca(), "outra": {}}),
		Orientacoes: map[string]string{"mudanca": "Orientação do mapeamento."},
	}

	ag, _ := construirComSkills(t, cfg)

	if !strings.Contains(ag.Subagentes["mudanca"].Instrucao, "Orientação do mapeamento.") {
		t.Errorf("sem orientação efetiva vale a do mapeamento:\n%s", ag.Subagentes["mudanca"].Instrucao)
	}
}

func TestSkillsNilUsaOsArquivos(t *testing.T) {
	ag, _ := construirComSkills(t, Config{})

	sub, ok := ag.Subagentes["mudanca"]
	if !ok || ag.Subagentes["outra"] == nil {
		t.Fatalf("sem Skills todas as skills do catálogo são montadas: %v", ag.Subagentes)
	}
	if !strings.Contains(sub.Instrucao, "Siga o arquivo.") || sub.Def.Descricao != "mudança em arquivo" {
		t.Errorf("sem Skills vale o arquivo:\n%s", sub.Instrucao)
	}
}
