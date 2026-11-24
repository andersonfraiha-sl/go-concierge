package pedidos

import (
	"context"
	"go-concierge/internal/core/banco"
	"go-concierge/internal/escopo"
)

type Cache struct{ pg *Postgres }

func NovoCache(pg *Postgres) *Cache { return &Cache{pg: pg} }

func (c *Cache) Ler(ctx context.Context, contato escopo.Contato) (Carteira, error) {
	return c.pg.Ler(ctx, contato)
}

func (c *Cache) Gravar(ctx context.Context, contato escopo.Contato, cart Carteira) error {
	return c.pg.Gravar(ctx, contato, cart)
}

func (c *Cache) Listar(ctx context.Context, companyID string) ([]Carteira, error) {
	return c.pg.Listar(ctx, companyID)
}

func (c *Cache) AquecerNoLote(context.Context, *banco.Lote, escopo.Contato) {}

func (c *Cache) GravarNoLote(ctx context.Context, lote *banco.Lote, contato escopo.Contato, cart Carteira) (func(), error) {
	return nil, c.pg.GravarNoLote(ctx, lote, contato, cart)
}
