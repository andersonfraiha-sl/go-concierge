package okf

import (
	"fmt"
	"os"
	"sort"

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
	Kind        string      `json:"kind" yaml:"kind"`
	Visibility  string      `json:"visibility" yaml:"visibility"` // public | private
	Write       bool        `json:"write" yaml:"write"`
	Description string      `json:"description" yaml:"description"`
	Params      []ParamMeta `json:"params" yaml:"params"`
	Output      string      `json:"output,omitempty" yaml:"output"`
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
