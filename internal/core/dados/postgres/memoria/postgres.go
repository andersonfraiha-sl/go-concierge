package memoria

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"go-concierge/internal/core/banco"
	"go-concierge/internal/core/banco/sqlc"
	"go-concierge/internal/escopo"
	"slices"
	"strconv"
	"time"
)

const timeoutConsulta = 3 * time.Second

type Postgres struct{ b banco.Banco }

func NovoPostgres(b banco.Banco) *Postgres { return &Postgres{b: b} }

func (p *Postgres) Ler(ctx context.Context, c escopo.Contato) (Memoria, error) {
	if err := validarContato(c); err != nil {
		return Memoria{}, err
	}
	c = c.Canonico()
	ctx, cancel := context.WithTimeout(ctx, timeoutConsulta)
	defer cancel()
	var m Memoria
	lote := banco.NovoLote()
	p.LerNoLote(ctx, lote, c, func(lida Memoria) { m = lida })
	if err := lote.Enviar(ctx, p.b.Pool); err != nil {
		return Memoria{}, fmt.Errorf("ler memória de %s: %w", c.Mascarado(), err)
	}
	return m, nil
}

func (p *Postgres) LerNoLote(ctx context.Context, lote *banco.Lote, c escopo.Contato, guardar func(Memoria)) {
	if err := validarContato(c); err != nil {
		lote.Erro(err)
		return
	}
	c = c.Canonico()
	lote.Por(func(q *sqlc.Queries) func() error {
		res := q.LoadMemory(ctx, []sqlc.LoadMemoryParams{{CompanyID: c.CompanyID, PhoneNumber: c.Telefone, AllCondos: c.CondoID == nil, CondoID: inteiro(c.CondoID)}})
		return func() error {
			var errLer error
			res.Query(func(_ int, linhas []sqlc.LoadMemoryRow, err error) {
				if err != nil {
					errLer = err
					return
				}
				guardar(memoriaDasLinhas(c, linhas))
			})
			return errLer
		}
	})
}

func memoriaDasLinhas(c escopo.Contato, linhas []sqlc.LoadMemoryRow) Memoria {
	m := vazia(c)
	for _, l := range linhas {
		m.Nome, m.AtualizadaEm = l.Name, l.UpdatedAt.Time
		if !l.Key.Valid {
			continue
		}
		m.Fatos = append(m.Fatos, Fato{Chave: l.Key.String, Texto: l.Text.String, Fonte: l.Source.String, CondoID: ponteiro(l.CondoID), Quando: l.OccurredAt.Time, Confianca: confiancaDe(l.Confidence.Float32)})
	}
	return m
}

func (p *Postgres) Gravar(ctx context.Context, c escopo.Contato, m Memoria) error {
	params, err := parametrosMemoria(c, m)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, timeoutConsulta)
	defer cancel()
	tx, err := p.b.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	if err := sqlc.New(tx).SaveMemory(ctx, params); err != nil {
		return fmt.Errorf("gravar memória de %s: %w", c.Mascarado(), err)
	}
	return banco.Confirmar(ctx, tx)
}

func (p *Postgres) GravarNaTransacao(ctx context.Context, tx pgx.Tx, c escopo.Contato, m Memoria) error {
	params, err := parametrosMemoria(c, m)
	if err != nil {
		return err
	}
	return sqlc.New(tx).SaveMemory(ctx, params)
}

func parametrosMemoria(c escopo.Contato, m Memoria) (sqlc.SaveMemoryParams, error) {
	if err := validarContato(c); err != nil {
		return sqlc.SaveMemoryParams{}, err
	}
	c = c.Canonico()
	m, err := preparar(c, m)
	if err != nil {
		return sqlc.SaveMemoryParams{}, err
	}
	return paramsDeGravacao(c, comoGuardada(c, m, time.Now()))
}

func (p *Postgres) Listar(ctx context.Context, companyID string) ([]Memoria, error) {
	if err := validarAdministradora(companyID); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, timeoutConsulta)
	defer cancel()
	rows, err := p.b.Pool.Query(ctx, `SELECT phone_number FROM contact_memories WHERE company_id = $1 ORDER BY updated_at DESC, phone_number`, companyID)
	if err != nil {
		return nil, err
	}
	var contatos []escopo.Contato
	for rows.Next() {
		var phone string
		if err := rows.Scan(&phone); err != nil {
			rows.Close()
			return nil, err
		}
		contatos = append(contatos, escopo.Contato{CompanyID: companyID, Telefone: phone})
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	out := make([]Memoria, 0, len(contatos))
	for _, c := range contatos {
		m, err := p.Ler(ctx, c)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	slices.SortFunc(out, maisRecentesPrimeiro)
	return out, nil
}

func confiancaDe(real float32) float64 {
	v, _ := strconv.ParseFloat(strconv.FormatFloat(float64(real), 'g', -1, 32), 64)
	return v
}

func inteiro(n *int) pgtype.Int4 {
	if n == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*n), Valid: true}
}

func ponteiro(n pgtype.Int4) *int {
	if !n.Valid {
		return nil
	}
	v := int(n.Int32)
	return &v
}

func paramsDeGravacao(c escopo.Contato, m Memoria) (sqlc.SaveMemoryParams, error) {
	type fatoSQL struct {
		CondoID    *int      `json:"condo_id"`
		Key        string    `json:"key"`
		Text       string    `json:"text"`
		Source     string    `json:"source"`
		OccurredAt time.Time `json:"occurred_at"`
		Confidence float32   `json:"confidence"`
	}
	fatos := make([]fatoSQL, 0, len(m.Fatos))
	for _, f := range m.Fatos {
		fatos = append(fatos, fatoSQL{f.CondoID, f.Chave, f.Texto, f.Fonte, f.Quando.Truncate(time.Microsecond), float32(f.Confianca)})
	}
	dados, err := json.Marshal(fatos)
	return sqlc.SaveMemoryParams{CompanyID: c.CompanyID, PhoneNumber: c.Telefone, TargetCondoID: inteiro(c.CondoID), Name: m.Nome, UpdatedAt: pgtype.Timestamptz{Time: m.AtualizadaEm, Valid: true}, Facts: dados}, err
}
