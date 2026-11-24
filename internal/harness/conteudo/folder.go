package conteudo

import (
	"fmt"
	"strings"
)

const (
	Orquestrador = "orquestrador"
	Operador     = "operador"

	DirAgentes = "agentes"
	AgentYAML  = "agent.yaml"
	PersonaMD  = "persona.md"
	CasoYAML   = "caso.yaml"
)

// Arquivo é um item de uma caixinha, próprio ou herdado.
type Arquivo struct {
	Caminho   string `json:"caminho"` // relativo à pasta do agente dono
	Bytes     int64  `json:"bytes"`
	Resumo    string `json:"resumo,omitempty"`
	HerdadoDe string `json:"herdado_de,omitempty"` // nome do ancestral dono
}

// Agent é a pasta carregada.
type Agent struct {
	Path string `json:"-"`
	// Rel é o caminho relativo à raiz, com barras: agentes/concierge/agentes/regimento.
	Rel           string               `json:"caminho"`
	Nome          string               `json:"nome"`
	Tipo          string               `json:"tipo"`
	TipoDeclarado bool                 `json:"tipo_declarado"`
	Descricao     string               `json:"descricao"`
	Modelo        string               `json:"modelo,omitempty"`
	Recursos      []string             `json:"recursos"` // nomes de recursos do Nexus que o agente declara usar
	Config        map[string]any       `json:"config"`
	Arquivos      map[string][]Arquivo `json:"arquivos"`             // por caixinha: só os próprios
	Herdados      map[string][]Arquivo `json:"herdados"`             // por caixinha: dos ancestrais, não sobrepostos
	Persona       string               `json:"persona"`              // persona.md; só o agente de primeiro nível tem (subagentes: vazio)
	PersonaDe     string               `json:"persona_de,omitempty"` // reservado (sempre vazio): a voz é aplicada pelo orquestrador na entrega
	Caso          *Caso                `json:"caso,omitempty"`
	Subagentes    []*Agent             `json:"subagentes"`
	Problemas     []Problema           `json:"problemas"`
}

// Problema é algo errado ou suspeito encontrado ao carregar ou validar.
type Problema struct {
	Nivel   string `json:"nivel"` // erro | aviso
	Onde    string `json:"onde"`
	Texto   string `json:"texto"`
	Solucao string `json:"solucao,omitempty"`
}

func StringList(v any) []string {
	out := []string{}
	switch t := v.(type) {
	case []any:
		for _, x := range t {
			out = append(out, fmt.Sprint(x))
		}
	case []string:
		out = append(out, t...)
	case string:
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

// StripFrontmatter devolve o corpo sem o bloco inicial.
func StripFrontmatter(texto string) string {
	_, corpo := SplitFrontmatter(texto)
	return corpo
}

func SplitFrontmatter(texto string) (fm, corpo string) {
	if !strings.HasPrefix(texto, "---") {
		return "", texto
	}
	fim := strings.Index(texto[3:], "\n---")
	if fim == -1 {
		return "", texto
	}
	return texto[3 : 3+fim], strings.TrimLeft(texto[3+fim+4:], "\n")
}
