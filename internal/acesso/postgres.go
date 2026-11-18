package acesso

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go-concierge/internal/banco/ent"
	"go-concierge/internal/escopo"
)

const timeoutCarregar = 3 * time.Second

var ErrUsuarioDesconhecido = errors.New("usuário desconhecido")

type Postgres struct{ cli *ent.Client }

func NovoPostgres(cli *ent.Client) *Postgres { return &Postgres{cli: cli} }

func (p *Postgres) Carregar(ctx context.Context, user string, imp escopo.Implantacao) (Acesso, error) {
	ctx, cancel := context.WithTimeout(ctx, timeoutCarregar)
	defer cancel()
	a := Acesso{UsuarioID: user, Papeis: map[string]Papel{}, AdministradoraDe: map[string]string{}, Implantacao: imp}
	rows, err := p.cli.QueryContext(ctx, `SELECT name FROM users WHERE id=$1`, user)
	if err != nil {
		return a, err
	}
	if !rows.Next() {
		err = rows.Err()
		rows.Close()
		if err != nil {
			return a, err
		}
		return a, ErrUsuarioDesconhecido
	}
	if err := rows.Scan(&a.Nome); err != nil {
		rows.Close()
		return a, err
	}
	rows.Close()
	rows, err = p.cli.QueryContext(ctx, `SELECT id,inherits_from FROM roles`)
	if err != nil {
		return a, err
	}
	for rows.Next() {
		var id string
		var parent sql.NullString
		if err := rows.Scan(&id, &parent); err != nil {
			rows.Close()
			return a, err
		}
		a.Papeis[id] = Papel{ID: id, HerdaDe: parent.String}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return a, err
	}
	rows, err = p.cli.QueryContext(ctx, `SELECT role_id,permission FROM role_permissions`)
	if err != nil {
		return a, err
	}
	for rows.Next() {
		var id, permission string
		if err := rows.Scan(&id, &permission); err != nil {
			rows.Close()
			return a, err
		}
		role := a.Papeis[id]
		role.Permissoes = append(role.Permissoes, Permissao(permission))
		a.Papeis[id] = role
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return a, err
	}
	var all []Papel
	for _, role := range a.Papeis {
		all = append(all, role)
	}
	if err := ValidarHeranca(all); err != nil {
		return a, err
	}
	rows, err = p.cli.QueryContext(ctx, `SELECT role_id,scope_type,company_id,condo_id FROM role_assignments WHERE user_id=$1`, user)
	if err != nil {
		return a, err
	}
	for rows.Next() {
		var role, scope string
		var company sql.NullString
		var condo sql.NullInt64
		if err := rows.Scan(&role, &scope, &company, &condo); err != nil {
			rows.Close()
			return a, err
		}
		at := Atribuicao{PapelID: role, AdministradoraID: company.String}
		switch scope {
		case "PLATFORM":
			at.Escopo = Escopo{Tipo: TipoSuperlogica}
		case "COMPANY":
			at.Escopo = Escopo{Tipo: TipoAdministradora, ID: company.String}
		case "CONDO":
			if !company.Valid || !condo.Valid {
				rows.Close()
				return a, fmt.Errorf("atribuição condominial inválida")
			}
			at.Escopo = EscopoCondominio(company.String, strconv.FormatInt(condo.Int64, 10))
			a.AdministradoraDe[at.Escopo.ID] = company.String
		default:
			rows.Close()
			return a, fmt.Errorf("escopo desconhecido")
		}
		a.Atribuicoes = append(a.Atribuicoes, at)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return a, err
	}
	rows, err = p.cli.QueryContext(ctx, `SELECT company_id,role_id,permission FROM company_role_grants WHERE company_id IN (SELECT company_id FROM role_assignments WHERE user_id=$1 AND company_id IS NOT NULL)`, user)
	if err != nil {
		return a, err
	}
	for rows.Next() {
		var c Concessao
		var permission string
		if err := rows.Scan(&c.AdministradoraID, &c.PapelID, &permission); err != nil {
			rows.Close()
			return a, err
		}
		c.Permissao = Permissao(permission)
		a.Concessoes = append(a.Concessoes, c)
	}
	err = rows.Err()
	rows.Close()
	return a, err
}

func (p *Postgres) Conceder(ctx context.Context, c Concessao) error {
	if strings.TrimSpace(c.AdministradoraID) == "" {
		return fmt.Errorf("concessão exige empresa")
	}
	_, err := p.cli.ExecContext(ctx, `INSERT INTO company_role_grants(company_id,role_id,permission) VALUES($1,$2,$3) ON CONFLICT(company_id,role_id,permission) DO NOTHING`, c.AdministradoraID, c.PapelID, string(c.Permissao))
	return err
}

func (p *Postgres) Atribuir(ctx context.Context, u Usuario, at Atribuicao) error {
	var company, condo any
	scope := ""
	switch at.Escopo.Tipo {
	case TipoSuperlogica:
		scope = "PLATFORM"
	case TipoAdministradora:
		scope = "COMPANY"
		if strings.TrimSpace(at.Escopo.ID) == "" {
			return fmt.Errorf("atribuição exige empresa")
		}
		company = at.Escopo.ID
	case TipoCondominio:
		scope = "CONDO"
		c, id, ok := CondominioDoEscopo(at.Escopo)
		if !ok || at.AdministradoraID != c {
			return fmt.Errorf("atribuição condominial exige empresa e condomínio ERP explícitos")
		}
		n, err := strconv.Atoi(id)
		if err != nil {
			return err
		}
		company, condo = c, n
	default:
		return fmt.Errorf("escopo desconhecido")
	}
	tx, err := p.cli.Tx(ctx)
	if err != nil {
		return err
	}
	cli := tx.Client()
	if _, err := cli.ExecContext(ctx, `INSERT INTO users(id,name) VALUES($1,$2) ON CONFLICT(id) DO NOTHING`, u.ID, u.Nome); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err := cli.ExecContext(ctx, `INSERT INTO role_assignments(user_id,role_id,scope_type,company_id,condo_id) VALUES($1,$2,$3,$4,$5) ON CONFLICT(user_id,role_id,scope_type,company_id,condo_id) DO NOTHING`, u.ID, at.PapelID, scope, company, condo); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}
