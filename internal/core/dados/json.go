package dados

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"time"
)

func requisitarJSON(ctx context.Context, cliente *http.Client, destino, token string, pedido, resposta any, limitePedido, limiteResposta int, prazo time.Duration) (int, bool, error) {
	alvo, err := url.Parse(destino)
	if err != nil || alvo.Scheme != "http" || alvo.Hostname() != "127.0.0.1" || alvo.Port() == "" || alvo.User != nil || alvo.RawQuery != "" || alvo.Fragment != "" {
		return 0, false, errors.New("destino da API privada inválido")
	}
	dados, err := json.Marshal(pedido)
	if err != nil {
		return 0, false, err
	}
	if len(dados) > limitePedido {
		return 0, false, errors.New("pedido excede o limite")
	}
	ctx, cancelar := context.WithTimeout(ctx, prazo)
	defer cancelar()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, destino, bytes.NewReader(dados))
	if err != nil {
		return 0, false, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	res, err := cliente.Do(req)
	if err != nil {
		return 0, true, err
	}
	defer res.Body.Close()
	corpo, err := io.ReadAll(io.LimitReader(res.Body, int64(limiteResposta)+1))
	if err != nil || len(corpo) > limiteResposta {
		return res.StatusCode, true, errors.New("resposta inválida ou excede o limite")
	}
	dec := json.NewDecoder(bytes.NewReader(corpo))
	dec.DisallowUnknownFields()
	if err := dec.Decode(resposta); err != nil {
		return res.StatusCode, true, err
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		return res.StatusCode, true, errors.New("resposta contém dados adicionais")
	}
	return res.StatusCode, true, nil
}
