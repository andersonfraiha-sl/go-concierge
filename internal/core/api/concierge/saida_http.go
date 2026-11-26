package conciergeapi

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"go-concierge/internal/entrada"
	atende "go-concierge/internal/harness/atendimento"
)

type servidorSaida struct{ http *http.Server }

func iniciarSaidaHTTP(sender atende.Sender, transferidor atende.Transferidor) (*servidorSaida, *atende.ClienteSaida, error) {
	if sender == nil || transferidor == nil {
		return nil, nil, errors.New("serviços de saída ausentes")
	}
	segredo := make([]byte, 32)
	if _, err := rand.Read(segredo); err != nil {
		return nil, nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(segredo)
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, nil, err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /interno/core/v1/saida", func(w http.ResponseWriter, r *http.Request) {
		if !hmac.Equal([]byte(r.Header.Get("Authorization")), []byte("Bearer "+token)) {
			http.Error(w, "não autorizado", http.StatusUnauthorized)
			return
		}
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20))
		dec.DisallowUnknownFields()
		var cmd entrada.ComandoSaida
		if dec.Decode(&cmd) != nil {
			http.Error(w, "comando inválido", http.StatusBadRequest)
			return
		}
		var extra any
		if dec.Decode(&extra) != io.EOF || !destinoSaidaValido(cmd.Destino) || (cmd.Operacao != "enviar" && cmd.Operacao != "transferir") {
			http.Error(w, "comando inválido", http.StatusBadRequest)
			return
		}
		ctx, cancelar := context.WithTimeout(context.WithoutCancel(r.Context()), 120*time.Second)
		defer cancelar()
		resultado := entrada.ResultadoSaida{}
		if cmd.Operacao == "enviar" {
			err := sender.Enviar(ctx, cmd.Destino, cmd.Texto)
			if err == nil {
				resultado.Resultado = "enviada"
			} else {
				resultado.Resultado, resultado.Erro = "falha", err.Error()
				var parcial *entrada.EntregaParcial
				if errors.As(err, &parcial) {
					resultado.Partes = append([]string(nil), parcial.Partes...)
				}
			}
		} else {
			var err error
			if disponivel, ok := transferidor.(interface{ Disponivel(entrada.Destino) bool }); ok && !disponivel.Disponivel(cmd.Destino) {
				err = errors.New("transferência indisponível para este canal")
			} else {
				err = transferidor.Transferir(ctx, cmd.Destino, cmd.Motivo)
			}
			if err == nil {
				resultado.Resultado = "transferida"
			} else {
				resultado.Resultado, resultado.Erro = "falha", err.Error()
				var incerta *entrada.TransferenciaIncerta
				if errors.As(err, &incerta) {
					resultado.Resultado = "incerta"
				}
			}
			if cmd.Destino.Conector == "atende" {
				if outbox, ok := sender.(*atende.Outbox); ok {
					outbox.RegistrarTransbordo(cmd.Destino, cmd.Motivo, err == nil, resultado.Erro)
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resultado)
	})
	servidor := &servidorSaida{http: &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 125 * time.Second, IdleTimeout: 30 * time.Second}}
	go func() {
		if err := servidor.http.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("servidor de saída interrompido", "operacao", "servir_saida", "erro", err)
		}
	}()
	cliente := &atende.ClienteSaida{URL: "http://" + listener.Addr().String() + "/interno/core/v1/saida", Token: token, HTTP: &http.Client{Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
	return servidor, cliente, nil
}

func destinoSaidaValido(d entrada.Destino) bool {
	if strings.TrimSpace(d.Protocolo) == "" || strings.TrimSpace(d.Telefone) == "" {
		return false
	}
	switch d.Conector {
	case "atende":
		return d.ChannelID != uuid.Nil && strings.TrimSpace(d.CompanyID) != ""
	case "atende-emulado":
		return true
	default:
		return false
	}
}

func (s *servidorSaida) Fechar(ctx context.Context) error { return s.http.Shutdown(ctx) }
