package memoria

import (
	"context"
	"go-concierge/internal/core/banco"
	"go-concierge/internal/escopo"
)

type Cache struct{ pg *Postgres }

func NovoCache(pg *Postgres) *Cache { return &Cache{pg: pg} }

func (c *Cache) Ler(ctx context.Context, contato escopo.Contato) (Memoria, error) {
	return c.pg.Ler(ctx, contato)
}

func (c *Cache) Gravar(ctx context.Context, contato escopo.Contato, m Memoria) error {
	return c.pg.Gravar(ctx, contato, m)
}

func (c *Cache) Listar(ctx context.Context, companyID string) ([]Memoria, error) {
	return c.pg.Listar(ctx, companyID)
}

func (c *Cache) AquecerNoLote(context.Context, *banco.Lote, escopo.Contato) {}
