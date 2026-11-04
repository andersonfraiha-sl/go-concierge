package atende

import (
	"context"
	"errors"

	"go-concierge/internal/entrada"
)

type Transferidor interface {
	Transferir(context.Context, entrada.Destino, string) error
}

type TransferenciaIncerta struct {
	Erro error
}

func (e *TransferenciaIncerta) Error() string {
	return "resultado da transferência incerto: " + e.Erro.Error()
}
func (e *TransferenciaIncerta) Unwrap() error { return e.Erro }

type TransferidorEmulado struct {
	Outbox *Outbox
}

func (e *TransferidorEmulado) Transferir(_ context.Context, destino entrada.Destino, motivo string) error {
	if e.Outbox != nil {
		e.Outbox.Transbordar(destino, motivo)
	}
	return nil
}

type TransferidoresPorConector struct {
	ATENDE  Transferidor
	Emulado Transferidor
}

func (t TransferidoresPorConector) Disponivel(destino entrada.Destino) bool {
	switch destino.Conector {
	case "atende":
		return t.ATENDE != nil
	case "atende-emulado":
		return t.Emulado != nil
	default:
		return false
	}
}

func (t TransferidoresPorConector) Transferir(ctx context.Context, destino entrada.Destino, motivo string) error {
	var transferidor Transferidor
	switch destino.Conector {
	case "atende":
		transferidor = t.ATENDE
	case "atende-emulado":
		transferidor = t.Emulado
	}
	if transferidor == nil {
		return errors.New("transferência indisponível para este canal")
	}
	return transferidor.Transferir(ctx, destino, motivo)
}
