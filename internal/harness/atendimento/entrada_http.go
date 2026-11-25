package atendimento

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"go-concierge/internal/entrada"
	"go-concierge/internal/falha"
)

type comandoEntrada struct {
	Operacao  string           `json:"operacao"`
	Mensagem  entrada.Mensagem `json:"mensagem"`
	RequestID string           `json:"request_id,omitempty"`
}

type respostaEntrada struct {
	Protocolo string `json:"protocolo,omitempty"`
	SessaoID  string `json:"sessao_id,omitempty"`
	Aplicado  bool   `json:"aplicado,omitempty"`
	Erro      string `json:"erro,omitempty"`
}

type ServidorEntrada struct {
	http   *http.Server
	turnos sync.WaitGroup
}

type ClienteEntrada struct {
	url   string
	token string
	http  *http.Client
}

type servicoEntrada interface {
	PermiteTelefone(string) bool
	PrepararMensagem(context.Context, entrada.Mensagem) (entrada.Mensagem, *Sessao, error)
	Processar(context.Context, entrada.Mensagem) (*Sessao, error)
	RegistrarHumano(context.Context, entrada.Mensagem) (bool, error)
	Encerrar(context.Context, entrada.Mensagem) (bool, error)
}

func IniciarEntradaHTTP(servico servicoEntrada) (*ServidorEntrada, *ClienteEntrada, error) {
	if servico == nil {
		return nil, nil, errors.New("serviço de atendimento ausente")
	}
	segredo := make([]byte, 32)
	if _, err := rand.Read(segredo); err != nil {
		return nil, nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(segredo)
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, nil, err
	}
	servidor := &ServidorEntrada{}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /interno/harness/v1/eventos", func(w http.ResponseWriter, r *http.Request) {
		if !hmac.Equal([]byte(r.Header.Get("Authorization")), []byte("Bearer "+token)) {
			http.Error(w, "não autorizado", http.StatusUnauthorized)
			return
		}
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20))
		dec.DisallowUnknownFields()
		var cmd comandoEntrada
		if err := dec.Decode(&cmd); err != nil {
			http.Error(w, "comando inválido", http.StatusBadRequest)
			return
		}
		var extra any
		if dec.Decode(&extra) != io.EOF {
			http.Error(w, "comando inválido", http.StatusBadRequest)
			return
		}
		resposta := respostaEntrada{}
		var err error
		status := http.StatusOK
		switch cmd.Operacao {
		case "iniciar":
			if !servico.PermiteTelefone(cmd.Mensagem.Contato.Phone) {
				status = http.StatusForbidden
				err = errors.New("telefone diferente do morador configurado para o token B2C")
				break
			}
			var msg entrada.Mensagem
			var sess *Sessao
			msg, sess, err = servico.PrepararMensagem(r.Context(), cmd.Mensagem)
			if err == nil {
				resposta.Protocolo, resposta.SessaoID = sess.Protocolo, sess.ID
				servidor.turnos.Add(1)
				go func() {
					defer servidor.turnos.Done()
					ctx, cancelar := context.WithTimeout(context.Background(), 150*time.Second)
					defer cancelar()
					defer falha.Recuperar(ctx, "turno_atende")
					if _, err := servico.Processar(ctx, msg); err != nil {
						falha.Registrar(ctx, "turno do concierge falhou", err, slog.String("operacao", "turno_atende"), slog.String("request_id", cmd.RequestID), slog.String("protocolo", msg.ConversaID))
					}
				}()
				status = http.StatusAccepted
			}
		case "humano":
			resposta.Aplicado, err = servico.RegistrarHumano(r.Context(), cmd.Mensagem)
		case "encerrar":
			resposta.Aplicado, err = servico.Encerrar(r.Context(), cmd.Mensagem)
		default:
			status = http.StatusBadRequest
			err = errors.New("operação inválida")
		}
		if err != nil {
			if status == http.StatusOK {
				status = http.StatusInternalServerError
			}
			resposta.Erro = err.Error()
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(resposta)
	})
	servidor.http = &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 30 * time.Second}
	go func() {
		if err := servidor.http.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("servidor do harness interrompido", "operacao", "servir_harness", "erro", err)
		}
	}()
	cliente := &ClienteEntrada{url: "http://" + ln.Addr().String() + "/interno/harness/v1/eventos", token: token, http: &http.Client{Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
	return servidor, cliente, nil
}

func (s *ServidorEntrada) Fechar(ctx context.Context) error {
	if err := s.http.Shutdown(ctx); err != nil {
		return err
	}
	feito := make(chan struct{})
	go func() { s.turnos.Wait(); close(feito) }()
	select {
	case <-feito:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *ClienteEntrada) Aplicar(ctx context.Context, operacao string, msg entrada.Mensagem, requestID string) (respostaEntrada, int, error) {
	var resultado respostaEntrada
	corpo, err := json.Marshal(comandoEntrada{Operacao: operacao, Mensagem: msg, RequestID: requestID})
	if err != nil {
		return resultado, 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(corpo))
	if err != nil {
		return resultado, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return resultado, 0, fmt.Errorf("harness: resultado do encaminhamento desconhecido: %w", err)
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&resultado); err != nil {
		return resultado, resp.StatusCode, fmt.Errorf("harness: resposta inválida: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resultado, resp.StatusCode, errors.New(resultado.Erro)
	}
	return resultado, resp.StatusCode, nil
}
