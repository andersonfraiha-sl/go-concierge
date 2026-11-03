package acesso

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go-concierge/internal/banco/ent"
	"go-concierge/internal/banco/ent/usuario"
	"go-concierge/internal/escopo"
)

const timeoutCarregar = 5 * time.Second

var ErrUsuarioDesconhecido = errors.New("usuário desconhecido")

// Postgres carrega o Acesso de um usuário a partir das tabelas de papéis e atribuições.
type Postgres struct{ cli *ent.Client }

func NovoPostgres(cli *ent.Client) *Postgres { return &Postgres{cli: cli} }

// Carregar monta o Acesso do usuário em 3 consultas do ent: o usuário com suas atribuições,
// todos os papéis com permissões e concessões, e todos os condomínios com a administradora
// atual de cada um. Papéis, concessões e condomínios vêm inteiros porque Permitido precisa da
// cadeia de herança e do mapa condomínio → administradora para qualquer alvo.
func (p *Postgres) Carregar(ctx context.Context, usuarioID string, imp escopo.Implantacao) (Acesso, error) {
	ctx, cancel := context.WithTimeout(ctx, timeoutCarregar)
	defer cancel()

	u, err := p.cli.Usuario.Query().Where(usuario.ID(usuarioID)).WithAtribuicoes().Only(ctx)
	if ent.IsNotFound(err) {
		return Acesso{}, fmt.Errorf("carregar acesso de %s: %w", usuarioID, ErrUsuarioDesconhecido)
	}
	if err != nil {
		return Acesso{}, fmt.Errorf("carregar usuário %s: %w", usuarioID, err)
	}
	papeis, err := p.cli.Papel.Query().WithPermissoes().WithConcessoes().All(ctx)
	if err != nil {
		return Acesso{}, fmt.Errorf("carregar papéis: %w", err)
	}
	condominios, err := p.cli.Condominio.Query().All(ctx)
	if err != nil {
		return Acesso{}, fmt.Errorf("carregar condomínios: %w", err)
	}
	return Acesso{
		UsuarioID:        usuarioID,
		Papeis:           papeisDe(papeis),
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
		out = append(out, Atribuicao{PapelID: r.PapelID, Escopo: Escopo{Tipo: TipoEscopo(r.EscopoTipo), ID: r.EscopoID}})
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
