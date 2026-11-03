package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"go-concierge/internal/escopo"
)

// logger é o slog do servidor; sem um configurado (testes antigos), o padrão do processo.
func (s *servidor) logger() *slog.Logger {
	if s.registro != nil {
		return s.registro
	}
	return slog.Default()
}

// registroRequisicao junta o que a requisição descobre no caminho (o usuário, depois de
// autenticar) para a linha de log que sai no fim dela.
type registroRequisicao struct{ usuarioID string }

type chaveRegistro struct{}

func anotarUsuario(ctx context.Context, usuarioID string) {
	if reg, ok := ctx.Value(chaveRegistro{}).(*registroRequisicao); ok {
		reg.usuarioID = usuarioID
	}
}

// respostaRegistrada guarda o status; Unwrap deixa o http.ResponseController achar o Flush
// (o proxy do MCP responde em SSE).
type respostaRegistrada struct {
	http.ResponseWriter
	status int
}

func (rr *respostaRegistrada) WriteHeader(status int) {
	rr.status = status
	rr.ResponseWriter.WriteHeader(status)
}

func (rr *respostaRegistrada) Unwrap() http.ResponseWriter { return rr.ResponseWriter }

// registrar dá um request_id a cada requisição (devolvido em X-Request-Id) e escreve uma
// linha de log no fim, com usuário, operação (o padrão da rota), status e duração. Os
// arquivos estáticos não geram linha.
func (s *servidor) registrar(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inicio := time.Now()
		requestID := novoRequestID()
		w.Header().Set("X-Request-Id", requestID)
		reg := &registroRequisicao{}
		r = r.WithContext(context.WithValue(r.Context(), chaveRegistro{}, reg))
		rr := &respostaRegistrada{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rr, r)
		if r.Pattern == padraoEstaticos {
			return
		}
		nivel := slog.LevelInfo
		if rr.status >= http.StatusInternalServerError {
			nivel = slog.LevelWarn
		}
		s.logger().LogAttrs(r.Context(), nivel, "requisição",
			slog.String("request_id", requestID),
			slog.String("usuario_id", reg.usuarioID),
			slog.String("operacao", operacaoDe(r)),
			slog.String("caminho", caminhoMascarado(r.URL)),
			slog.Int("status", rr.status),
			slog.Int64("duracao_ms", time.Since(inicio).Milliseconds()),
		)
	})
}

func operacaoDe(r *http.Request) string {
	if r.Pattern != "" {
		return r.Pattern
	}
	return r.Method + " " + r.URL.Path
}

// parametrosDeTelefone são os da query que levam telefone de morador (PII): vão mascarados.
var parametrosDeTelefone = []string{"phone", "telefone"}

// caminhoMascarado é o caminho com a query legível (sem percent-encoding: o handler JSON já
// escapa o que precisa) e os telefones mascarados.
func caminhoMascarado(u *url.URL) string {
	q := u.Query()
	if len(q) == 0 {
		return u.Path
	}
	var pares []string
	for _, k := range slices.Sorted(maps.Keys(q)) {
		for _, v := range q[k] {
			if slices.Contains(parametrosDeTelefone, k) {
				v = escopo.MascararTelefone(v)
			}
			pares = append(pares, k+"="+v)
		}
	}
	return u.Path + "?" + strings.Join(pares, "&")
}

func novoRequestID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b) // crypto/rand.Read nunca falha (Go 1.24+)
	return hex.EncodeToString(b)
}
