package arquivos

import (
	"os"
)

// LoadCaso lê e valida a forma mínima de um caso.yaml.
func LoadCaso(p string) (*Caso, error) {
	raw, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	return ParseCaso(raw)
}
