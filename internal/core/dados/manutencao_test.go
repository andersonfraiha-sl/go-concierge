package dados

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"

	atende "go-concierge/internal/harness/atendimento"
)

type retencaoTeste struct{ chamadas atomic.Int32 }

func (r *retencaoTeste) ReterAuditoria(context.Context) (int, error) {
	r.chamadas.Add(1)
	return 7, nil
}

func TestRetencaoHTTPReservaCapacidadeENaoAceitaCorteDoCliente(t *testing.T) {
	store := &retencaoTeste{}
	servidor, cliente, _, err := IniciarConfigurado(Config{Turno: gravadorFunc(nil), Manutencao: store})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = servidor.Fechar(context.Background()) })
	manutencao := servidor.Manutencao()
	for _, tc := range []struct {
		nome, token, extra string
		status             int
	}{
		{"token do runtime", cliente.token, "", http.StatusUnauthorized},
		{"corte enviado pelo cliente", manutencao.token, `,"agora":"2999-01-01T00:00:00Z"`, http.StatusBadRequest},
	} {
		t.Run(tc.nome, func(t *testing.T) {
			req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, cliente.base+rotaReterAuditoria, strings.NewReader(`{"operacao":"`+uuid.NewString()+`"`+tc.extra+`}`))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Authorization", "Bearer "+tc.token)
			res, err := cliente.http.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			if res.StatusCode != tc.status {
				t.Fatalf("HTTP %d, esperado %d", res.StatusCode, tc.status)
			}
		})
	}
	if store.chamadas.Load() != 0 {
		t.Fatal("requisição sem capacidade válida executou retenção")
	}
	if n, err := manutencao.ReterAuditoria(t.Context()); err != nil || n != 7 || store.chamadas.Load() != 1 {
		t.Fatalf("retenção autorizada: n=%d erro=%v chamadas=%d", n, err, store.chamadas.Load())
	}
}

func TestRetencaoHTTPRespostaPerdidaNaoRepete(t *testing.T) {
	var chamadas atomic.Int32
	servidor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		chamadas.Add(1)
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		_ = conn.Close()
	}))
	defer servidor.Close()
	cliente := &Manutencao{base: servidor.URL, token: "teste", http: servidor.Client()}
	_, err := cliente.ReterAuditoria(t.Context())
	if !errors.Is(err, atende.ErrGravacaoIncerta) || errors.Is(err, atende.ErrGravacaoNaoAplicada) || chamadas.Load() != 1 {
		t.Fatalf("resultado=%v chamadas=%d", err, chamadas.Load())
	}
}
