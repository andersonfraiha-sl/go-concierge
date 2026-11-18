package atende

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"go-concierge/internal/banco"
	"go-concierge/internal/banco/sqlc"
)

const timeoutSessao = 3 * time.Second

type SessaoPostgres struct{ b banco.Banco }

func NovoSessaoPostgres(b banco.Banco) *SessaoPostgres { return &SessaoPostgres{b: b} }

func (p *SessaoPostgres) Salvar(ctx context.Context, r RegistroSessao) error {
	ctx, cancel := context.WithTimeout(ctx, timeoutSessao)
	defer cancel()
	lote := banco.NovoLote()
	p.SalvarNoLote(ctx, lote, r)
	return lote.Enviar(ctx, p.b.Pool)
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
	e, ok := ctx.Value(chaveEscopoSessao{}).(escopoSessao)
	if !ok || e.companyID == "" {
		lote.Erro(ErrEscopoSessao)
		return
	}
	lote.Por(func(q *sqlc.Queries) func() error {
		res := q.LoadSession(ctx, []sqlc.LoadSessionParams{{ID: id, CompanyID: e.companyID, AllChannels: e.channelID == uuid.Nil, ChannelID: pgtype.UUID{Bytes: e.channelID, Valid: true}}})
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
	e, ok := ctx.Value(chaveEscopoSessao{}).(escopoSessao)
	if !ok || companyID == "" || e.companyID != companyID {
		return nil, ErrEscopoSessao
	}
	if limite < 0 {
		return nil, fmt.Errorf("limite inválido")
	}
	ctx, cancel := context.WithTimeout(ctx, timeoutSessao)
	defer cancel()
	rows, err := p.b.Pool.Query(ctx, `SELECT s.id, s.company_id, s.channel_id, s.connector, s.protocol, s.phone_number, s.condo_id, s.data, s.updated_at FROM sessions s JOIN channels c ON c.company_id=s.company_id AND c.id=s.channel_id WHERE s.company_id=$1 AND ($2::uuid IS NULL OR s.channel_id=$2) AND (c.condo_id IS NULL OR c.condo_id IS NOT DISTINCT FROM s.condo_id) AND (c.condo_id IS NULL OR c.condo_id IS NOT DISTINCT FROM (s.data #>> '{scope,condo_id}')::integer) ORDER BY s.updated_at DESC LIMIT $3`, companyID, pgtype.UUID{Bytes: e.channelID, Valid: e.channelID != uuid.Nil}, limite)
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
	e, ok := ctx.Value(chaveEscopoSessao{}).(escopoSessao)
	if !ok || e.companyID == "" {
		return ErrEscopoSessao
	}
	ctx, cancel := context.WithTimeout(ctx, timeoutSessao)
	defer cancel()
	_, err := p.b.Pool.Exec(ctx, `DELETE FROM sessions WHERE id=$1 AND company_id=$2 AND ($3::uuid IS NULL OR channel_id=$3)`, id, e.companyID, pgtype.UUID{Bytes: e.channelID, Valid: e.channelID != uuid.Nil})
	return err
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
