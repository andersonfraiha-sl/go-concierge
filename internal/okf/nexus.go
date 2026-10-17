package okf

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Recursos do Nexus: o que a account oferece em tempo de execução. O agente não
// copia nada disso para a pasta; declara em agent.yaml → recursos (e nos passos do
// caso.yaml) o que precisa, e o harness cruza a declaração com o que a account tem
// e o que a identidade do usuário pode.

// Tipos (kind) de recurso. Os quatro primeiros vêm do sl-concierge; os demais mapeiam o
// resto do que o Nexus oferece: ações (APIs que executam algo; uma atividade cadastrada
// no workspace é uma ação também), agentes executores e forms (procedimento externo,
// chamada pública).
const (
	KindAPI       = "api"
	KindDatabase  = "database"
	KindKnowledge = "knowledge"
	KindMemory    = "memory"
	KindAgent     = "agent"
	KindAction    = "action"
	KindActivity  = "activity"
	KindForm      = "form"
)

// Classes, na ordem em que a interface as mostra. A classe é o que o recurso É para o
// agente; `write` é ortogonal (um recurso de qualquer classe com write: true só entra
// por um passo do caso.yaml, depois do confirm).
var Classes = []string{"leitura", "escrita", "conhecimento", "memoria", "acao", "agente", "form"}

// Recurso é um item do catálogo de recursos de uma account no Nexus.
type Recurso struct {
	Name        string      `json:"name" yaml:"name"`
	Titulo      string      `json:"titulo,omitempty" yaml:"titulo,omitempty"` // nome comercial, para humanos ("Áreas reserváveis")
	Kind        string      `json:"kind" yaml:"kind"`
	Visibility  string      `json:"visibility" yaml:"visibility"` // public | private
	Write       bool        `json:"write" yaml:"write"`
	Description string      `json:"description" yaml:"description"`
	Params      []ParamMeta `json:"params" yaml:"params"`
	Output      string      `json:"output,omitempty" yaml:"output"` // resumo humano do retorno
	// Conexao é como o Nexus chega ao sistema por trás do recurso (método, URL com
	// {{param}}, headers, corpo). Documental aqui: quem executa é o Nexus/MCP.
	Conexao *Conexao `json:"conexao,omitempty" yaml:"conexao,omitempty"`
	// Saida mapeia o retorno em campos nomeados (caminho no JSON devolvido), para o
	// agente e a indexação semântica saberem o que vem de volta.
	Saida []SaidaCampo `json:"saida,omitempty" yaml:"saida,omitempty"`
	// Definicoes são os ajustes que um condomínio pode dar a este recurso (antecedência
	// mínima, horário, código da empresa…): o Admin os mostra como formulário, em
	// linguagem de administradora, e grava no payload do condomínio. Sem definições,
	// o recurso não aparece nos Ajustes.
	Definicoes []Definicao `json:"definicoes,omitempty" yaml:"definicoes"`
}

// Definicao é um campo de ajuste de um recurso por condomínio.
type Definicao struct {
	Name        string   `json:"name" yaml:"name"`
	Label       string   `json:"label" yaml:"label"`
	Type        string   `json:"type,omitempty" yaml:"type"` // string | number | boolean | select | text
	Options     []string `json:"options,omitempty" yaml:"options"`
	Default     any      `json:"default,omitempty" yaml:"default"`
	Description string   `json:"description,omitempty" yaml:"description"`
}

