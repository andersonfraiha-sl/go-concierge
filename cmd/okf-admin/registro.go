package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"net/url"
	"runtime/debug"
	"slices"
	"strings"
	"time"

	"go-concierge/internal/escopo"
	"go-concierge/internal/falha"
)

// logger é o slog do servidor; sem um configurado (testes antigos), o padrão do processo.
func (s *servidor) logger() *slog.Logger {
	if s.registro != nil {
		return s.registro
	}
	return slog.Default()
}

// registroRequisicao junta o que a requisição descobre no caminho (o usuário, depois de
// autenticar) para a linha de log que sai no fim dela, e leva o logger do servidor a quem não
// é método dele (exigir).
type registroRequisicao struct {
	usuarioID string
	logger    *slog.Logger
}

type chaveRegistro struct{}

func anotarUsuario(ctx context.Context, usuarioID string) {
	if reg, ok := ctx.Value(chaveRegistro{}).(*registroRequisicao); ok {
		reg.usuarioID = usuarioID
	}
}

// respostaRegistrada guarda o status e a falha que o handler anotou; Unwrap deixa o
// http.ResponseController achar o Flush (o proxy do MCP responde em SSE).
type respostaRegistrada struct {
	http.ResponseWriter
	status   int
	escreveu bool
	falha    *falhaDaRequisicao
}

// falhaDaRequisicao é o que vai na linha de ERROR: o erro, a pilha (a capturada por
// falha.ComPilha ou a do pânico) e quem respondeu o erro.
type falhaDaRequisicao struct {
	erro     string
	pilha    string
	chamador string
}

func (rr *respostaRegistrada) WriteHeader(status int) {
	rr.status, rr.escreveu = status, true
	rr.ResponseWriter.WriteHeader(status)
}

func (rr *respostaRegistrada) Write(b []byte) (int, error) {
	rr.escreveu = true
	return rr.ResponseWriter.Write(b)
}

func (rr *respostaRegistrada) Unwrap() http.ResponseWriter { return rr.ResponseWriter }

// recuperar troca o pânico do handler por 500 com o JSON de erro do projeto; a pilha vai na
// linha da requisição. ErrAbortHandler é o jeito do net/http de abortar e segue adiante.
func (rr *respostaRegistrada) recuperar(p any) {
	if err, ok := p.(error); ok && errors.Is(err, http.ErrAbortHandler) {
		panic(p)
	}
	rr.falha = &falhaDaRequisicao{erro: fmt.Sprint("pânico: ", p), pilha: string(debug.Stack())}
	if !rr.escreveu {
		writeJSON(rr, http.StatusInternalServerError, erroAPI{Codigo: "ERRO_INTERNO", Mensagem: "Não deu para concluir. Tente de novo."})
	}
	rr.status = http.StatusInternalServerError
}

// anotarFalha guarda err para a linha de ERROR da requisição (uma por falha, com
// request_id e usuário); fora do registrar (teste de handler solto), loga direto.
// pular conta os quadros entre quem respondeu o erro e anotarFalha.
func anotarFalha(ctx context.Context, w http.ResponseWriter, err error, pular int) {
	f := &falhaDaRequisicao{erro: err.Error(), pilha: falha.Pilha(err), chamador: falha.Chamador(pular + 1)}
	if rr, ok := registradaEm(w); ok {
		rr.falha = f
		return
	}
	atributos := []slog.Attr{slog.String("erro", f.erro), slog.String("chamador", f.chamador)}
	if f.pilha != "" {
		atributos = append(atributos, slog.String("stack", f.pilha))
	}
	slog.Default().LogAttrs(ctx, slog.LevelError, "requisição falhou", atributos...)
}

func registradaEm(w http.ResponseWriter) (*respostaRegistrada, bool) {
	for {
		if rr, ok := w.(*respostaRegistrada); ok {
			return rr, true
		}
		embrulho, ok := w.(interface{ Unwrap() http.ResponseWriter })
		if !ok {
			return nil, false
		}
		w = embrulho.Unwrap()
	}
}

// registrar dá um request_id a cada requisição (devolvido em X-Request-Id) e escreve uma
// linha de log no fim, com usuário, operação (o padrão da rota), status e duração. Os
// arquivos estáticos e as sondas de saúde não geram linha; um /api/ que cai no padrão dos
// estáticos (rota inexistente) gera. Falha anotada ou pânico sobem a linha para ERROR, com
// o erro e a pilha: uma linha por falha.
func (s *servidor) registrar(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inicio := time.Now()
		requestID := novoRequestID()
		w.Header().Set("X-Request-Id", requestID)
		reg := &registroRequisicao{logger: s.logger()}
		r = r.WithContext(context.WithValue(r.Context(), chaveRegistro{}, reg))
		rr := &respostaRegistrada{ResponseWriter: w, status: http.StatusOK}
		defer func() {
			if p := recover(); p != nil {
				rr.recuperar(p)
			}
			s.escreverLinha(r, rr, reg.usuarioID, requestID, time.Since(inicio))
		}()
		next.ServeHTTP(rr, r)
	})
}

func (s *servidor) escreverLinha(r *http.Request, rr *respostaRegistrada, usuarioID, requestID string, duracao time.Duration) {
	semLinha := ehSondaDeSaude(r.Pattern) || r.Pattern == padraoEstaticos && !strings.HasPrefix(r.URL.Path, "/api/")
	if semLinha && rr.falha == nil {
		return
	}
	atributos := []slog.Attr{
		slog.String("request_id", requestID),
		slog.String("usuario_id", usuarioID),
		slog.String("metodo", r.Method),
		slog.String("operacao", operacaoDe(r)),
		slog.String("caminho", caminhoMascarado(r.URL)),
		slog.Int("status", rr.status),
		slog.Int64("duracao_ms", duracao.Milliseconds()),
	}
	nivel, mensagem := slog.LevelInfo, "requisição"
	if rr.status >= http.StatusInternalServerError {
		nivel = slog.LevelWarn
	}
	if f := rr.falha; f != nil {
		nivel, mensagem = slog.LevelError, "requisição falhou"
		atributos = append(atributos, slog.String("erro", f.erro))
		if f.chamador != "" {
			atributos = append(atributos, slog.String("chamador", f.chamador))
		}
		if f.pilha != "" {
			atributos = append(atributos, slog.String("stack", f.pilha))
		}
	}
	s.logger().LogAttrs(r.Context(), nivel, mensagem, atributos...)
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
