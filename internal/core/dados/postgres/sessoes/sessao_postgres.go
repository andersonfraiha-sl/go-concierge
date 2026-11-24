package sessoes

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"go-concierge/internal/core/banco"
	"go-concierge/internal/core/banco/sqlc"
)

const timeoutSessao = 3 * time.Second

type SessaoPostgres struct{ b banco.Banco }

func NovoSessaoPostgres(b banco.Banco) *SessaoPostgres { return &SessaoPostgres{b: b} }

func (p *SessaoPostgres) Salvar(ctx context.Context, r RegistroSessao) error {
	ctx, cancel := context.WithTimeout(ctx, timeoutSessao)
	defer cancel()
	lote := banco.NovoLote()
	p.SalvarNoLote(ctx, lote, r)
	if err := lote.Enviar(ctx, p.b.Pool); err != nil {
		if errors.Is(err, banco.ErrCommitIncerto) {
			return err
		}
		return errors.Join(ErrGravacaoNaoAplicada, err)
	}
	return nil
}

func (p *SessaoPostgres) SalvarNoLote(ctx context.Context, lote *banco.Lote, r RegistroSessao) {
	if err := ValidarRegistroSessao(r); err != nil {
		lote.Erro(err)
		return
	}
	if err := ValidarEscopoSessao(ctx, r); err != nil {
		lote.Erro(err)
		return
	}
	snapshot, err := serializarSessaoSnapshot(r)
	if err != nil {
		lote.Erro(err)
		return
	}
	var condo pgtype.Int4
	if r.CondoID != nil {
		condo = pgtype.Int4{Int32: int32(*r.CondoID), Valid: true}
	}
	lote.Por(func(q *sqlc.Queries) func() error {
		res := q.SaveSession(ctx, []sqlc.SaveSessionParams{{ID: r.ID, CompanyID: r.CompanyID, ChannelID: pgtype.UUID{Bytes: r.ChannelID, Valid: true}, Connector: r.Connector, Protocol: r.Protocolo, PhoneNumber: r.UserPhone, CondoID: condo, Data: snapshot, UpdatedAt: pgtype.Timestamptz{Time: r.AtualizadaEm, Valid: true}}})
		return func() error {
			var errSalvar error
			res.QueryRow(func(_ int, _ string, err error) {
				if errors.Is(err, pgx.ErrNoRows) {
					errSalvar = ErrEscopoSessao
				} else {
					errSalvar = err
				}
			})
			return errSalvar
		}
	})
}

func (p *SessaoPostgres) Carregar(ctx context.Context, id string) (RegistroSessao, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, timeoutSessao)
	defer cancel()
	var r RegistroSessao
	var achou bool
	lote := banco.NovoLote()
	p.CarregarNoLote(ctx, lote, id, func(lido RegistroSessao, ok bool) { r, achou = lido, ok })
	if err := lote.Enviar(ctx, p.b.Pool); err != nil {
		return RegistroSessao{}, false, err
	}
	return r, achou, nil
}

func (p *SessaoPostgres) CarregarNoLote(ctx context.Context, lote *banco.Lote, id string, guardar func(RegistroSessao, bool)) {
	empresa, canal, ok := EscopoSessaoDoContexto(ctx)
	if !ok || empresa == "" {
		lote.Erro(ErrEscopoSessao)
		return
	}
	lote.Por(func(q *sqlc.Queries) func() error {
		res := q.LoadSession(ctx, []sqlc.LoadSessionParams{{ID: id, CompanyID: empresa, AllChannels: canal == uuid.Nil, ChannelID: pgtype.UUID{Bytes: canal, Valid: true}}})
		return func() error {
			var errLer error
			res.QueryRow(func(_ int, l sqlc.Session, err error) {
				if errors.Is(err, pgx.ErrNoRows) {
					guardar(RegistroSessao{}, false)
					return
				}
				if err != nil {
					errLer = err
					return
				}
				r, err := restaurarSessaoSnapshot(registroDaLinha(l))
				errLer = err
				if errLer == nil {
					guardar(r, true)
				}
			})
			return errLer
		}
	})
}

func (p *SessaoPostgres) Listar(ctx context.Context, companyID string, limite int) ([]RegistroSessao, error) {
	empresa, canal, ok := EscopoSessaoDoContexto(ctx)
	if !ok || companyID == "" || empresa != companyID {
		return nil, ErrEscopoSessao
	}
	if limite < 0 {
		return nil, fmt.Errorf("limite inválido")
	}
	ctx, cancel := context.WithTimeout(ctx, timeoutSessao)
	defer cancel()
	rows, err := p.b.Pool.Query(ctx, `SELECT s.id, s.company_id, s.channel_id, s.connector, s.protocol, s.phone_number, s.condo_id, s.data, s.updated_at FROM sessions s JOIN channels c ON c.company_id=s.company_id AND c.id=s.channel_id WHERE s.company_id=$1 AND ($2::uuid IS NULL OR s.channel_id=$2) AND (c.condo_id IS NULL OR c.condo_id IS NOT DISTINCT FROM s.condo_id) AND (c.condo_id IS NULL OR c.condo_id IS NOT DISTINCT FROM (s.data #>> '{scope,condo_id}')::integer) ORDER BY s.updated_at DESC LIMIT $3`, companyID, pgtype.UUID{Bytes: canal, Valid: canal != uuid.Nil}, limite)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	regs := []RegistroSessao{}
	for rows.Next() {
		var l sqlc.Session
		if err := rows.Scan(&l.ID, &l.CompanyID, &l.ChannelID, &l.Connector, &l.Protocol, &l.PhoneNumber, &l.CondoID, &l.Data, &l.UpdatedAt); err != nil {
			return nil, err
		}
		r, err := restaurarSessaoSnapshot(registroDaLinha(l))
		if err != nil {
			return nil, err
		}
		regs = append(regs, r)
	}
	return regs, rows.Err()
}

