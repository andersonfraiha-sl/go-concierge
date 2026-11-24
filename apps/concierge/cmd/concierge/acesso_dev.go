package main

import (
	"context"
	"fmt"

	"go-concierge/internal/acesso"
	"go-concierge/internal/admin"
)

const (
	administradoraAvancadoDev = "adm-avancado"
	administradoraUnicaDev    = "adm-um-condominio"
	condominioUnicoDev        = "cond-unico-dev"
	gestorDeCondominioDev     = "gestor-condominio"
	gestorDeUmCondominioDev   = "gestor-um-condominio"
)

// acessoDev é o que o boot em dev escreve no RBAC: concessões de permissão a papéis e
// usuários com atribuição.
type acessoDev struct {
	conceder func(context.Context, acesso.Concessao) error
	atribuir func(context.Context, acesso.Usuario, acesso.Atribuicao) error
}

// usuarioDev é um login de dev com o papel no escopo onde vale.
type usuarioDev struct {
	usuario    acesso.Usuario
	atribuicao acesso.Atribuicao
}

// usuariosDeEscopoRestrito são os logins de dev que não enxergam além de um condomínio: um
// gestor atribuído direto a um condomínio da adm-exemplo e o gestor de uma administradora
// que só tem um condomínio.
func usuariosDeEscopoRestrito(condominioExemplo string) []usuarioDev {
	return []usuarioDev{
		{
			usuario: acesso.Usuario{ID: gestorDeCondominioDev, Nome: "Gestor de um condomínio (dev)"},
			atribuicao: acesso.Atribuicao{
				PapelID:          "gestor_administradora",
				Escopo:           acesso.Escopo{Tipo: acesso.TipoCondominio, ID: condominioExemplo},
				AdministradoraID: "adm-exemplo",
			},
		},
		{
			usuario: acesso.Usuario{ID: gestorDeUmCondominioDev, Nome: "Gestor Administradora de um condomínio (dev)"},
			atribuicao: acesso.Atribuicao{
				PapelID: "gestor_administradora",
				Escopo:  acesso.Escopo{Tipo: acesso.TipoAdministradora, ID: administradoraUnicaDev},
			},
		},
	}
}

// semearAcessoDev cria as administradoras e os logins de dev e concede o Avançado à
// adm-avancado. Roda só no boot em dev (nunca numa migration, para não chegar a produção) e
// pode repetir: administradora, condomínio, usuário e atribuição só são gravados se faltam, e a
// concessão é idempotente.
func semearAcessoDev(ctx context.Context, repo admin.Repositorio, rbac acessoDev, condominioExemplo string) error {
	ctx, cancel := context.WithTimeout(ctx, timeoutBoot)
	defer cancel()
	if err := garantirMapeamentoDev(ctx, repo); err != nil {
		return fmt.Errorf("semear acesso de dev: %w", err)
	}
	c := acesso.Concessao{AdministradoraID: administradoraAvancadoDev, PapelID: "gestor_administradora", Permissao: acesso.StudioAvancado}
	if err := rbac.conceder(ctx, c); err != nil {
		return fmt.Errorf("semear acesso de dev: %w", err)
	}
	for _, u := range usuariosDeEscopoRestrito(condominioExemplo) {
		if err := rbac.atribuir(ctx, u.usuario, u.atribuicao); err != nil {
			return fmt.Errorf("semear acesso de dev: %w", err)
		}
	}
	return nil
}

func garantirMapeamentoDev(ctx context.Context, repo admin.Repositorio) error {
	m, err := repo.Ler(ctx)
	if err != nil {
		return err
	}
	administradoras := []admin.Administradora{
		{ID: administradoraAvancadoDev, Nome: "Administradora com Avançado (dev)"},
		{ID: administradoraUnicaDev, Nome: "Administradora de um condomínio (dev)"},
	}
	for _, a := range administradoras {
		if temAdministradora(m, a.ID) {
			continue
		}
		if _, err := repo.SalvarAdministradora(ctx, a); err != nil {
			return err
		}
	}
	if temCondominio(m, condominioUnicoDev) {
		return nil
	}
	_, err = repo.SalvarCondominio(ctx, admin.Condominio{ID: condominioUnicoDev, Nome: "Condomínio único (dev)", Administradora: administradoraUnicaDev})
	return err
}

func temAdministradora(m admin.Mapeamento, id string) bool {
	for _, a := range m.Administradoras {
		if a.ID == id {
			return true
		}
	}
	return false
}

func temCondominio(m admin.Mapeamento, id string) bool {
	for _, c := range m.Condominios {
		if c.ID == id {
			return true
		}
	}
	return false
}
