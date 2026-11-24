package canais

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"strings"
)

type Postgres struct{ Pool *pgxpool.Pool }

func (p *Postgres) ListarMetadados(ctx context.Context, company string) ([]Resumo, error) {
	rows, err := p.Pool.Query(ctx, `SELECT id,company_id,condo_id,coalesce(name,''),provider,channel_type,coalesce(external_channel_id,''),endpoint_identifier,enabled FROM channels WHERE ($1='' OR company_id=$1) ORDER BY company_id,name,id`, company)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Resumo{}
	for rows.Next() {
		var c Resumo
		if err := rows.Scan(&c.ID, &c.CompanyID, &c.CondoID, &c.Nome, &c.Provider, &c.Tipo, &c.Tenant, &c.Endpoint, &c.Habilitado); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func NovoPostgres(pool *pgxpool.Pool) *Postgres { return &Postgres{Pool: pool} }

const colunas = `id, company_id, condo_id, coalesce(name,''), provider, channel_type, coalesce(external_channel_id,''), endpoint_identifier, feature_mode, enabled, root_agent, coalesce(api_base_url,''), coalesce(api_key,''), coalesce(webhook_secret,''), coalesce(config_jsonb,'{}'::jsonb)`

func ler(row pgx.Row) (Canal, error) {
	var c Canal
	var raw []byte
	err := row.Scan(&c.ID, &c.CompanyID, &c.CondoID, &c.Nome, &c.Provider, &c.Tipo, &c.Tenant, &c.Endpoint, &c.Modo, &c.Habilitado, &c.Agente, &c.APIBaseURL, &c.APIKey, &c.WebhookSecret, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return c, ErrDesconhecido
	}
	if err != nil {
		return c, fmt.Errorf("ler canal: %w", err)
	}
	if json.Unmarshal(raw, &c.Integracoes) != nil || strings.TrimSpace(c.CompanyID) == "" || c.ID == uuid.Nil || strings.TrimSpace(c.Agente) == "" {
		return Canal{}, ErrInvalido
	}
	return c, nil
}

func (p *Postgres) Resolver(ctx context.Context, id Identidade) (Canal, error) {
	if strings.TrimSpace(id.Provider) == "" || strings.TrimSpace(id.Tipo) == "" || strings.TrimSpace(id.Endpoint) == "" {
		return Canal{}, ErrDesconhecido
	}
	return ler(p.Pool.QueryRow(ctx, `SELECT `+colunas+` FROM channels WHERE provider=$1 AND channel_type=$2 AND external_channel_id IS NOT DISTINCT FROM nullif($3,'') AND endpoint_identifier=$4`, id.Provider, id.Tipo, id.Tenant, id.Endpoint))
}

func (p *Postgres) PorID(ctx context.Context, company string, id uuid.UUID) (Canal, error) {
	if strings.TrimSpace(company) == "" {
		return Canal{}, ErrDesconhecido
	}
	return ler(p.Pool.QueryRow(ctx, `SELECT `+colunas+` FROM channels WHERE company_id=$1 AND id=$2`, company, id))
}

func (p *Postgres) Casos(ctx context.Context, c Canal) ([]Caso, error) {
	rows, err := p.Pool.Query(ctx, `SELECT cu.use_case_id,cu.enabled,cu.default_available,cu.roles,ov.enabled FROM company_use_cases cu JOIN channels ch ON ch.company_id=cu.company_id AND ch.id=$2 JOIN use_cases uc ON uc.id=cu.use_case_id LEFT JOIN channel_use_cases ov ON ov.company_id=cu.company_id AND ov.company_use_case_id=cu.id AND ov.channel_id=ch.id WHERE cu.company_id=$1`, c.CompanyID, c.ID)
	if err != nil {
		return nil, fmt.Errorf("ler casos do canal: %w", err)
	}
	defer rows.Close()
	out := []Caso{}
	for rows.Next() {
		var v Caso
		if err := rows.Scan(&v.ID, &v.Habilitado, &v.Padrao, &v.Papeis, &v.Override); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
