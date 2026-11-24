package admin

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"go-concierge/internal/core/banco/ent"
	"go-concierge/internal/falha"
	"strconv"
	"strings"
	"time"
)

type Postgres struct{ cli *ent.Client }

func NovoPostgres(cli *ent.Client) *Postgres { return &Postgres{cli: cli} }

var ErrCadastroExterno = errors.New("empresas e condomínios são identificadores externos; canais e disponibilidade são preparados por SQL/seed")

func (p *Postgres) Ler(ctx context.Context) (Mapeamento, error) {
	m := Mapeamento{}.Normalizado()
	rows, err := p.cli.QueryContext(ctx, `SELECT DISTINCT company_id, condo_id FROM channels ORDER BY company_id, condo_id`)
	if err != nil {
		return m, falha.ComPilha(fmt.Errorf("ler escopos de canais: %w", err))
	}
	defer rows.Close()
	companies := map[string]bool{}
	for rows.Next() {
		var company string
		var condo sql.NullInt64
		if err := rows.Scan(&company, &condo); err != nil {
			return m, err
		}
		if !companies[company] {
			m.Administradoras = append(m.Administradoras, Administradora{ID: company, Nome: company})
			companies[company] = true
		}
		if condo.Valid {
			id := strconv.FormatInt(condo.Int64, 10)
			m.Condominios = append(m.Condominios, Condominio{ID: id, Nome: id, Administradora: company})
		}
	}
	if err := rows.Err(); err != nil {
		return m, err
	}
	return m, nil
}

func (p *Postgres) Conteudo(ctx context.Context, company string, condo *int) (Mapeamento, error) {
	return p.conteudo(ctx, company, condo, true)
}

func (p *Postgres) ConteudoConfigurado(ctx context.Context, company string, condo *int) (Mapeamento, error) {
	return p.conteudo(ctx, company, condo, false)
}

