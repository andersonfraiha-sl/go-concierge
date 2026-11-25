package atendimento

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"go-concierge/internal/entrada"
)

type ClienteSaida struct {
	URL   string
	Token string
	HTTP  *http.Client
	Prazo time.Duration
}

type saidaRecusada struct{ erro error }

func (e *saidaRecusada) Error() string { return e.erro.Error() }
func (e *saidaRecusada) Unwrap() error { return e.erro }

func (c *ClienteSaida) Disponivel(d entrada.Destino) bool {
	return d.Conector == "atende" || d.Conector == "atende-emulado"
}

func (c *ClienteSaida) Enviar(ctx context.Context, destino entrada.Destino, texto string) error {
	if ctx.Err() != nil || !entrada.AutorizarEntrega(ctx) {
		return entrada.ErrEntregaSuprimida
	}
	prazo := c.Prazo
	if prazo <= 0 {
		prazo = 120 * time.Second
	}
	ctx, cancelar := context.WithTimeout(context.WithoutCancel(ctx), prazo)
	defer cancelar()
	res, err := c.executar(ctx, entrada.ComandoSaida{Operacao: "enviar", Destino: destino, Texto: texto})
	if err != nil {
		var recusada *saidaRecusada
		if errors.As(err, &recusada) {
			return err
		}
		return &entrada.EntregaIncerta{Erro: err}
	}
	if res.Resultado == "enviada" {
		return nil
	}
	if res.Resultado == "incerta" {
		incerta := &entrada.EntregaIncerta{Erro: errors.New(res.Erro)}
		if len(res.Partes) != 0 {
			return &entrada.EntregaParcial{Partes: res.Partes, Erro: incerta}
		}
		return incerta
	}
	falha := errors.New(res.Erro)
	if len(res.Partes) != 0 {
		return &entrada.EntregaParcial{Partes: res.Partes, Erro: falha}
	}
	return falha
}

func (c *ClienteSaida) Transferir(ctx context.Context, destino entrada.Destino, motivo string) error {
	prazo := c.Prazo
	if prazo <= 0 {
		prazo = 120 * time.Second
	}
	ctx, cancelar := context.WithTimeout(ctx, prazo)
	defer cancelar()
	res, err := c.executar(ctx, entrada.ComandoSaida{Operacao: "transferir", Destino: destino, Motivo: motivo})
	if err != nil {
		var recusada *saidaRecusada
		if errors.As(err, &recusada) {
			return err
		}
		return &entrada.TransferenciaIncerta{Erro: err}
	}
	if res.Resultado == "transferida" {
		return nil
	}
	if res.Resultado == "incerta" {
		return &entrada.TransferenciaIncerta{Erro: errors.New(res.Erro)}
	}
	return errors.New(res.Erro)
}

func (c *ClienteSaida) executar(ctx context.Context, comando entrada.ComandoSaida) (entrada.ResultadoSaida, error) {
	var resultado entrada.ResultadoSaida
	corpo, err := json.Marshal(comando)
	if err != nil {
		return resultado, &saidaRecusada{erro: err}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL, bytes.NewReader(corpo))
	if err != nil {
		return resultado, &saidaRecusada{erro: err}
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	cliente := c.HTTP
	if cliente == nil {
		cliente = &http.Client{Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	res, err := cliente.Do(req)
	if err != nil {
		return resultado, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		if res.StatusCode == http.StatusBadRequest || res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden || res.StatusCode == http.StatusNotFound || res.StatusCode == http.StatusMethodNotAllowed {
			return resultado, &saidaRecusada{erro: fmt.Errorf("core recusou a operação: HTTP %d", res.StatusCode)}
		}
		return resultado, fmt.Errorf("core respondeu HTTP %d", res.StatusCode)
	}
	dec := json.NewDecoder(io.LimitReader(res.Body, 4<<20))
	if err := dec.Decode(&resultado); err != nil {
		return resultado, fmt.Errorf("resposta do core inválida: %w", err)
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		return resultado, errors.New("resposta do core inválida")
	}
	if resultado.Resultado == "falha" || resultado.Resultado == "incerta" {
		if resultado.Erro == "" {
			return resultado, errors.New("resultado do core sem erro")
		}
		return resultado, nil
	}
	if comando.Operacao == "enviar" && resultado.Resultado != "enviada" || comando.Operacao == "transferir" && resultado.Resultado != "transferida" || len(resultado.Partes) != 0 {
		return resultado, errors.New("resultado do core inválido")
	}
	return resultado, nil
}
