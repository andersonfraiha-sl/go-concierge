package banco

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
)

type commitTeste struct{ err error }

func (c commitTeste) Commit(context.Context) error { return c.err }

func TestResultadoCommitDoLote(t *testing.T) {
	perda := errors.New("conexão perdida")
	for _, tc := range []struct {
		nome    string
		err     error
		incerto bool
	}{
		{"aplicado", nil, false},
		{"rollback confirmado", pgx.ErrTxCommitRollback, false},
		{"resposta perdida", perda, true},
	} {
		t.Run(tc.nome, func(t *testing.T) {
			err := confirmarLote(t.Context(), commitTeste{tc.err})
			if !errors.Is(err, tc.err) || errors.Is(err, ErrCommitIncerto) != tc.incerto {
				t.Fatalf("resultado = %v", err)
			}
		})
	}
}
