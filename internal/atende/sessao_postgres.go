package atende

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"entgo.io/ent/dialect/sql"

	"go-concierge/internal/banco/ent"
	entsessao "go-concierge/internal/banco/ent/sessao"
)

// timeoutSessao limita cada leitura ou escrita de sessão no Postgres.
const timeoutSessao = 3 * time.Second

// SessaoPostgres guarda as sessões do ATENDE na tabela sessoes.
type SessaoPostgres struct {
	cli *ent.Client
}

func NovoSessaoPostgres(cli *ent.Client) *SessaoPostgres { return &SessaoPostgres{cli: cli} }

func (p *SessaoPostgres) Salvar(ctx context.Context, r RegistroSessao) error {
	ctx, cancel := context.WithTimeout(ctx, timeoutSessao)
	defer cancel()
	err := p.cli.Sessao.Create().
		SetID(r.ID).
		SetAdministradoraID(r.AdministradoraID).
		SetCondominioID(r.CondominioID).
		SetProtocolo(r.Protocolo).
		SetAtualizadaEm(r.AtualizadaEm).
		SetDados(json.RawMessage(r.Dados)).
		OnConflictColumns(entsessao.FieldID).
		UpdateNewValues().
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("salvar sessão %s: %w", r.ID, err)
	}
	return nil
}

func (p *SessaoPostgres) Carregar(ctx context.Context, id string) (RegistroSessao, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, timeoutSessao)
	defer cancel()
	linha, err := p.cli.Sessao.Get(ctx, id)
	if ent.IsNotFound(err) {
		return RegistroSessao{}, false, nil
	}
	if err != nil {
		return RegistroSessao{}, false, fmt.Errorf("carregar sessão %s: %w", id, err)
	}
	return registroDaLinha(linha), true, nil
}

func (p *SessaoPostgres) Listar(ctx context.Context, administradoraID string, limite int) ([]RegistroSessao, error) {
	ctx, cancel := context.WithTimeout(ctx, timeoutSessao)
	defer cancel()
	q := p.cli.Sessao.Query()
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
	if _, err := p.cli.Sessao.Delete().Where(entsessao.ID(id)).Exec(ctx); err != nil {
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
