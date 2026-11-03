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

// SessaoPostgres guarda as sessões do ATENDE na tabela sessoes. Carregar e Salvar vão pelo
// pgx (cabem nos lotes do turno); Listar e Remover, pelo ent.
type SessaoPostgres struct {
	b banco.Banco
}

func NovoSessaoPostgres(b banco.Banco) *SessaoPostgres { return &SessaoPostgres{b: b} }

func (p *SessaoPostgres) Salvar(ctx context.Context, r RegistroSessao) error {
	ctx, cancel := context.WithTimeout(ctx, timeoutSessao)
	defer cancel()
	if err := p.b.SQL.SalvarSessao(ctx, paramsDaSessao(r)); err != nil {
		return fmt.Errorf("salvar sessão %s: %w", r.ID, err)
	}
	return nil
}

// SalvarNoLote põe no lote a gravação da sessão; o erro volta no Close do lote.
func (p *SessaoPostgres) SalvarNoLote(lote *pgx.Batch, r RegistroSessao) {
	// o enfileirador do lote não executa nada nem devolve erro aqui
	_ = banco.NoLote(lote).SalvarSessao(context.Background(), paramsDaSessao(r))
}

func paramsDaSessao(r RegistroSessao) sqlc.SalvarSessaoParams {
	return sqlc.SalvarSessaoParams{
		ID: r.ID, AdministradoraID: r.AdministradoraID, CondominioID: r.CondominioID, Protocolo: r.Protocolo,
		AtualizadaEm: pgtype.Timestamptz{Time: r.AtualizadaEm, Valid: true}, Dados: r.Dados,
	}
}

func (p *SessaoPostgres) Carregar(ctx context.Context, id string) (RegistroSessao, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, timeoutSessao)
	defer cancel()
	var r RegistroSessao
	var achou bool
	lote := &pgx.Batch{}
	p.CarregarNoLote(lote, id, func(lido RegistroSessao, ok bool) { r, achou = lido, ok })
	if err := p.b.Pool.SendBatch(ctx, lote).Close(); err != nil {
		return RegistroSessao{}, false, fmt.Errorf("carregar sessão %s: %w", id, err)
	}
	return r, achou, nil
}

// sqlCarregarSessao: por que não ent nem sqlc: entra no pgx.Batch que carrega o turno a frio
// (sessão, memória e pedidos numa ida), e o sqlc não junta queries diferentes num lote.
const sqlCarregarSessao = `SELECT id, administradora_id, condominio_id, protocolo, atualizada_em, dados FROM sessoes WHERE id = $1`

// CarregarNoLote põe no lote a leitura da sessão; guardar recebe o registro (ok=false se a
// sessão não existe) quando o lote roda.
func (p *SessaoPostgres) CarregarNoLote(lote *pgx.Batch, id string, guardar func(RegistroSessao, bool)) {
	lote.Queue(sqlCarregarSessao, id).QueryRow(func(row pgx.Row) error {
		var r RegistroSessao
		err := row.Scan(&r.ID, &r.AdministradoraID, &r.CondominioID, &r.Protocolo, &r.AtualizadaEm, &r.Dados)
		if errors.Is(err, pgx.ErrNoRows) {
			guardar(RegistroSessao{}, false)
			return nil
		}
		if err != nil {
			return err
		}
		guardar(r, true)
		return nil
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