func (p *SessaoPostgres) Remover(ctx context.Context, id string) error {
	empresa, canal, ok := EscopoSessaoDoContexto(ctx)
	if !ok || empresa == "" {
		return ErrEscopoSessao
	}
	ctx, cancel := context.WithTimeout(ctx, timeoutSessao)
	defer cancel()
	_, err := p.b.Pool.Exec(ctx, `DELETE FROM sessions WHERE id=$1 AND company_id=$2 AND ($3::uuid IS NULL OR channel_id=$3)`, id, empresa, pgtype.UUID{Bytes: canal, Valid: canal != uuid.Nil})
	return err
}

func (p *SessaoPostgres) SalvarAutorizado(ctx context.Context, r RegistroSessao, autorizar func(RegistroSessao, bool) error) error {
	if err := ValidarRegistroSessao(r); err != nil {
		return errors.Join(ErrGravacaoNaoAplicada, err)
	}
	if err := ValidarEscopoSessao(ctx, r); err != nil {
		return errors.Join(ErrGravacaoNaoAplicada, err)
	}
	ctx, cancelar := context.WithTimeout(ctx, timeoutSessao)
	defer cancelar()
	tx, err := p.b.Pool.Begin(ctx)
	if err != nil {
		return errors.Join(ErrGravacaoNaoAplicada, err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	atual, achou, err := sessaoBloqueada(ctx, tx, r.ID, r.CompanyID)
	if err != nil {
		return errors.Join(ErrGravacaoNaoAplicada, err)
	}
	if err := autorizar(atual, achou); err != nil {
		return errors.Join(ErrGravacaoNaoAplicada, err)
	}
	lote := banco.NovoLote()
	p.SalvarNoLote(ctx, lote, r)
	if err := lote.Executar(ctx, tx); err != nil {
		return errors.Join(ErrGravacaoNaoAplicada, err)
	}
	if err := banco.Confirmar(ctx, tx); err != nil {
		if errors.Is(err, banco.ErrCommitIncerto) {
			return err
		}
		return errors.Join(ErrGravacaoNaoAplicada, err)
	}
	return nil
}

func (p *SessaoPostgres) RemoverAutorizado(ctx context.Context, id string, autorizar func(RegistroSessao, bool) error) error {
	company, _, ok := EscopoSessaoDoContexto(ctx)
	if !ok {
		return errors.Join(ErrGravacaoNaoAplicada, ErrEscopoSessao)
	}
	ctx, cancelar := context.WithTimeout(ctx, timeoutSessao)
	defer cancelar()
	tx, err := p.b.Pool.Begin(ctx)
	if err != nil {
		return errors.Join(ErrGravacaoNaoAplicada, err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	atual, achou, err := sessaoBloqueada(ctx, tx, id, company)
	if err != nil {
		return errors.Join(ErrGravacaoNaoAplicada, err)
	}
	if err := autorizar(atual, achou); err != nil {
		return errors.Join(ErrGravacaoNaoAplicada, err)
	}
	if achou {
		if _, err := tx.Exec(ctx, `DELETE FROM sessions WHERE id=$1 AND company_id=$2`, id, company); err != nil {
			return errors.Join(ErrGravacaoNaoAplicada, err)
		}
	}
	if err := banco.Confirmar(ctx, tx); err != nil {
		if errors.Is(err, banco.ErrCommitIncerto) {
			return err
		}
		return errors.Join(ErrGravacaoNaoAplicada, err)
	}
	return nil
}

func sessaoBloqueada(ctx context.Context, tx pgx.Tx, id, company string) (RegistroSessao, bool, error) {
	var l sqlc.Session
	err := tx.QueryRow(ctx, `SELECT s.id, s.company_id, s.channel_id, s.connector, s.protocol, s.phone_number, s.condo_id, s.data, s.updated_at FROM sessions s JOIN channels c ON c.company_id=s.company_id AND c.id=s.channel_id WHERE s.id=$1 AND s.company_id=$2 AND (c.condo_id IS NULL OR c.condo_id IS NOT DISTINCT FROM s.condo_id) AND (c.condo_id IS NULL OR c.condo_id IS NOT DISTINCT FROM (s.data #>> '{scope,condo_id}')::integer) FOR UPDATE OF s FOR SHARE OF c`, id, company).Scan(&l.ID, &l.CompanyID, &l.ChannelID, &l.Connector, &l.Protocol, &l.PhoneNumber, &l.CondoID, &l.Data, &l.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return RegistroSessao{}, false, nil
	}
	if err != nil {
		return RegistroSessao{}, false, err
	}
	r, err := restaurarSessaoSnapshot(registroDaLinha(l))
	return r, err == nil, err
}

func registroDaLinha(l sqlc.Session) RegistroSessao {
	r := RegistroSessao{ID: l.ID, CompanyID: l.CompanyID, ChannelID: uuid.UUID(l.ChannelID.Bytes), Connector: l.Connector, Protocolo: l.Protocol, UserPhone: l.PhoneNumber, AtualizadaEm: l.UpdatedAt.Time, Dados: l.Data, AdministradoraID: l.CompanyID}
	if l.CondoID.Valid {
		n := int(l.CondoID.Int32)
		r.CondoID = &n
		r.CondominioID = fmt.Sprint(n)
	}
	return r
}
