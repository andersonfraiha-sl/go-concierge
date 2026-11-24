package conteudo

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

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

// Recurso é um item do catálogo de recursos de uma account no Nexus.
// OrigemMCP marca um recurso que veio do catálogo MCP (tools/list), não do Nexus nem do
// instantâneo: o harness o executa por tools/call e o Backdoor não o edita.
const OrigemMCP = "mcp"

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
	// Amostra é o retorno que o mock devolve quando não há Nexus no ar (o emulador
	// funciona inteiro sem integração). Escrita sem amostra devolve um protocolo gerado.
	Amostra any `json:"amostra,omitempty" yaml:"amostra,omitempty"`
	// Definicoes são os ajustes que um condomínio pode dar a este recurso (antecedência
	// mínima, horário, código da empresa…): o Admin os mostra como formulário, em
	// linguagem de administradora, e grava no payload do condomínio. Sem definições,
	// o recurso não aparece nos Ajustes.
	Definicoes []Definicao `json:"definicoes,omitempty" yaml:"definicoes"`
	// Origem: de onde o recurso veio em tempo de execução ("mcp"); vazio = Nexus ou instantâneo.
	Origem string `json:"origem,omitempty" yaml:"-"`
	// Schema é o inputSchema completo da tool MCP (JSON Schema), para a interface e o manual.
	Schema any `json:"schema,omitempty" yaml:"-"`
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
