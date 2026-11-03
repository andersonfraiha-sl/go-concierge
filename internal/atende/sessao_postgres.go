package atende

import (
	"context"
	"errors"
	"fmt"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"go-concierge/internal/banco"
	"go-concierge/internal/banco/ent"
	entsessao "go-concierge/internal/banco/ent/sessao"
	"go-concierge/internal/banco/sqlc"
)

// timeoutSessao limita cada leitura ou escrita de sessão no Postgres.
const timeoutSessao = 3 * time.Second

// SessaoPostgres guarda as sessões do ATENDE na tabela sessoes. Carregar e Salvar são
// queries do sqlc (cabem nos lotes do turno); Listar e Remover vão pelo ent.
type SessaoPostgres struct {
	b banco.Banco
}

func NovoSessaoPostgres(b banco.Banco) *SessaoPostgres { return &SessaoPostgres{b: b} }

func (p *SessaoPostgres) Salvar(ctx context.Context, r RegistroSessao) error {
	ctx, cancel := context.WithTimeout(ctx, timeoutSessao)
	defer cancel()
	lote := banco.NovoLote()
	p.SalvarNoLote(ctx, lote, r)
	if err := lote.Enviar(ctx, p.b.Pool); err != nil {
		return fmt.Errorf("salvar sessão %s: %w", r.ID, err)
	}
	return nil
}

// SalvarNoLote põe no lote a gravação da sessão (SalvarSessao); o erro volta no Enviar.
func (p *SessaoPostgres) SalvarNoLote(ctx context.Context, lote *banco.Lote, r RegistroSessao) {
	res := lote.Queries().SalvarSessao(ctx, []sqlc.SalvarSessaoParams{{
		ID: r.ID, AdministradoraID: r.AdministradoraID, CondominioID: r.CondominioID, Protocolo: r.Protocolo,
		AtualizadaEm: pgtype.Timestamptz{Time: r.AtualizadaEm, Valid: true}, Dados: r.Dados,
	}})
	lote.Depois(func() error {
		var errSalvar error
		res.Exec(func(_ int, err error) { errSalvar = err })
		return errSalvar
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
		return RegistroSessao{}, false, fmt.Errorf("carregar sessão %s: %w", id, err)
	}
	return r, achou, nil
}

// CarregarNoLote põe no lote a leitura da sessão (CarregarSessao); guardar recebe o registro
// (ok=false se a sessão não existe) quando o lote roda.
func (p *SessaoPostgres) CarregarNoLote(ctx context.Context, lote *banco.Lote, id string, guardar func(RegistroSessao, bool)) {
	res := lote.Queries().CarregarSessao(ctx, []string{id})
	lote.Depois(func() error {
		var errCarregar error
		res.QueryRow(func(_ int, l sqlc.SessaoGravada, err error) {
			switch {
			case errors.Is(err, pgx.ErrNoRows):
				guardar(RegistroSessao{}, false)
			case err != nil:
				errCarregar = err
			default:
				guardar(RegistroSessao{
					ID: l.ID, AdministradoraID: l.AdministradoraID, CondominioID: l.CondominioID, Protocolo: l.Protocolo,
					AtualizadaEm: l.AtualizadaEm.Time, Dados: l.Dados,
				}, true)
			}
		})
		return errCarregar
	})
}

func (p *SessaoPostgres) Listar(ctx context.Context, administradoraID string, limite int) ([]RegistroSessao, error) {
	ctx, cancel := context.WithTimeout(ctx, timeoutSessao)
	defer cancel()
	q := p.b.Ent.Sessao.Query()
	if administradoraID != "" {
		q = q.Where(entsessao.AdministradoraID(administradoraID))
	}
	linhas, err := q.Order(entsessao.ByAtualizadaEm(sql.OrderDesc())).Limit(limite).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("listar sessões da administradora %q: %w", administradoraID, err)
	}
	regs := make([]RegistroSessao, len(linhas))
	for i, l := range linhas {
		regs[i] = registroDaLinha(l)
	}
	return regs, nil
}

func (p *SessaoPostgres) Remover(ctx context.Context, id string) error {
	ctx, cancel := context.WithTimeout(ctx, timeoutSessao)
	defer cancel()
	if _, err := p.b.Ent.Sessao.Delete().Where(entsessao.ID(id)).Exec(ctx); err != nil {
		return fmt.Errorf("remover sessão %s: %w", id, err)
	}
	return nil
}

func registroDaLinha(l *ent.Sessao) RegistroSessao {
	return RegistroSessao{
		ID: l.ID, AdministradoraID: l.AdministradoraID, CondominioID: l.CondominioID, Protocolo: l.Protocolo,
		AtualizadaEm: l.AtualizadaEm, Dados: l.Dados,
	}
}
