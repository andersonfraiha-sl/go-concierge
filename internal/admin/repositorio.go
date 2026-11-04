package admin

import (
	"context"
	"errors"
)

// ErrConflitoConcorrente: outra escrita simultânea venceu todas as tentativas; o pedido pode
// ser repetido (o Admin responde 503).
var ErrConflitoConcorrente = errors.New("conflito com outra escrita simultânea")

// Repositorio guarda o mapeamento. Toda escrita aplica a regra pura correspondente ao
// mapeamento atual e devolve o mapeamento inteiro depois de gravar; o que a regra recusa
// volta como ErrInvalido.
type Repositorio interface {
	Ler(ctx context.Context) (Mapeamento, error)
	// Salvar* criam ou atualizam (semente e importação); o Admin usa Criar* (ErrJaExiste) e
	// Editar* (ErrNaoEncontrado), que não confundem um caso com o outro.
	SalvarAdministradora(ctx context.Context, a Administradora) (Mapeamento, error)
	CriarAdministradora(ctx context.Context, a Administradora) (Mapeamento, error)
	EditarAdministradora(ctx context.Context, a Administradora) (Mapeamento, error)
	RemoverAdministradora(ctx context.Context, id string) (Mapeamento, error)
	SalvarCondominio(ctx context.Context, c Condominio) (Mapeamento, error)
	CriarCondominio(ctx context.Context, c Condominio) (Mapeamento, error)
	// EditarCondominio recusa com ErrConflitoConcorrente se a dona ou as chaves de roteamento
	// do condomínio não são mais as vistas na autorização.
	EditarCondominio(ctx context.Context, c Condominio, visto Chaves) (Mapeamento, error)
	RemoverCondominio(ctx context.Context, alvo Alvo) (Mapeamento, error)
	SalvarHabilitacao(ctx context.Context, alvo Alvo, caso string, h Habilitacao, limpar bool) (Mapeamento, error)
	SalvarOrientacao(ctx context.Context, alvo Alvo, o Orientacao) (Mapeamento, error)
	SalvarAjustes(ctx context.Context, alvo Alvo, ajustes map[string]map[string]any) (Mapeamento, error)
	SalvarPayload(ctx context.Context, alvo Alvo, recurso string, payload map[string]any) (Mapeamento, error)
}

// Alvo é o condomínio (ou o padrão "*") de uma escrita e a dona com que ela foi autorizada:
// se o condomínio mudou de dona no meio, a escrita é recusada com ErrConflitoConcorrente.
type Alvo struct{ Condominio, Dona string }

// regra é uma das transições puras de regras.go, já com os argumentos.
type regra func(Mapeamento) (Mapeamento, error)
