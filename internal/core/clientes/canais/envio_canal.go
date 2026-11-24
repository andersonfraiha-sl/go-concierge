package canaiscore

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"go-concierge/internal/entrada"
	"go-concierge/internal/harness/canais"
)

type LeitorCanalDeSaida interface {
	PorID(context.Context, string, uuid.UUID) (canais.Canal, error)
}

type SenderPorCanal struct{ Canais LeitorCanalDeSaida }

func (s SenderPorCanal) Enviar(ctx context.Context, d entrada.Destino, texto string) error {
	c, err := s.Canais.PorID(ctx, d.CompanyID, d.ChannelID)
	if err != nil {
		return err
	}
	if c.Provider != "ATENDE" || !c.Habilitado {
		return errors.New("envio indisponível para este canal")
	}
	return (&ATENDESender{BaseURL: c.APIBaseURL, APIKey: c.APIKey}).Enviar(ctx, d, texto)
}

type TransferidorPorCanal struct{ Canais LeitorCanalDeSaida }

func (t TransferidorPorCanal) Disponivel(d entrada.Destino) bool { return d.ChannelID != uuid.Nil }

func (t TransferidorPorCanal) Transferir(ctx context.Context, d entrada.Destino, motivo string) error {
	c, err := t.Canais.PorID(ctx, d.CompanyID, d.ChannelID)
	if err != nil {
		return err
	}
	if c.Provider != "ATENDE" || !c.Habilitado {
		return errors.New("transferência indisponível para este canal")
	}
	return (&ATENDETransferidor{BaseURL: c.APIBaseURL, APIKey: c.APIKey}).Transferir(ctx, d, motivo)
}
