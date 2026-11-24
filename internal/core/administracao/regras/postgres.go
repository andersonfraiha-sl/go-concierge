package regras

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"go-concierge/internal/core/banco/ent"
	"go-concierge/internal/harness/ganchos"
)

const timeoutConsulta = 3 * time.Second
const timeoutLote = 30 * time.Second
const limiteMinimo = 1
const limiteMaximo = 200

type Postgres struct {
	cli       *ent.Client
	aoAlterar func([]RegraGravada)
	consultar func(context.Context, string, ...any) (*sql.Rows, error)
}

func NovoPostgres(cli *ent.Client) *Postgres { return &Postgres{cli: cli} }

func (p *Postgres) Salvar(ctx context.Context, r RegraGravada, v int) (RegraGravada, error) {
	rs, err := p.SalvarVarias(ctx, []RegraGravada{r}, []int{v})
	if err != nil {
		return RegraGravada{}, err
	}
	return rs[0], nil
}

func validarIdentidadeDaRegra(r RegraGravada) ([]int, error) {
	definition, err := Decodificar(r.Definicao)
	if err != nil {
		return nil, err
	}
	if definition.ID != "" && definition.ID != r.ID {
		return nil, fmt.Errorf("ID da definição difere da regra")
	}
	if definition.Alvo.Administradora != r.AdministradoraID || !slices.Equal(definition.Alvo.Condominios, r.Condominios) || definition.Nome != r.Nome || definition.Evento != r.Evento || definition.Ativa != r.Ativa {
		return nil, fmt.Errorf("definição da regra difere de suas colunas")
	}
	if len(r.Condominios) > 0 && strings.TrimSpace(r.AdministradoraID) == "" {
		return nil, fmt.Errorf("regra condominial exige empresa")
	}
	condos := make([]int, len(r.Condominios))
	for i, c := range r.Condominios {
		n, err := strconv.Atoi(c)
		if err != nil {
			return nil, fmt.Errorf("condomínio ERP inválido: %w", err)
		}
		if strconv.Itoa(n) != c {
			return nil, fmt.Errorf("condomínio ERP deve ser inteiro canônico")
		}
		condos[i] = n
	}
	return condos, nil
}

func (p *Postgres) SalvarVarias(ctx context.Context, rs []RegraGravada, versions []int) ([]RegraGravada, error) {
	if len(rs) != len(versions) {
		return nil, fmt.Errorf("quantidade de versões incompatível")
	}
	for _, r := range rs {
		if _, err := validarIdentidadeDaRegra(r); err != nil {
			return nil, err
		}
	}
	ctx, cancel := context.WithTimeout(ctx, timeoutLote)
	defer cancel()
	tx, err := p.cli.Tx(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]RegraGravada, len(rs))
	for i, r := range rs {
		saved, err := gravarSQL(ctx, tx.Client(), r, versions[i])
		if err != nil {
			_ = tx.Rollback()
			return nil, err
		}
		out[i] = saved
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	if p.aoAlterar != nil {
		p.aoAlterar(out)
	}
	return out, nil
}

func gravarSQL(ctx context.Context, cli *ent.Client, r RegraGravada, expected int) (RegraGravada, error) {
	condos, err := validarIdentidadeDaRegra(r)
	if err != nil {
		return r, err
	}
	if r.ID == "" {
		if expected != 0 {
			return r, ErrRegraDesconhecida
		}
		r.ID = uuid.NewString()
	}
	if uuid.Validate(r.ID) != nil {
		return r, ErrRegraDesconhecida
	}
	persistida, err := converterDefinicao(r.Definicao, true)
	if err != nil {
		return r, err
	}
	r.Versao = expected + 1
	r.AlteradaEm = time.Now().UTC().Truncate(time.Microsecond)
	scope := "GLOBAL"
	var company any
	if r.AdministradoraID != "" {
		scope = "COMPANY"
		company = r.AdministradoraID
	}
	condoJSON, err := json.Marshal(condos)
	if err != nil {
		return r, err
	}
	result, err := cli.ExecContext(ctx, `INSERT INTO hook_rules(id,scope_type,company_id,condo_ids,version,name,event,enabled,definition,updated_by,updated_at) SELECT $1,$2,$3,ARRAY(SELECT jsonb_array_elements_text($4::jsonb)::integer),$5,$6,$7,$8,$9,$10,$11 WHERE $12=0 ON CONFLICT(id) DO NOTHING`, r.ID, scope, company, condoJSON, r.Versao, r.Nome, ganchos.EventoNoBanco(r.Evento), r.Ativa, persistida, r.AlteradaPor, r.AlteradaEm, expected)
	if err != nil {
		return r, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return r, err
	}
	if expected > 0 {
		result, err = cli.ExecContext(ctx, `UPDATE hook_rules SET condo_ids=ARRAY(SELECT jsonb_array_elements_text($4::jsonb)::integer),version=$5,name=$6,event=$7,enabled=$8,definition=$9,updated_by=$10,updated_at=$11 WHERE id=$1 AND scope_type=$2 AND company_id IS NOT DISTINCT FROM $3 AND version=$12`, r.ID, scope, company, condoJSON, r.Versao, r.Nome, ganchos.EventoNoBanco(r.Evento), r.Ativa, persistida, r.AlteradaPor, r.AlteradaEm, expected)
		if err != nil {
			return r, err
		}
		n, err = result.RowsAffected()
		if err != nil {
			return r, err
		}
	}
	if n != 1 {
		if expected > 0 {
			rows, err := cli.QueryContext(ctx, `SELECT 1 FROM hook_rules WHERE id=$1`, r.ID)
			if err != nil {
				return r, err
			}
			exists := rows.Next()
			rowErr := rows.Err()
			rows.Close()
			if rowErr != nil {
				return r, rowErr
			}
			if !exists {
				return r, ErrRegraDesconhecida
			}
		}
		return r, ErrVersaoVelha
	}
	_, err = cli.ExecContext(ctx, `INSERT INTO hook_rule_versions(rule_id,version,definition,updated_by,updated_at) VALUES($1,$2,$3,$4,$5)`, r.ID, r.Versao, persistida, r.AlteradaPor, r.AlteradaEm)
	return r, err
}

