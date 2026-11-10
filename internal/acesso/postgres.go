package acesso

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"

	"go-concierge/internal/banco/ent"
	"go-concierge/internal/banco/ent/atribuicao"
	"go-concierge/internal/banco/ent/concessao"
	"go-concierge/internal/banco/ent/usuario"
	"go-concierge/internal/escopo"
	"go-concierge/internal/falha"
)

const timeoutCarregar = 3 * time.Second

// ErrUsuarioDesconhecido indica que o id não existe na tabela de usuários.
var ErrUsuarioDesconhecido = errors.New("usuário desconhecido")

// Postgres carrega o Acesso de um usuário a partir das tabelas de papéis e atribuições.
type Postgres struct{ cli *ent.Client }

// NovoPostgres usa cli, que o chamador abre e fecha.
func NovoPostgres(cli *ent.Client) *Postgres { return &Postgres{cli: cli} }

// Carregar monta o Acesso do usuário em 3 chamadas ao ent (cerca de 6 comandos SQL, pois cada
// With* é uma consulta própria): o usuário com suas atribuições, todos os papéis com
// permissões e concessões, e todos os condomínios com a administradora atual de cada um.
// Papéis, concessões e condomínios vêm inteiros porque Permitido precisa da cadeia de herança
// e do mapa condomínio → administradora para qualquer alvo. Herança cíclica no banco é erro:
// o acesso falha fechado em vez de seguir com papéis incoerentes.
func (p *Postgres) Carregar(ctx context.Context, usuarioID string, imp escopo.Implantacao) (Acesso, error) {
	ctx, cancel := context.WithTimeout(ctx, timeoutCarregar)
	defer cancel()

	u, err := p.cli.Usuario.Query().Where(usuario.ID(usuarioID)).WithAtribuicoes().Only(ctx)
	if ent.IsNotFound(err) {
		return Acesso{}, fmt.Errorf("carregar acesso de %s: %w", usuarioID, ErrUsuarioDesconhecido)
	}
	if err != nil {
		return Acesso{}, falha.ComPilha(fmt.Errorf("carregar usuário %s: %w", usuarioID, err))
	}
	papeis, err := p.cli.Papel.Query().WithPermissoes().WithConcessoes().All(ctx)
	if err != nil {
		return Acesso{}, falha.ComPilha(fmt.Errorf("carregar papéis: %w", err))
	}
	// ponytail: lê todos os condomínios a cada Carregar; quando a tabela passar de alguns
	// milhares, trocar por consulta só dos ids de atribuições e dos alvos da requisição.
	condominios, err := p.cli.Condominio.Query().All(ctx)
	if err != nil {
		return Acesso{}, falha.ComPilha(fmt.Errorf("carregar condomínios: %w", err))
	}
	porID := papeisDe(papeis)
	if err := ValidarHeranca(slices.Collect(maps.Values(porID))); err != nil {
		return Acesso{}, falha.ComPilha(fmt.Errorf("carregar papéis: %w", err))
	}
	return Acesso{
		UsuarioID:        usuarioID,
		Nome:             u.Nome,
		Papeis:           porID,
		Atribuicoes:      atribuicoesDe(u.Edges.Atribuicoes),
		Concessoes:       concessoesDe(papeis),
		AdministradoraDe: administradorasPorCondominio(condominios),
		Implantacao:      imp,
	}, nil
}

func papeisDe(rows []*ent.Papel) map[string]Papel {
	out := make(map[string]Papel, len(rows))
	for _, r := range rows {
		papel := Papel{ID: r.ID}
		if r.HerdaDe != nil {
			papel.HerdaDe = *r.HerdaDe
		}
		for _, perm := range r.Edges.Permissoes {
			papel.Permissoes = append(papel.Permissoes, Permissao(perm.Permissao))
		}
		out[r.ID] = papel
	}
	return out
}

func atribuicoesDe(rows []*ent.Atribuicao) []Atribuicao {
	out := make([]Atribuicao, 0, len(rows))
	for _, r := range rows {
		at := Atribuicao{PapelID: r.PapelID, Escopo: Escopo{Tipo: TipoEscopo(r.EscopoTipo), ID: r.EscopoID}}
		if r.AdministradoraID != nil {
			at.AdministradoraID = *r.AdministradoraID
		}
		out = append(out, at)
	}
	return out
}

func concessoesDe(papeis []*ent.Papel) []Concessao {
	var out []Concessao
	for _, p := range papeis {
		for _, c := range p.Edges.Concessoes {
			out = append(out, Concessao{AdministradoraID: c.AdministradoraID, PapelID: c.PapelID, Permissao: Permissao(c.Permissao)})
		}
	}
	return out
}

// administradorasPorCondominio mapeia condomínio → administradora atual; condomínio sem administradora
// fica fora do mapa e Permitido o trata como desconhecido.
func administradorasPorCondominio(rows []*ent.Condominio) map[string]string {
	out := make(map[string]string, len(rows))
	for _, r := range rows {
		if r.AdministradoraID != nil {
			out[r.ID] = *r.AdministradoraID
		}
	}
	return out
}

// Conceder libera a permissão ao papel dentro da administradora (que já existe). Repetir a
// mesma concessão não faz nada.
func (p *Postgres) Conceder(ctx context.Context, c Concessao) error {
	ctx, cancel := context.WithTimeout(ctx, timeoutCarregar)
	defer cancel()
	err := p.cli.Concessao.Create().
		SetAdministradoraID(c.AdministradoraID).SetPapelID(c.PapelID).SetPermissao(string(c.Permissao)).
		OnConflictColumns(concessao.FieldAdministradoraID, concessao.FieldPapelID, concessao.FieldPermissao).
		DoNothing().Exec(ctx)
	// o ent devolve sql.ErrNoRows quando o DoNothing não insere nada
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return falha.ComPilha(fmt.Errorf("conceder %s a %s em %s: %w", c.Permissao, c.PapelID, c.AdministradoraID, err))
	}
	return nil
}

// Atribuir garante o usuário e lhe dá o papel no escopo. Repetir não faz nada: o usuário e a
// atribuição só são gravados se faltam. Em escopo de condomínio, AdministradoraID é obrigatório.
func (p *Postgres) Atribuir(ctx context.Context, u Usuario, at Atribuicao) error {
	ctx, cancel := context.WithTimeout(ctx, timeoutCarregar)
	defer cancel()
	err := p.cli.Usuario.Create().SetID(u.ID).SetNome(u.Nome).
		OnConflictColumns(usuario.FieldID).DoNothing().Exec(ctx)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return falha.ComPilha(fmt.Errorf("criar usuário %s: %w", u.ID, err))
	}
	criar := p.cli.Atribuicao.Create().SetUsuarioID(u.ID).SetPapelID(at.PapelID).
		SetEscopoTipo(string(at.Escopo.Tipo)).SetEscopoID(at.Escopo.ID)
	if at.AdministradoraID != "" {
		criar.SetAdministradoraID(at.AdministradoraID)
	}
	err = criar.OnConflictColumns(atribuicao.FieldUsuarioID, atribuicao.FieldPapelID, atribuicao.FieldEscopoTipo, atribuicao.FieldEscopoID).
		DoNothing().Exec(ctx)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return falha.ComPilha(fmt.Errorf("atribuir %s a %s: %w", at.PapelID, u.ID, err))
	}
	return nil
}
