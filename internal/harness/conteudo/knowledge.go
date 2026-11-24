package conteudo

// Doc é um documento OKF em Markdown com frontmatter.
type Doc struct {
	Caminho   string   `json:"caminho"`
	Tipo      string   `json:"tipo"`
	ID        string   `json:"id,omitempty"`
	Titulo    string   `json:"titulo"`
	Descricao string   `json:"descricao,omitempty"`
	Status    string   `json:"status,omitempty"`
	Versao    string   `json:"versao,omitempty"`
	Dominio   string   `json:"dominio,omitempty"`
	Resource  string   `json:"resource,omitempty"`  // OKF v0.1: o recurso que o conceito descreve (tabela, API…)
	Timestamp string   `json:"timestamp,omitempty"` // OKF v0.1: quando o conceito foi produzido/atualizado
	Tags      []string `json:"tags"`
	Respostas []string `json:"respostas,omitempty"`
	HerdadoDe string   `json:"herdado_de,omitempty"`
}

// Termo é uma entrada de glossário.
type Termo struct {
	ID        string   `json:"id"`
	Nome      string   `json:"display_name"`
	Sinonimos []string `json:"synonyms"`
	Definicao string   `json:"definition"`
	Labels    []string `json:"labels"`
	Arquivo   string   `json:"arquivo"`
	HerdadoDe string   `json:"herdado_de,omitempty"`
}

type Conhecimento struct {
	Docs   []Doc   `json:"docs"`
	Termos []Termo `json:"termos"`
}
