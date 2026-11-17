package main

import (
	"context"
	"fmt"

	"go-concierge/internal/acesso"
)

// atribuirFunc é o acesso.Postgres.Atribuir; os testes usam um fake.
type atribuirFunc func(context.Context, acesso.Usuario, acesso.Atribuicao) error

// garantirAdminInicial dá o papel admin_superlogica ao e-mail de ACESSO_ADMIN_INICIAL. Sem ele
// o primeiro login num ambiente novo não teria quem cadastrar os demais. Pode repetir a cada
// boot: usuário e atribuição só são gravados se faltam. Vazio não faz nada.
func garantirAdminInicial(ctx context.Context, bruto string, atribuir atribuirFunc) error {
	if bruto == "" {
		return nil
	}
	email, ok := normalizarEmail(bruto)
	if !ok {
		return errAdminInicialInvalido
	}
	ctx, cancel := context.WithTimeout(ctx, timeoutBoot)
	defer cancel()
	err := atribuir(ctx,
		acesso.Usuario{ID: email, Nome: email},
		acesso.Atribuicao{PapelID: "admin_superlogica", Escopo: escopoSuperlogica})
	if err != nil {
		return fmt.Errorf("garantir admin inicial: %w", err)
	}
	return nil
}
