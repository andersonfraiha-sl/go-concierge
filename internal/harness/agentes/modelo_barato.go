package agentes

import (
	"context"
	"github.com/cloudwego/eino/schema"
	"time"
)

// modeloBarato monta o chat model barato, ou ok=false quando não há provedor.
func ModeloBarato(ctx context.Context, cfg Config) (func(ctx context.Context, msgs []*schema.Message) (*schema.Message, error), bool) {
	if cfg.Mock || cfg.Modelos == nil {
		return nil, false
	}
	m, _, _, disponivel, err := cfg.Modelos.Criar(ctx, "barato", "", 60*time.Second)
	if err != nil || !disponivel {
		return nil, false
	}
	return func(ctx context.Context, msgs []*schema.Message) (*schema.Message, error) {
		return m.Generate(ctx, msgs)
	}, true
}
