package recursos

import (
	"context"
	"encoding/json"
)

type Definicao struct {
	Nome      string
	Descricao string
	Schema    json.RawMessage
}

type Cliente interface {
	Descrever(context.Context, string) (Definicao, error)
	Executar(context.Context, string, map[string]any) (any, error)
}
