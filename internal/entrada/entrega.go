package entrada

import (
	"context"
	"errors"
)

type ChaveControleEntrega struct{}

var ErrEntregaSuprimida = errors.New("entrega suprimida após mudança de atendimento")

type EntregaParcial struct {
	Partes []string
	Erro   error
}

func (e *EntregaParcial) Error() string { return e.Erro.Error() }
func (e *EntregaParcial) Unwrap() error { return e.Erro }

func AutorizarEntrega(ctx context.Context) bool {
	verificar, ok := ctx.Value(ChaveControleEntrega{}).(func() bool)
	return !ok || verificar()
}

type TransferenciaIncerta struct{ Erro error }

func (e *TransferenciaIncerta) Error() string {
	return "resultado da transferência incerto: " + e.Erro.Error()
}
func (e *TransferenciaIncerta) Unwrap() error { return e.Erro }
