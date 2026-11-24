package canaiscore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"go-concierge/internal/entrada"
	comfalha "go-concierge/internal/falha"
)

// HTTPSender manda a resposta ao ATENDE por HTTP: POST {URL} com
// {"protocol": ..., "message": ...} e Bearer {Token}. O contrato real de resposta
// do ATENDE ainda não foi passado; este é o encaixe para quando vier.
type HTTPSender struct {
	URL   string
	Token string
	HTTP  *http.Client
}

func (h *HTTPSender) Enviar(ctx context.Context, destino entrada.Destino, texto string) error {
	raw, _ := json.Marshal(map[string]string{"protocol": destino.Protocolo, "session_id": destino.SessaoID, "message": texto})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.URL, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	query := req.URL.Query()
	if destino.CompanyID != "" {
		query.Set("company_id", destino.CompanyID)
	}
	if destino.ChannelID != uuid.Nil {
		query.Set("channel_id", destino.ChannelID.String())
	}
	req.URL.RawQuery = query.Encode()
	if h.Token != "" {
		req.Header.Set("Authorization", "Bearer "+h.Token)
	}
	cli := h.HTTP
	if cli == nil {
		cli = &http.Client{Timeout: 20 * time.Second}
	}
	res, err := cli.Do(req)
	if err != nil {
		return comfalha.ComPilha(err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return comfalha.ComPilha(fmt.Errorf("HTTP %d", res.StatusCode))
	}
	return nil
}
