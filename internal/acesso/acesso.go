// Package acesso decide permissões: papéis com herança, atribuições por escopo e
// concessões por administradora. Núcleo puro, sem I/O.
package acesso

import (
	"errors"
	"fmt"
	"slices"

	"go-concierge/internal/escopo"
)

type Permissao string

const (
	SkillsLer             Permissao = "skills.ler"
	SkillsEditarTextos    Permissao = "skills.editar_textos"
	SkillsEditarFluxo     Permissao = "skills.editar_fluxo"
	SkillsCriar           Permissao = "skills.criar"
	SkillsExcluir         Permissao = "skills.excluir"
	FerramentasEditar     Permissao = "ferramentas.editar"
	CondominiosHabilitar  Permissao = "condominios.habilitar"
	CondominiosEditar     Permissao = "condominios.editar"
	ConversasVer          Permissao = "conversas.ver"
	ConversasAtender      Permissao = "conversas.atender"
	PedidosVer            Permissao = "pedidos.ver"
	PedidosEditar         Permissao = "pedidos.editar"
	MemoriaVer            Permissao = "memoria.ver"
	MemoriaEsquecer       Permissao = "memoria.esquecer"
	AdministradorasEditar Permissao = "administradoras.editar"
	UsuariosEditar        Permissao = "usuarios.editar"
	StudioAvancado        Permissao = "studio.avancado"
)

type TipoEscopo string

const (
	TipoSuperlogica    TipoEscopo = "superlogica"
	TipoAdministradora TipoEscopo = "administradora"
	TipoCondominio     TipoEscopo = "condominio"
)

type Escopo struct {
	Tipo TipoEscopo
	ID   string
}

type Papel struct {
	ID, HerdaDe string
	Permissoes  []Permissao
}

type Atribuicao struct {
	PapelID string
	Escopo  Escopo
}

type Concessao struct {
	AdministradoraID, PapelID string
	Permissao                 Permissao
}

type Acesso struct {
	UsuarioID        string
	Papeis           map[string]Papel
	Atribuicoes      []Atribuicao
	Concessoes       []Concessao
	AdministradoraDe map[string]string // condomínio → administradora
	Implantacao      escopo.Implantacao
}

var ErrCicloHeranca = errors.New("ciclo na herança de papéis")

// PapeisIniciais é a tabela de papéis semeada no banco e usada pelos testes.
func PapeisIniciais() []Papel {
	return []Papel{
		{ID: "operador_administradora", Permissoes: []Permissao{SkillsLer, ConversasVer, ConversasAtender, PedidosVer, MemoriaVer}},
		{ID: "gestor_administradora", HerdaDe: "operador_administradora", Permissoes: []Permissao{CondominiosHabilitar, SkillsEditarTextos, PedidosEditar, MemoriaEsquecer}},
		{ID: "construtor_superlogica", HerdaDe: "gestor_administradora", Permissoes: []Permissao{SkillsEditarFluxo, SkillsCriar, SkillsExcluir, FerramentasEditar, CondominiosEditar, StudioAvancado}},
		{ID: "admin_superlogica", HerdaDe: "construtor_superlogica", Permissoes: []Permissao{AdministradorasEditar, UsuariosEditar}},
	}
}

// Permitido diz se o usuário tem a permissão sobre o alvo.
func Permitido(a Acesso, p Permissao, alvo Escopo) bool {
	if !implantacaoAceita(a, alvo) {
		return false
	}
	return slices.ContainsFunc(a.Atribuicoes, func(at Atribuicao) bool {
		return contem(a, at.Escopo, alvo) && atribuicaoTem(a, at, p)
	})
}

// PermitidoEmAlgum diz se alguma atribuição válida na implantação confere a permissão.
func PermitidoEmAlgum(a Acesso, p Permissao) bool {
	return slices.ContainsFunc(a.Atribuicoes, func(at Atribuicao) bool {
		return implantacaoAceitaAtribuicao(a, at.Escopo) && atribuicaoTem(a, at, p)
	})
}

// ValidarHeranca devolve ErrCicloHeranca se algum papel herda de si mesmo, direta ou indiretamente.
func ValidarHeranca(papeis []Papel) error {
	porID := indexar(papeis)
	for _, p := range papeis {
		if cadeiaCicla(porID, p.ID) {
			return fmt.Errorf("papel %s: %w", p.ID, ErrCicloHeranca)
		}
	}
	return nil
}

func indexar(papeis []Papel) map[string]Papel {
	m := make(map[string]Papel, len(papeis))
	for _, p := range papeis {
		m[p.ID] = p
	}
	return m
}

func cadeiaCicla(papeis map[string]Papel, id string) bool {
	visto := map[string]bool{}
	for atual, ok := papeis[id]; ok; atual, ok = papeis[atual.HerdaDe] {
		if visto[atual.ID] {
			return true
		}
		visto[atual.ID] = true
	}
	return false
}

// cadeia devolve o papel e seus ancestrais; tolera ciclos parando no repetido.
func cadeia(papeis map[string]Papel, id string) []Papel {
	var out []Papel
	visto := map[string]bool{}
	for atual, ok := papeis[id]; ok && !visto[atual.ID]; atual, ok = papeis[atual.HerdaDe] {
		visto[atual.ID] = true
		out = append(out, atual)
	}
	return out
}

func atribuicaoTem(a Acesso, at Atribuicao, p Permissao) bool {
	heranca := cadeia(a.Papeis, at.PapelID)
	if slices.ContainsFunc(heranca, func(pp Papel) bool { return slices.Contains(pp.Permissoes, p) }) {
		return true
	}
	return slices.ContainsFunc(a.Concessoes, func(c Concessao) bool {
		return c.Permissao == p &&
			slices.ContainsFunc(heranca, func(pp Papel) bool { return pp.ID == c.PapelID }) &&
			contem(a, Escopo{TipoAdministradora, c.AdministradoraID}, at.Escopo)
	})
}

func contem(a Acesso, externo, interno Escopo) bool {
	switch externo.Tipo {
	case TipoSuperlogica:
		return true
	case TipoAdministradora:
		return externo.ID != "" && administradoraDe(a, interno) == externo.ID
	case TipoCondominio:
		return interno == externo
	}
	return false
}

// administradoraDe devolve a administradora dona do escopo; vazio se superlogica ou desconhecida.
func administradoraDe(a Acesso, e Escopo) string {
	switch e.Tipo {
	case TipoAdministradora:
		return e.ID
	case TipoCondominio:
		return a.AdministradoraDe[e.ID]
	}
	return ""
}

func implantacaoAceita(a Acesso, alvo Escopo) bool {
	if a.Implantacao.AdministradoraFixa == "" {
		return true
	}
	return alvo.Tipo != TipoSuperlogica && a.Implantacao.Permite(administradoraDe(a, alvo))
}

func implantacaoAceitaAtribuicao(a Acesso, e Escopo) bool {
	return e.Tipo == TipoSuperlogica || a.Implantacao.Permite(administradoraDe(a, e))
}
