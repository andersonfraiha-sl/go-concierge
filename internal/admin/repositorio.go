package admin

import "context"

// Repositorio guarda o mapeamento. Toda escrita aplica a regra pura correspondente ao
// mapeamento atual e devolve o mapeamento inteiro depois de gravar; o que a regra recusa
// volta como ErrInvalido.
type Repositorio interface {
	Ler(ctx context.Context) (Mapeamento, error)
	SalvarAdministradora(ctx context.Context, a Administradora) (Mapeamento, error)
	RemoverAdministradora(ctx context.Context, id string) (Mapeamento, error)
	SalvarCondominio(ctx context.Context, c Condominio) (Mapeamento, error)
	RemoverCondominio(ctx context.Context, id string) (Mapeamento, error)
	SalvarHabilitacao(ctx context.Context, condoID, caso string, h Habilitacao, limpar bool) (Mapeamento, error)
	SalvarOrientacao(ctx context.Context, condoID string, o Orientacao) (Mapeamento, error)
	SalvarAjustes(ctx context.Context, condoID string, ajustes map[string]map[string]any) (Mapeamento, error)
	SalvarPayload(ctx context.Context, condoID, recurso string, payload map[string]any) (Mapeamento, error)
}

// regra é uma das transições puras de regras.go, já com os argumentos.
type regra func(Mapeamento) (Mapeamento, error)
