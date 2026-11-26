package camadas

import (
	"time"

	okf "go-concierge/internal/harness/conteudo"
)

type Tipo string

const (
	TipoBase           Tipo = "BASE"
	TipoAdministradora Tipo = "COMPANY"
	TipoCondominio     Tipo = "CONDO"
)

type Camada struct {
	Skill     string `json:"skill"`
	Tipo      Tipo   `json:"tipo"`
	ID        string `json:"id"`
	CompanyID string `json:"company_id,omitempty"`
}

type Documento struct {
	Textos      okf.Textos `json:"textos"`
	Fluxo       okf.Fluxo  `json:"fluxo"`
	Ferramentas []string   `json:"ferramentas"`
	Orientacao  string     `json:"orientacao"`
}

type Cabeca struct {
	VersaoID  int
	Documento Documento
}

type VersaoBase struct {
	Cabeca
	CasoYAML  []byte
	EncerraEm time.Time
}

type Retrato struct {
	Bases           map[string]VersaoBase
	Administradoras map[string]map[string]Cabeca
	Condominios     map[string]map[string]Cabeca
}
