package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"
)

const (
	prazoLeituraCabecalho = 10 * time.Second
	prazoLeitura          = 60 * time.Second  // o importar recebe zip de até 100 MB
	prazoEscrita          = 70 * time.Second  // acima dos 55 s do Copiloto, a requisição mais longa
	prazoOcioso           = 120 * time.Second // keep-alive entre requisições
	desligamentoPadrao    = 30 * time.Second  // cobre um turno do concierge em andamento
)

// novoServidorHTTP aplica os prazos contra cliente lento (slowloris) e resposta pendurada.
// Quem precisa escrever por mais tempo (o repasse SSE do MCP) libera o próprio prazo.
func novoServidorHTTP(handler http.Handler) *http.Server {
	return &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: prazoLeituraCabecalho,
		ReadTimeout:       prazoLeitura,
		WriteTimeout:      prazoEscrita,
		IdleTimeout:       prazoOcioso,
		ErrorLog:          slog.NewLogLogger(slog.Default().Handler(), slog.LevelWarn),
	}
}

// desligamento diz quanto esperar no sinal e o que esperar além das requisições: os
// trabalhos de fundo (turnos do webhook, extração de memória, retenção da auditoria).
type desligamento struct {
	prazo    time.Duration
	aguardar func()
}

// prazoDeDesligamento lê DESLIGAMENTO_PRAZO (duração do Go, ex.: 45s); vazio é o padrão.
func prazoDeDesligamento(valor string) (time.Duration, error) {
	if valor == "" {
		return desligamentoPadrao, nil
	}
	d, err := time.ParseDuration(valor)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("DESLIGAMENTO_PRAZO inválido %q: use uma duração positiva, ex.: 30s", valor)
	}
	return d, nil
}

// servir atende em ln até ctx cancelar. Aí para de aceitar conexões, deixa as requisições em
// andamento terminarem e espera os trabalhos de fundo, tudo dentro de d.prazo.
func servir(ctx context.Context, srv *http.Server, ln net.Listener, d desligamento) error {
	parou := make(chan error, 1)
	go func() { parou <- srv.Serve(ln) }()
	select {
	case err := <-parou:
		return fmt.Errorf("servir http: %w", err)
	case <-ctx.Done():
	}
	ctxFim, cancelar := context.WithTimeout(context.Background(), d.prazo)
	defer cancelar()
	if err := srv.Shutdown(ctxFim); err != nil {
		return fmt.Errorf("desligar http: %w", err)
	}
	return esperar(ctxFim, d.aguardar)
}

func esperar(ctx context.Context, aguardar func()) error {
	if aguardar == nil {
		return nil
	}
	feito := make(chan struct{})
	go func() {
		aguardar()
		close(feito)
	}()
	select {
	case <-feito:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("trabalhos de fundo não terminaram no prazo: %w", ctx.Err())
	}
}

// semPrazoDeEscrita libera o WriteTimeout do servidor para respostas em fluxo (SSE).
func semPrazoDeEscrita(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := http.NewResponseController(w).SetWriteDeadline(time.Time{}); err != nil {
			slog.WarnContext(r.Context(), "prazo de escrita não liberado", "operacao", "sem_prazo_escrita", "erro", err)
		}
		h.ServeHTTP(w, r)
	})
}

// encerrarComErro sai com código 1 quando o desligamento falha (prazo estourado, porta ocupada).
func encerrarComErro(err error) {
	slog.Error("okf-admin: encerrado com erro", "operacao", "desligar", "erro", err)
	os.Exit(1)
}