func (p *Postgres) VersoesAtuais(ctx context.Context, ids []string) (map[string]int, error) {
	out := map[string]int{}
	for _, id := range ids {
		r, err := p.Ler(ctx, id)
		if errors.Is(err, ErrRegraDesconhecida) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out[id] = r.Versao
	}
	return out, nil
}

func (p *Postgres) Ler(ctx context.Context, id string) (RegraGravada, error) {
	if uuid.Validate(id) != nil {
		return RegraGravada{}, ErrRegraDesconhecida
	}
	rows, err := p.listar(ctx, `WHERE id=$1`, id)
	if err != nil {
		return RegraGravada{}, err
	}
	if len(rows) == 0 {
		return RegraGravada{}, ErrRegraDesconhecida
	}
	return rows[0], nil
}

func (p *Postgres) Listar(ctx context.Context, depois string, limite int) ([]RegraGravada, error) {
	if depois != "" && uuid.Validate(depois) != nil {
		return nil, fmt.Errorf("cursor de regra inválido")
	}
	return p.listar(ctx, `WHERE ($1='' OR id>NULLIF($1,'')::uuid) ORDER BY id LIMIT $2`, depois, min(max(limite, limiteMinimo), limiteMaximo))
}

func (p *Postgres) Ativas(ctx context.Context) ([]RegraGravada, error) { return p.ativas(ctx) }

func (p *Postgres) ObservarAlteracoes(observar func([]RegraGravada)) { p.aoAlterar = observar }
func (p *Postgres) ativas(ctx context.Context) ([]RegraGravada, error) {
	return p.listar(ctx, `WHERE enabled=true ORDER BY id`)
}

func (p *Postgres) listar(ctx context.Context, where string, args ...any) ([]RegraGravada, error) {
	consultar := p.cli.QueryContext
	if p.consultar != nil {
		consultar = p.consultar
	}
	rows, err := consultar(ctx, `SELECT id::text,version,name,event,company_id,to_json(condo_ids),enabled,definition,updated_by,updated_at FROM hook_rules `+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RegraGravada{}
	for rows.Next() {
		var r RegraGravada
		var company sql.NullString
		var condos []byte
		var event string
		if err := rows.Scan(&r.ID, &r.Versao, &r.Nome, &event, &company, &condos, &r.Ativa, &r.Definicao, &r.AlteradaPor, &r.AlteradaEm); err != nil {
			return nil, err
		}
		r.Evento = ganchos.EventoDoBanco(event)
		r.Definicao, err = converterDefinicao(r.Definicao, false)
		if err != nil {
			return nil, err
		}
		r.AdministradoraID = company.String
		var ids []int
		if err := json.Unmarshal(condos, &ids); err != nil {
			return nil, err
		}
		r.Condominios = []string{}
		for _, id := range ids {
			r.Condominios = append(r.Condominios, strconv.Itoa(id))
		}
		if _, err := validarIdentidadeDaRegra(r); err != nil {
			return nil, fmt.Errorf("regra %s inconsistente: %w", r.ID, err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (p *Postgres) LerVersao(ctx context.Context, id string, version int) (VersaoGravada, error) {
	if uuid.Validate(id) != nil {
		return VersaoGravada{}, ErrRegraDesconhecida
	}
	ctx, cancel := context.WithTimeout(ctx, timeoutConsulta)
	defer cancel()
	rows, err := p.cli.QueryContext(ctx, `SELECT v.rule_id::text,v.version,v.definition,v.updated_by,v.updated_at FROM hook_rule_versions v JOIN hook_rules r ON r.id=v.rule_id WHERE v.rule_id=$1 AND v.version=$2`, id, version)
	if err != nil {
		return VersaoGravada{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return VersaoGravada{}, err
		}
		return VersaoGravada{}, ErrRegraDesconhecida
	}
	var v VersaoGravada
	err = rows.Scan(&v.RegraID, &v.Versao, &v.Definicao, &v.AlteradaPor, &v.AlteradaEm)
	if err != nil {
		return v, err
	}
	v.Definicao, err = converterDefinicao(v.Definicao, false)
	return v, err
}
