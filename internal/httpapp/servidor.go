package httpapp

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go-concierge/internal/env"
)

type Config struct {
	ConciergeURL string
	BackdoorURL  string
	ConsoleURL   string
}

func Executar(nome, endereco string, novo func(Config) (http.Handler, error)) {
	if len(os.Args) > 1 && os.Args[1] == "saude" {
		os.Exit(saude(os.Args[2:]))
	}
	if err := executar(nome, endereco, novo); err != nil {
		log.Fatal(err)
	}
}

func executar(nome, endereco string, novo func(Config) (http.Handler, error)) error {
	addr := flag.String("addr", endereco, "endereço HTTP")
	dotenv := flag.String("env", "apps/"+nome+"/.env", "arquivo de configuração da aplicação")
	flag.Parse()
	env.Carregar(*dotenv)
	handler, err := novo(Config{
		ConciergeURL: env.Ou("CONCIERGE_URL", "http://127.0.0.1:8090"),
		BackdoorURL:  env.Ou("BACKDOOR_URL", "http://127.0.0.1:8091"),
		ConsoleURL:   env.Ou("CONSOLE_URL", "http://127.0.0.1:8092"),
	})
	if err != nil {
		return err
	}
	srv := &http.Server{Addr: *addr, Handler: Cabecalhos(handler), ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 60 * time.Second, WriteTimeout: 135 * time.Second, IdleTimeout: 120 * time.Second}
	ctx, cancelar := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelar()
	parou := make(chan error, 1)
	go func() { parou <- srv.ListenAndServe() }()
	log.Printf("%s: http://%s", nome, *addr)
	select {
	case err := <-parou:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		fim, encerrar := context.WithTimeout(context.Background(), 30*time.Second)
		defer encerrar()
		if err := srv.Shutdown(fim); err != nil {
			return err
		}
	}
	return nil
}

func Saude(mux *http.ServeMux, concierge string) {
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		JSON(w, http.StatusOK, map[string]string{"status": "vivo"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancelar := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancelar()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, concierge+"/readyz", nil)
		if err != nil {
			JSON(w, http.StatusServiceUnavailable, map[string]string{"status": "indisponivel"})
			return
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			JSON(w, http.StatusServiceUnavailable, map[string]string{"status": "indisponivel"})
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			JSON(w, http.StatusServiceUnavailable, map[string]string{"status": "indisponivel"})
			return
		}
		JSON(w, http.StatusOK, map[string]string{"status": "pronto"})
	})
}

func saude(args []string) int {
	f := flag.NewFlagSet("saude", flag.ContinueOnError)
	alvo := f.String("url", "", "URL de readiness")
	if err := f.Parse(args); err != nil {
		return 1
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(*alvo)
	if err != nil {
		return 1
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "aplicação indisponível")
		return 1
	}
	return 0
}
