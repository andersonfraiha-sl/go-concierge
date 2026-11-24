package canaiscore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go-concierge/internal/entrada"
	"net/http"
	"net/url"
	"strings"
	"time"
)

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
		return &entrada.TransferenciaIncerta{Erro: err}
	}
	defer res.Body.Close()
	if res.StatusCode >= 200 && res.StatusCode < 300 {
		return nil
	}
	err = fmt.Errorf("ATENDE respondeu HTTP %d", res.StatusCode)
	if res.StatusCode >= 500 {
		return &entrada.TransferenciaIncerta{Erro: err}
	}
	return err
}
