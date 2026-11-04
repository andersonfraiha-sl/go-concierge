package main

import (
	"context"
	"fmt"

	"go-concierge/internal/acesso"
	"go-concierge/internal/admin"
)

const administradoraAvancadoDev = "adm-avancado"

// semearAcessoDev cria a administradora do gestor de dev com o Avançado e lhe concede o
// Avançado. Roda só no boot em dev (nunca numa migration, para não chegar a produção) e pode
// repetir: a administradora só é gravada se falta, e a concessão é idempotente.
func semearAcessoDev(ctx context.Context, repo admin.Repositorio, conceder func(context.Context, acesso.Concessao) error) error {
	ctx, cancel := context.WithTimeout(ctx, timeoutBoot)
	defer cancel()
	if err := garantirAdministradoraAvancado(ctx, repo); err != nil {
		return fmt.Errorf("semear acesso de dev: %w", err)
	}
	c := acesso.Concessao{AdministradoraID: administradoraAvancadoDev, PapelID: "gestor_administradora", Permissao: acesso.StudioAvancado}
	if err := conceder(ctx, c); err != nil {
		return fmt.Errorf("semear acesso de dev: %w", err)
	}
	return nil
}

func garantirAdministradoraAvancado(ctx context.Context, repo admin.Repositorio) error {
	m, err := repo.Ler(ctx)
	if err != nil {
		return err
	}
	for _, a := range m.Administradoras {
		if a.ID == administradoraAvancadoDev {
			return nil
		}
	}
	_, err = repo.SalvarAdministradora(ctx, admin.Administradora{ID: administradoraAvancadoDev, Nome: "Administradora com Avançado (dev)"})
	return err
}
