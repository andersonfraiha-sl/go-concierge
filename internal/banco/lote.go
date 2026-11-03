package banco

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"go-concierge/internal/banco/sqlc"
)

// Lote junta queries :batch* do sqlc de vários stores num pgx.Batch só, enviado numa ida ao
// banco. Cada query do sqlc chama SendBatch com o seu lote de um; aqui esse lote entra no
// Lote, e os resultados que o sqlc lê chegam do envio único, na ordem em que entraram.
//
// Uso: o store chama uma query por Queries() e registra em Depois a leitura do resultado
// (o Query/QueryRow/Exec do sqlc); Enviar manda tudo e roda as leituras, na mesma ordem.
// Toda query enfileirada precisa da sua leitura em Depois, senão as seguintes leem o
// resultado errado.
type Lote struct {
	batch      pgx.Batch
	resultados *resultadosDoEnvio
	leituras   []func() error
}

func NovoLote() *Lote { return &Lote{resultados: &resultadosDoEnvio{}} }

// Queries devolve as queries do sqlc que entram neste lote em vez de ir ao banco.
func (l *Lote) Queries() *sqlc.Queries { return sqlc.New(enfileirador{l}) }

// Depois registra a leitura do resultado da última query enfileirada.
func (l *Lote) Depois(ler func() error) { l.leituras = append(l.leituras, ler) }

func (l *Lote) Vazio() bool { return l.batch.Len() == 0 }

// Enviar manda o lote numa ida e roda as leituras na ordem. O lote roda numa transação
// implícita: um erro em qualquer comando desfaz todos.
func (l *Lote) Enviar(ctx context.Context, pool *pgxpool.Pool) error {
	envio := pool.SendBatch(ctx, &l.batch)
	l.resultados.envio = envio
	errs := make([]error, 0, len(l.leituras)+1)
	for _, ler := range l.leituras {
		errs = append(errs, ler())
	}
	return errors.Join(append(errs, envio.Close())...)
}

var errSoLote = errors.New("dentro de um Lote só cabem queries :batch* do sqlc")

type enfileirador struct{ lote *Lote }

func (e enfileirador) SendBatch(_ context.Context, b *pgx.Batch) pgx.BatchResults {
	e.lote.batch.QueuedQueries = append(e.lote.batch.QueuedQueries, b.QueuedQueries...)
	return e.lote.resultados
}

func (enfileirador) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errSoLote
}

func (enfileirador) Query(context.Context, string, ...any) (pgx.Rows, error) { return nil, errSoLote }

func (enfileirador) QueryRow(context.Context, string, ...any) pgx.Row { return linhaComErro{} }

type linhaComErro struct{}

func (linhaComErro) Scan(...any) error { return errSoLote }

// resultadosDoEnvio entrega aos leitores do sqlc os resultados do envio único. O Close de
// cada leitor não fecha nada: quem fecha é o Enviar, depois de todas as leituras.
type resultadosDoEnvio struct{ envio pgx.BatchResults }

func (r *resultadosDoEnvio) Exec() (pgconn.CommandTag, error) { return r.envio.Exec() }
func (r *resultadosDoEnvio) Query() (pgx.Rows, error)         { return r.envio.Query() }
func (r *resultadosDoEnvio) QueryRow() pgx.Row                { return r.envio.QueryRow() }
func (*resultadosDoEnvio) Close() error                       { return nil }
func (r *resultadosDoEnvio) Tx() pgx.Tx                       { return r.envio.Tx() }
