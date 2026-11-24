package execucao

import (
	"go-concierge/internal/core/catalogo"
	"go-concierge/internal/harness/canais"
	admin "go-concierge/internal/harness/configuracao"
)

const RotaConfiguracao = "/api/runtime/configuracao"

type Configuracao struct {
	Canal    canais.Canal       `json:"canal"`
	Casos    []canais.Caso      `json:"casos"`
	Conteudo admin.Mapeamento   `json:"conteudo"`
	Arquivos []catalogo.Arquivo `json:"arquivos"`
}
