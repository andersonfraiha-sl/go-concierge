package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func statusDoCorpo(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("Content-Type = %q, quer JSON", ct)
	}
	var corpo struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &corpo); err != nil {
		t.Fatalf("corpo não é JSON: %v: %s", err, w.Body.String())
	}
	return corpo.Status
}

func TestHealthzSemAutenticacaoESemLog(t *testing.T) {
	amb := novoAmbienteRotas(t)
	var buf bytes.Buffer
	amb.s.registro = slog.New(slog.NewJSONHandler(&buf, nil))
	amb.handler = amb.s.rotear(nil)

	w := amb.pedir(t, "", "GET", "/healthz", "")
	if w.Code != http.StatusOK || statusDoCorpo(t, w) != "ok" {
		t.Fatalf("healthz: %d %s", w.Code, w.Body.String())
	}
	if w := amb.pedir(t, "", "GET", "/readyz", ""); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("readyz sem regras aquecidas: %d, quer 503", w.Code)
	}
	if buf.Len() != 0 {
		t.Fatalf("sonda de saúde não entra no log de requisições: %s", buf.String())
	}
}

func TestReadyz(t *testing.T) {
	s, _, _ := servidorComGanchos(t)
	semBanco, err := pgxpool.New(t.Context(), "postgres://x:x@127.0.0.1:1/x?connect_timeout=1")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(semBanco.Close)

	casos := []struct {
		nome       string
		preparar   func(*servidor)
		quer       int
		querStatus string
	}{
		{"pronto", func(*servidor) {}, http.StatusOK, "pronto"},
		{"regras frias", func(s *servidor) { s.regrasAquecidas = func() bool { return false } }, http.StatusServiceUnavailable, "indisponivel"},
		{"postgres fora", func(s *servidor) { s.banco.Pool = semBanco }, http.StatusServiceUnavailable, "indisponivel"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			sc := &servidor{banco: s.banco, regrasAquecidas: s.regrasAquecidas}
			c.preparar(sc)
			w := httptest.NewRecorder()
			inicio := time.Now()
			sc.readyz(w, httptest.NewRequest("GET", "/readyz", nil))
			if w.Code != c.quer || statusDoCorpo(t, w) != c.querStatus {
				t.Fatalf("readyz: %d %s", w.Code, w.Body.String())
			}
			if d := time.Since(inicio); d > 3*time.Second {
				t.Fatalf("readyz demorou %s; o ping tem prazo de %s", d, timeoutHealthz)
			}
		})
	}
}

func TestSubcomandoSaude(t *testing.T) {
	pronto := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer pronto.Close()
	fora := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }))
	defer fora.Close()
	var erro bytes.Buffer
	if got := saude([]string{"-url", pronto.URL}, &erro); got != 0 {
		t.Fatalf("saude com 200 = %d: %s", got, erro.String())
	}
	if got := saude([]string{"-url", fora.URL}, &erro); got != 1 {
		t.Fatalf("saude com 503 = %d, quer 1", got)
	}
}
