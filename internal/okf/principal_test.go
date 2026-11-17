package okf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestGerarPrincipalIgualAoArquivoAtual(t *testing.T) {
	fixture, err := LerRecursosFixture("../../okf/nexus/cond-001.yaml")
	if err != nil {
		t.Fatal(err)
	}
	gravam := map[string]bool{}
	for _, r := range fixture {
		gravam[r.Name] = r.Write
	}
	escrita := func(recurso string) bool { return gravam[recurso] }

	for _, agente := range []string{"agendar-mudanca", "atualizacao-cadastral", "comunicar-obra", "reserva-area-comum"} {
		t.Run(agente, func(t *testing.T) {
			dir := filepath.Join("../../okf/agentes/concierge/agentes", agente)
			raw, err := os.ReadFile(filepath.Join(dir, "caso.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			want, err := os.ReadFile(filepath.Join(dir, "workflows", "principal.md"))
			if err != nil {
				t.Fatal(err)
			}
			var cabecalho struct {
				Name        string `yaml:"name"`
				Description string `yaml:"description"`
			}
			if err := yaml.Unmarshal(raw, &cabecalho); err != nil {
				t.Fatal(err)
			}
			fluxo, err := FluxoDeYAML(raw, escrita)
			if err != nil {
				t.Fatal(err)
			}
			got := GerarPrincipal(cabecalho.Name, cabecalho.Description, fluxo.Blocos)
			if got != string(want) {
				t.Errorf("principal.md gerado difere do atual\n--- got\n%s\n--- want\n%s", got, want)
			}
		})
	}
}

func TestGerarPrincipalPassos(t *testing.T) {
	tests := []struct {
		name  string
		bloco Bloco
		want  string
	}{
		{"escolha sem reaproveitar nem se vazio", Bloco{Tipo: BlocoEscolha, Param: "area", De: "areas", Rotulo: "nome", Pergunta: "Qual área?"},
			`Defina ` + "`area`" + ` a partir de ` + "`areas`" + ` (mostre ` + "`nome`" + `): com UMA só opção, use-a sem perguntar; com várias, pergunte "Qual área?" oferecendo as opções.`},
		{"escolha sem pergunta usa o título", Bloco{Tipo: BlocoEscolha, Param: "u", Titulo: "Unidade", De: "us", Rotulo: "label"},
			`Defina ` + "`u`" + ` a partir de ` + "`us`" + ` (mostre ` + "`label`" + `): com UMA só opção, use-a sem perguntar; com várias, pergunte "Qual unidade?" oferecendo as opções.`},
		{"escolha sem pergunta nem título usa o param", Bloco{Tipo: BlocoEscolha, Param: "Area", De: "as", Rotulo: "nome"},
			`Defina ` + "`Area`" + ` a partir de ` + "`as`" + ` (mostre ` + "`nome`" + `): com UMA só opção, use-a sem perguntar; com várias, pergunte "Qual area?" oferecendo as opções.`},
		{"montar texto colapsa espaços", Bloco{Tipo: BlocoMontarTexto, Texto: "uma\n  frase   longa"},
			`Redija com o modelo, seguindo: "uma frase longa".`},
		{"reaproveitar com pergunta", Bloco{Tipo: BlocoReaproveitar, Campos: []string{"a", "b"}, Pergunta: "Posso usar?", Confirmacao: "ok"},
			"Se a conversa já tem `a`, `b`, pergunte \"Posso usar?\" (sim/não, guardado em `ok`); com o sim, reaproveite sem perguntar de novo."},
		{"reaproveitar sem pergunta", Bloco{Tipo: BlocoReaproveitar, Campos: []string{"a"}, Confirmacao: "ok"},
			"Se a conversa já tem `a`, pergunte se pode reaproveitar (sim/não, guardado em `ok`); com o sim, reaproveite sem perguntar de novo."},
		{"buscar sem com nem guardar", Bloco{Tipo: BlocoBuscar, Recurso: "x"},
			"Chame `x` (chamar_recurso)."},
		{"gravar sem com nem guardar", Bloco{Tipo: BlocoGravar, Recurso: "x"},
			"ESCRITA: chame `x`. Só depois do confirm; o protocolo devolvido vai na resposta."},
		{"com não string sai como JSON", Bloco{Tipo: BlocoBuscar, Recurso: "x", Com: []CampoValor{
			{Campo: "visitantes", Valor: []any{map[string]any{"name": "{{.visitante_nome}}"}}},
			{Campo: "unidade", Valor: "{{.u}}"},
		}}, "Chame `x` (chamar_recurso) com `visitantes` = " + `[{"name":"{{.visitante_nome}}"}]` + ", `unidade` = {{.u}}."},
		{"formulario", Bloco{Tipo: BlocoFormulario, De: "campos", GuardarEm: "dados"},
			"Monte o formulário dinâmico com os campos de `campos` e guarde as alterações em `dados`."},
		{"responder", Bloco{Tipo: BlocoResponder, Texto: "Feito.\n Obrigado"},
			"Responda no formato: Feito. Obrigado"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GerarPrincipal("T", "", []Bloco{tt.bloco})
			want := "# Workflow — T\n\n" + passosIntro + "\n\n1. " + tt.want + "\n"
			if got != want {
				t.Errorf("got\n%s\nwant\n%s", got, want)
			}
		})
	}
}

func TestGerarPrincipalDescricaoVaziaOmiteParagrafo(t *testing.T) {
	got := GerarPrincipal("T", "  \n ", nil)
	if strings.Contains(got, "\n\n\n") || !strings.HasPrefix(got, "# Workflow — T\n\nA ordem") {
		t.Errorf("parágrafo da descrição deveria ser omitido:\n%s", got)
	}
}
