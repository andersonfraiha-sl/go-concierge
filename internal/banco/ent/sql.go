package ent

import (
	"context"
	"database/sql"
	"fmt"
)

func (c *Client) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	q, ok := c.driver.(interface {
		QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	})
	if !ok {
		return nil, fmt.Errorf("cliente sem consulta SQL")
	}
	return q.QueryContext(ctx, query, args...)
}

func (c *Client) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	q, ok := c.driver.(interface {
		ExecContext(context.Context, string, ...any) (sql.Result, error)
	})
	if !ok {
		return nil, fmt.Errorf("cliente sem execução SQL")
	}
	return q.ExecContext(ctx, query, args...)
}