func (p *Postgres) conteudo(ctx context.Context, company string, condo *int, herdar bool) (Mapeamento, error) {
	m := Mapeamento{}.Normalizado()
	if strings.TrimSpace(company) == "" && (herdar || condo != nil) {
		return m, fmt.Errorf("conteúdo exige empresa")
	}
	if company != "" {
		m.Administradoras = []Administradora{{ID: company, Nome: company}}
	}
	var id any
	scopeAlvo := "COMPANY"
	if company == "" {
		scopeAlvo = "GLOBAL"
	}
	target := Padrao
	if condo != nil {
		id = *condo
		scopeAlvo = "CONDO"
		target = strconv.Itoa(*condo)
		m.Condominios = []Condominio{{ID: target, Nome: target, Administradora: company}}
	}
	rows, err := p.cli.QueryContext(ctx, `SELECT scope_type, resource_key, data FROM payloads WHERE (scope_type='GLOBAL' OR (company_id=$1 AND (scope_type='COMPANY' OR (scope_type='CONDO' AND condo_id=$2)))) AND ($3 OR scope_type=$4) ORDER BY CASE scope_type WHEN 'GLOBAL' THEN 0 WHEN 'COMPANY' THEN 1 ELSE 2 END`, company, id, herdar, scopeAlvo)
	if err != nil {
		return m, fmt.Errorf("ler payloads: %w", err)
	}
	for rows.Next() {
		var scope, key string
		var data []byte
		if err := rows.Scan(&scope, &key, &data); err != nil {
			rows.Close()
			return m, err
		}
		var payload map[string]any
		if err := json.Unmarshal(data, &payload); err != nil {
			rows.Close()
			return m, err
		}
		dest := Padrao
		if scope == "CONDO" {
			dest = target
		}
		if m.Payloads[dest] == nil {
			m.Payloads[dest] = map[string]map[string]any{}
		}
		if m.Payloads[dest][key] == nil {
			m.Payloads[dest][key] = map[string]any{}
		}
		for k, v := range payload {
			m.Payloads[dest][key][k] = v
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return m, err
	}
	rows, err = p.cli.QueryContext(ctx, `SELECT scope_type, general_text, by_use_case FROM guidelines WHERE (scope_type='GLOBAL' OR (company_id=$1 AND (scope_type='COMPANY' OR (scope_type='CONDO' AND condo_id=$2)))) AND ($3 OR scope_type=$4) ORDER BY CASE scope_type WHEN 'GLOBAL' THEN 0 WHEN 'COMPANY' THEN 1 ELSE 2 END`, company, id, herdar, scopeAlvo)
	if err != nil {
		return m, fmt.Errorf("ler orientações: %w", err)
	}
	for rows.Next() {
		var scope, general string
		var data []byte
		if err := rows.Scan(&scope, &general, &data); err != nil {
			rows.Close()
			return m, err
		}
		var byCase map[string]string
		if err := json.Unmarshal(data, &byCase); err != nil {
			rows.Close()
			return m, err
		}
		dest := Padrao
		if scope == "CONDO" {
			dest = target
		}
		o := m.Orientacoes[dest]
		if o.PorCaso == nil {
			o.PorCaso = map[string]string{}
		}
		o.Geral = concatenar(o.Geral, general)
		for k, v := range byCase {
			o.PorCaso[k] = concatenar(o.PorCaso[k], v)
		}
		m.Orientacoes[dest] = o
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return m, err
	}
	rows, err = p.cli.QueryContext(ctx, `SELECT use_case_id, enabled, to_json(roles) FROM company_use_cases WHERE company_id=$1`, company)
	if err != nil {
		return m, fmt.Errorf("ler casos empresariais: %w", err)
	}
	m.Habilitacoes[Padrao] = map[string]Habilitacao{}
	for rows.Next() {
		var caseID string
		var enabled bool
		var roles []byte
		if err := rows.Scan(&caseID, &enabled, &roles); err != nil {
			rows.Close()
			return m, err
		}
		var papeis []string
		if err := json.Unmarshal(roles, &papeis); err != nil {
			rows.Close()
			return m, err
		}
		m.Habilitacoes[Padrao][caseID] = Habilitacao{Habilitado: enabled, Papeis: papeis}
	}
	err = rows.Err()
	rows.Close()
	return m, err
}

func concatenar(a, b string) string {
	if strings.TrimSpace(b) == "" {
		return a
	}
	if a == "" {
		return b
	}
	return a + "\n\n" + b
}

func (p *Postgres) SalvarAdministradora(context.Context, Administradora) (Mapeamento, error) {
	return Mapeamento{}, ErrCadastroExterno
}

func (p *Postgres) CriarAdministradora(context.Context, Administradora) (Mapeamento, error) {
	return Mapeamento{}, ErrCadastroExterno
}

func (p *Postgres) EditarAdministradora(context.Context, Administradora) (Mapeamento, error) {
	return Mapeamento{}, ErrCadastroExterno
}

func (p *Postgres) RemoverAdministradora(context.Context, string) (Mapeamento, error) {
	return Mapeamento{}, ErrCadastroExterno
}

func (p *Postgres) SalvarCondominio(context.Context, Condominio) (Mapeamento, error) {
	return Mapeamento{}, ErrCadastroExterno
}

func (p *Postgres) CriarCondominio(context.Context, Condominio) (Mapeamento, error) {
	return Mapeamento{}, ErrCadastroExterno
}

func (p *Postgres) EditarCondominio(context.Context, Condominio, Chaves) (Mapeamento, error) {
	return Mapeamento{}, ErrCadastroExterno
}

func (p *Postgres) RemoverCondominio(context.Context, Alvo) (Mapeamento, error) {
	return Mapeamento{}, ErrCadastroExterno
}

func (p *Postgres) SalvarHabilitacao(context.Context, Alvo, string, Habilitacao, bool) (Mapeamento, error) {
	return Mapeamento{}, ErrCadastroExterno
}

func alvoDeConteudo(a Alvo) (string, any, error) {
	if a.Condominio == Padrao || a.Condominio == "" {
		if a.Dona == "" {
			return "GLOBAL", nil, nil
		}
		return "COMPANY", nil, nil
	}
	if strings.TrimSpace(a.Dona) == "" {
		return "", nil, fmt.Errorf("conteúdo condominial exige empresa")
	}
	condo, err := strconv.Atoi(a.Condominio)
	if err != nil {
		return "", nil, fmt.Errorf("condomínio ERP inválido: %w", err)
	}
	return "CONDO", condo, nil
}

func (p *Postgres) SalvarOrientacao(ctx context.Context, a Alvo, o Orientacao) (Mapeamento, error) {
	scope, condo, err := alvoDeConteudo(a)
	if err != nil {
		return Mapeamento{}, err
	}
	porCaso := map[string]string{}
	for caso, texto := range o.PorCaso {
		if texto = strings.TrimSpace(texto); texto != "" {
			porCaso[caso] = texto
		}
	}
	o.PorCaso = porCaso
	o.Geral = strings.TrimSpace(o.Geral)
	if o.Geral == "" && len(o.PorCaso) == 0 {
		_, err := p.cli.ExecContext(ctx, `DELETE FROM guidelines WHERE scope_type=$1 AND company_id IS NOT DISTINCT FROM NULLIF($2,'') AND condo_id IS NOT DISTINCT FROM $3`, scope, a.Dona, condo)
		if err != nil {
			return Mapeamento{}, err
		}
		return p.depoisDeGravar(ctx, a)
	}
	data, err := json.Marshal(o.PorCaso)
	if err != nil {
		return Mapeamento{}, err
	}
	_, err = p.cli.ExecContext(ctx, `INSERT INTO guidelines(scope_type,company_id,condo_id,general_text,by_use_case) VALUES ($1,NULLIF($2,''),$3,$4,$5) ON CONFLICT(scope_type,company_id,condo_id) DO UPDATE SET general_text=EXCLUDED.general_text,by_use_case=EXCLUDED.by_use_case`, scope, a.Dona, condo, o.Geral, data)
	if err != nil {
		return Mapeamento{}, err
	}
	return p.depoisDeGravar(ctx, a)
}

func (p *Postgres) SalvarPayload(ctx context.Context, a Alvo, recurso string, payload map[string]any) (Mapeamento, error) {
	scope, condo, err := alvoDeConteudo(a)
	if err != nil {
		return Mapeamento{}, err
	}
	if strings.TrimSpace(recurso) == "" {
		return Mapeamento{}, fmt.Errorf("recurso obrigatório")
	}
	if payload == nil {
		_, err := p.cli.ExecContext(ctx, `DELETE FROM payloads WHERE scope_type=$1 AND company_id IS NOT DISTINCT FROM NULLIF($2,'') AND condo_id IS NOT DISTINCT FROM $3 AND resource_key=$4`, scope, a.Dona, condo, recurso)
		if err != nil {
			return Mapeamento{}, err
		}
		return p.depoisDeGravar(ctx, a)
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return Mapeamento{}, err
	}
	_, err = p.cli.ExecContext(ctx, `INSERT INTO payloads(scope_type,company_id,condo_id,resource_key,data) VALUES ($1,NULLIF($2,''),$3,$4,$5) ON CONFLICT(scope_type,company_id,condo_id,resource_key) DO UPDATE SET data=EXCLUDED.data`, scope, a.Dona, condo, recurso, data)
	if err != nil {
		return Mapeamento{}, err
	}
	return p.depoisDeGravar(ctx, a)
}

func (p *Postgres) SalvarAjustes(ctx context.Context, a Alvo, ajustes map[string]map[string]any) (Mapeamento, error) {
	scope, condo, err := alvoDeConteudo(a)
	if err != nil {
		return Mapeamento{}, err
	}
	for resource := range ajustes {
		if strings.TrimSpace(resource) == "" {
			return Mapeamento{}, fmt.Errorf("recurso obrigatório")
		}
	}
	tx, err := p.cli.Tx(ctx)
	if err != nil {
		return Mapeamento{}, err
	}
	cli := tx.Client()
	for resource, delta := range ajustes {
		values := map[string]any{}
		remove := []string{}
		for key, value := range delta {
			if value == nil || fmt.Sprint(value) == "" {
				remove = append(remove, key)
			} else {
				values[key] = value
			}
		}
		data, err := json.Marshal(values)
		if err != nil {
			_ = tx.Rollback()
			return Mapeamento{}, err
		}
		keys, err := json.Marshal(remove)
		if err != nil {
			_ = tx.Rollback()
			return Mapeamento{}, err
		}
		_, err = cli.ExecContext(ctx, `INSERT INTO payloads(scope_type,company_id,condo_id,resource_key,data) VALUES($1,NULLIF($2,''),$3,$4,$5) ON CONFLICT(scope_type,company_id,condo_id,resource_key) DO UPDATE SET data=(payloads.data || EXCLUDED.data) - ARRAY(SELECT jsonb_array_elements_text($6::jsonb))`, scope, a.Dona, condo, resource, data, keys)
		if err != nil {
			_ = tx.Rollback()
			return Mapeamento{}, err
		}
		_, err = cli.ExecContext(ctx, `DELETE FROM payloads WHERE scope_type=$1 AND company_id IS NOT DISTINCT FROM NULLIF($2,'') AND condo_id IS NOT DISTINCT FROM $3 AND resource_key=$4 AND data='{}'::jsonb`, scope, a.Dona, condo, resource)
		if err != nil {
			_ = tx.Rollback()
			return Mapeamento{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Mapeamento{}, err
	}
	return p.depoisDeGravar(ctx, a)
}

func (p *Postgres) depoisDeGravar(ctx context.Context, a Alvo) (Mapeamento, error) {
	if a.Dona == "" {
		return p.ConteudoConfigurado(ctx, "", nil)
	}
	var condo *int
	if a.Condominio != "" && a.Condominio != Padrao {
		n, err := strconv.Atoi(a.Condominio)
		if err != nil {
			return Mapeamento{}, err
		}
		condo = &n
	}
	m, err := p.ConteudoConfigurado(ctx, a.Dona, condo)
	m.AtualizadoEm = time.Now()
	return m, err
}
