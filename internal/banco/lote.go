package banco

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"go-concierge/internal/banco/sqlc"
)

// NoLote devolve as queries do sqlc enfileirando no lote em vez de executar: um comando
// :exec do sqlc entra num pgx.Batch com o SQL e os argumentos que o sqlc gerou. O erro de
// cada comando volta no Close do lote. Queries que devolvem linhas não cabem aqui.
func NoLote(lote *pgx.Batch) *sqlc.Queries { return sqlc.New(enfileirador{lote}) }

var errSoExec = errors.New("só comandos :exec entram no lote")

type enfileirador struct{ lote *pgx.Batch }

func (e enfileirador) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	e.lote.Queue(sql, args...)
	return pgconn.CommandTag{}, nil
}

func (enfileirador) Query(context.Context, string, ...any) (pgx.Rows, error) { return nil, errSoExec }

func (enfileirador) QueryRow(context.Context, string, ...any) pgx.Row { return linhaComErro{} }

type linhaComErro struct{}

func (linhaComErro) Scan(...any) error { return errSoExec }
