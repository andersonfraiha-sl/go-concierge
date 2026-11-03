package okf

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Caso é o caso de uso determinístico que um agente pode definir em caso.yaml, no
// mesmo formato do catálogo do sl-concierge: params, passos (resource, choose,
// collect, form, confirm, compose) e output. É a única porta para ESCRITA no Nexus:
// um recurso de escrita só é chamado por um passo `resource` de um caso, depois de
// coletar e (recomendado) confirmar.
type Caso struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Summary     string           `json:"summary"`
	Icon        string           `json:"icon"`
	Visibility  string           `json:"visibility"`
	Fallback    bool             `json:"fallback,omitempty"`
	Description string           `json:"description"`
	Keywords    []string         `json:"keywords"`
	Examples    []string         `json:"examples"`
	Params      []CasoParam      `json:"params"`
	Steps       []map[string]any `json:"steps"`
	Output      string           `json:"output"`
	// Recursos citados pelos passos `resource`, na ordem.
	Recursos []string `json:"recursos"`
	// Confirma: há um passo `confirm` antes do último recurso.
	Confirma bool `json:"confirma"`
}

// CasoParam é um parâmetro coletado do morador.
type CasoParam struct {
	Name     string `json:"name"`
	Title    string `json:"title,omitempty"`
	Ask      string `json:"ask,omitempty"`
	Type     string `json:"type,omitempty"`
	Validate string `json:"validate,omitempty"`
	Required *bool  `json:"required,omitempty"`
	Options  any    `json:"options,omitempty"`
	Reuse    string `json:"reuse,omitempty"`
}

// LoadCaso lê e valida a forma mínima de um caso.yaml.
func LoadCaso(p string) (*Caso, error) {
	raw, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	return ParseCaso(raw)
}

// ParseCaso interpreta o YAML de um caso.
func ParseCaso(raw []byte) (*Caso, error) {
	var c Caso
	if err := yaml.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("caso.yaml inválido: %w", err)
	}
	if strings.TrimSpace(c.ID) == "" {
		return nil, fmt.Errorf("caso.yaml sem `id`")
	}
	if len(c.Steps) == 0 {
		return nil, fmt.Errorf("caso.yaml sem `steps`")
	}
	if c.Keywords == nil {
		c.Keywords = []string{}
	}
	if c.Examples == nil {
		c.Examples = []string{}
	}
	if c.Params == nil {
		c.Params = []CasoParam{}
	}
	if c.Visibility == "" {
		c.Visibility = "private"
	}
	c.Recursos = []string{}
	for _, s := range c.Steps {
		if r, ok := s["resource"].(string); ok && r != "" {
			c.Recursos = append(c.Recursos, r)
		}
		if _, ok := s["confirm"]; ok {
			c.Confirma = true
		}
	}
	return &c, nil
}

// Passos descreve o fluxo em linhas curtas (para a projeção e a interface).
func (c *Caso) Passos() []string {
	var out []string
	for _, s := range c.Steps {
		switch {
		case s["resource"] != nil:
			l := fmt.Sprintf("recurso `%v`", s["resource"])
			if sa, ok := s["save_as"].(string); ok && sa != "" {
				l += " → guarda em " + sa
			}
			out = append(out, l)
		case s["choose"] != nil:
			if m, ok := s["choose"].(map[string]any); ok {
				out = append(out, fmt.Sprintf("escolher `%v` entre %v (uma só opção: use sem perguntar; se o morador já disse ou a identidade/memória responde, não pergunte)", m["param"], m["from"]))
			} else {
				out = append(out, "escolher")
			}
		case s["collect"] != nil:
			out = append(out, "coletar "+strings.Join(stringList(s["collect"]), ", "))
		case s["form"] != nil:
			out = append(out, "formulário dinâmico")
		case s["confirm"] != nil:
			out = append(out, "confirmar (sim/não)")
		case s["compose"] != nil:
			out = append(out, "redigir com o modelo")
		default:
			keys := make([]string, 0, len(s))
			for k := range s {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			out = append(out, "passo "+strings.Join(keys, ","))
		}
	}
	return out
}