// Conexao: os dados de chamada do recurso.
type Conexao struct {
	Metodo  string            `json:"metodo,omitempty" yaml:"metodo,omitempty"`
	URL     string            `json:"url,omitempty" yaml:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	Corpo   string            `json:"corpo,omitempty" yaml:"corpo,omitempty"` // template do body (JSON com {{param}})
}

// SaidaCampo é um campo do retorno: nome para o agente, caminho no JSON (ex.: data.items[].id).
type SaidaCampo struct {
	Name        string `json:"name" yaml:"name"`
	Path        string `json:"path,omitempty" yaml:"path,omitempty"`
	Type        string `json:"type,omitempty" yaml:"type,omitempty"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

var reNomeRecurso = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]*$`)

// Validar confere o mínimo de um recurso editado na interface.
func (r Recurso) Validar() error {
	if !reNomeRecurso.MatchString(r.Name) {
		return fmt.Errorf("nome de recurso inválido: %q (minúsculas, dígitos, ponto, hífen)", r.Name)
	}
	switch r.Kind {
	case KindAPI, KindDatabase, KindKnowledge, KindMemory, KindAgent, KindAction, KindActivity, KindForm:
	default:
		return fmt.Errorf("kind desconhecido: %q", r.Kind)
	}
	if r.Visibility != "public" && r.Visibility != "private" {
		return fmt.Errorf("visibility precisa ser public ou private: %q", r.Visibility)
	}
	for _, p := range r.Params {
		if strings.TrimSpace(p.Name) == "" {
			return errors.New("parâmetro sem nome")
		}
	}
	for _, s := range r.Saida {
		if strings.TrimSpace(s.Name) == "" {
			return errors.New("campo de saída sem nome")
		}
	}
	return nil
}

// SalvarRecursoFixture grava (cria ou substitui, pelo nome original) um recurso no
// instantâneo YAML da account, preservando o cabeçalho de comentários do arquivo.
func SalvarRecursoFixture(p, nomeOriginal string, r Recurso) error {
	if err := r.Validar(); err != nil {
		return err
	}
	lista, err := LerRecursosFixture(p)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if nomeOriginal == "" {
		nomeOriginal = r.Name
	}
	trocou := false
	for i := range lista {
		if lista[i].Name == nomeOriginal {
			lista[i] = r
			trocou = true
		} else if lista[i].Name == r.Name {
			return fmt.Errorf("já existe um recurso chamado %s", r.Name)
		}
	}
	if !trocou {
		lista = append(lista, r)
	}
	return gravarRecursosFixture(p, lista)
}

// RemoverRecursoFixture tira um recurso do instantâneo.
func RemoverRecursoFixture(p, nome string) error {
	lista, err := LerRecursosFixture(p)
	if err != nil {
		return err
	}
	var resto []Recurso
	for _, r := range lista {
		if r.Name != nome {
			resto = append(resto, r)
		}
	}
	if len(resto) == len(lista) {
		return fmt.Errorf("recurso não encontrado: %s", nome)
	}
	return gravarRecursosFixture(p, resto)
}

func gravarRecursosFixture(p string, lista []Recurso) error {
	var cab strings.Builder
	if raw, err := os.ReadFile(p); err == nil {
		for _, l := range strings.Split(string(raw), "\n") {
			if !strings.HasPrefix(l, "#") {
				break
			}
			cab.WriteString(l + "\n")
		}
	}
	if lista == nil {
		lista = []Recurso{}
	}
	corpo, err := yaml.Marshal(struct {
		Recursos []Recurso `yaml:"recursos"`
	}{lista})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(cab.String()+string(corpo)), 0o644)
}

// ParamMeta descreve um parâmetro do recurso.
type ParamMeta struct {
	Name        string `json:"name" yaml:"name"`
	Type        string `json:"type,omitempty" yaml:"type"`
	Required    bool   `json:"required,omitempty" yaml:"required"`
	Description string `json:"description,omitempty" yaml:"description"`
}

// Classe devolve o grupo em que a interface mostra o recurso.
func (r Recurso) Classe() string {
	switch r.Kind {
	case KindKnowledge:
		return "conhecimento"
	case KindMemory:
		return "memoria"
	case KindAction, KindActivity:
		return "acao"
	case KindAgent:
		return "agente"
	case KindForm:
		return "form"
	}
	if r.Write {
		return "escrita"
	}
	return "leitura"
}

// LerRecursosFixture lê um arquivo `recursos: [...]` (o instantâneo de uma account,
// usado quando o Nexus não está no ar).
func LerRecursosFixture(p string) ([]Recurso, error) {
	raw, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var f struct {
		Recursos []Recurso `yaml:"recursos"`
	}
	if err := yaml.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("%s: %w", p, err)
	}
	for i := range f.Recursos {
		if f.Recursos[i].Params == nil {
			f.Recursos[i].Params = []ParamMeta{}
		}
		if f.Recursos[i].Visibility == "" {
			f.Recursos[i].Visibility = "private"
		}
	}
	sort.Slice(f.Recursos, func(i, j int) bool { return f.Recursos[i].Name < f.Recursos[j].Name })
	return f.Recursos, nil
}

// RecursosNexus devolve a lista da fonte configurada (vazia se não há fonte).
func (c *Catalogo) RecursosNexus() []Recurso {
	if c.Recursos == nil {
		return nil
	}
	return c.Recursos()
}

// TemNexus diz se há uma fonte de recursos para conferir declarações.
func (c *Catalogo) TemNexus() bool { return c.Recursos != nil }

func (c *Catalogo) recursoPorNome() map[string]Recurso {
	m := map[string]Recurso{}
	for _, r := range c.RecursosNexus() {
		m[r.Name] = r
	}
	return m
}

// RecursosDoAgente resolve as declarações do agente (agent.yaml → recursos e os
// passos do caso.yaml) contra o Nexus. Escrita só vale dentro do caso.
type RecursoDeclarado struct {
	Nome      string   `json:"nome"`
	Origem    string   `json:"origem"` // agent.yaml | caso.yaml
	OK        bool     `json:"ok"`
	Problema  string   `json:"problema,omitempty"`
	Recurso   *Recurso `json:"recurso,omitempty"`
	Conferido bool     `json:"conferido"` // false quando não há Nexus para conferir
}

func (c *Catalogo) RecursosDoAgente(a *Agent) []RecursoDeclarado {
	tem := c.TemNexus()
	reg := c.recursoPorNome()
	var out []RecursoDeclarado
	vistos := map[string]bool{}
	add := func(nome, origem string) {
		if vistos[origem+":"+nome] {
			return
		}
		vistos[origem+":"+nome] = true
		d := RecursoDeclarado{Nome: nome, Origem: origem, Conferido: tem}
		if !tem {
			d.OK = true
			out = append(out, d)
			return
		}
		r, ok := reg[nome]
		switch {
		case !ok:
			d.Problema = "não existe na account do Nexus"
		case r.Write && origem == AgentYAML:
			d.Problema = "recurso de ESCRITA: só pode ser chamado por um passo do caso.yaml"
			d.Recurso = &r
		default:
			d.OK = true
			d.Recurso = &r
		}
		out = append(out, d)
	}
	for _, n := range a.Recursos {
		add(n, AgentYAML)
	}
	if a.Caso != nil {
		for _, n := range a.Caso.Recursos {
			add(n, CasoYAML)
		}
	}
	if out == nil {
		out = []RecursoDeclarado{}
	}
	return out
}
