package atende

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

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

type ATENDETransferidor struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
}

func (a *ATENDETransferidor) Transferir(ctx context.Context, destino entrada.Destino, _ string) error {
	if strings.TrimSpace(a.BaseURL) == "" || strings.TrimSpace(a.APIKey) == "" {
		return errors.New("ATENDE: configuração de transferência incompleta")
	}
	if strings.TrimSpace(destino.Protocolo) == "" {
		return errors.New("ATENDE: protocolo vazio para transferência")
	}
	corpo, err := json.Marshal(map[string]any{})
	if err != nil {
		return err
	}
	endpoint := strings.TrimRight(a.BaseURL, "/") + "/protocols/" + url.PathEscape(destino.Protocolo) + "/transfer"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(corpo))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.APIKey)
	client := a.HTTP
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	} else {
		copia := *client
		copia.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
		if copia.Timeout <= 0 {
			copia.Timeout = 20 * time.Second
		}
		client = &copia
	}
	res, err := client.Do(req)
	if err != nil {
		return &TransferenciaIncerta{Erro: err}
	}
	defer res.Body.Close()
	if res.StatusCode >= 200 && res.StatusCode < 300 {
		return nil
	}
	err = fmt.Errorf("ATENDE respondeu HTTP %d", res.StatusCode)
	if res.StatusCode >= 500 {
		return &TransferenciaIncerta{Erro: err}
	}
	return err
}

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
