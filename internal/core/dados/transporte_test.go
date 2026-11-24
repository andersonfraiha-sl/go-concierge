package dados

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"go-concierge/internal/entrada"
	"go-concierge/internal/escopo"
	atende "go-concierge/internal/harness/atendimento"
	"go-concierge/internal/harness/ganchos/auditoria"
	"go-concierge/internal/harness/pedidos"
)

type gravadorFunc func(context.Context, atende.RegistroSessao, escopo.Contato, pedidos.Carteira, []auditoria.Linha) error

func (f gravadorFunc) Gravar(ctx context.Context, r atende.RegistroSessao, c escopo.Contato, cart pedidos.Carteira, linhas []auditoria.Linha) error {
	return f(ctx, r, c, cart, linhas)
}

func TestTransporteTurnoUsaHTTPAutenticado(t *testing.T) {
	var chamadas atomic.Int32
	gravador := gravadorFunc(func(_ context.Context, _ atende.RegistroSessao, c escopo.Contato, _ pedidos.Carteira, _ []auditoria.Linha) error {
		chamadas.Add(1)
		if c.CompanyID != "empresa" {
			t.Errorf("empresa = %q", c.CompanyID)
		}
		return nil
	})
	servidor, cliente, err := Iniciar(gravador)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = servidor.Fechar(context.Background()) })
	for _, token := range []string{"", "Bearer inválido"} {
		req, err := http.NewRequest(http.MethodPost, cliente.url, strings.NewReader(`{}`))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", token)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, res.Body)
		res.Body.Close()
		if res.StatusCode != http.StatusUnauthorized || chamadas.Load() != 0 {
			t.Fatalf("status=%d chamadas=%d", res.StatusCode, chamadas.Load())
		}
	}
	contato := escopo.Contato{CompanyID: "empresa", Telefone: "5511999999999"}
	if err := cliente.Gravar(t.Context(), atende.RegistroSessao{}, contato, pedidos.Carteira{}, nil); err != nil {
		t.Fatal(err)
	}
	if chamadas.Load() != 1 {
		t.Fatalf("chamadas = %d", chamadas.Load())
	}
}

func TestTransporteTurnoRejeicaoComprovada(t *testing.T) {
	servidor, cliente, err := Iniciar(gravadorFunc(func(context.Context, atende.RegistroSessao, escopo.Contato, pedidos.Carteira, []auditoria.Linha) error {
		return atende.ErrGravacaoNaoAplicada
	}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = servidor.Fechar(context.Background()) })
	contato := escopo.Contato{CompanyID: "empresa", Telefone: "5511999999999"}
	if err := cliente.Gravar(t.Context(), atende.RegistroSessao{}, contato, pedidos.Carteira{}, nil); !errors.Is(err, atende.ErrGravacaoNaoAplicada) {
		t.Fatalf("resultado = %v", err)
	}
}

func TestTransporteTurnoRecusaDeEscopo(t *testing.T) {
	servidor, cliente, err := Iniciar(gravadorFunc(func(context.Context, atende.RegistroSessao, escopo.Contato, pedidos.Carteira, []auditoria.Linha) error {
		return errors.Join(atende.ErrGravacaoNaoAplicada, atende.ErrGravacaoRecusada)
	}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = servidor.Fechar(context.Background()) })
	contato := escopo.Contato{CompanyID: "empresa", Telefone: "5511999999999"}
	err = cliente.Gravar(t.Context(), atende.RegistroSessao{}, contato, pedidos.Carteira{}, nil)
	if !errors.Is(err, atende.ErrGravacaoNaoAplicada) || !errors.Is(err, atende.ErrGravacaoRecusada) || errors.Is(err, atende.ErrGravacaoIncerta) {
		t.Fatalf("resultado = %v", err)
	}
}

func TestTransporteTurnoRespostaPerdidaNaoRepete(t *testing.T) {
	var aplicadas atomic.Int32
	operacoes := make(chan string, 1)
	servidor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var cmd comandoTurno
		if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
			t.Error(err)
		}
		operacoes <- cmd.Operacao
		if r.Header.Get("Idempotency-Key") != "" {
			t.Error("identificador não deve prometer idempotência")
		}
		aplicadas.Add(1)
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		_ = conn.Close()
	}))
	defer servidor.Close()
	cliente := &Cliente{url: servidor.URL, token: "teste", http: servidor.Client()}
	contato := escopo.Contato{CompanyID: "empresa", Telefone: "5511999999999"}
	err := cliente.Gravar(t.Context(), atende.RegistroSessao{}, contato, pedidos.Carteira{}, nil)
	operacao := <-operacoes
	if !errors.Is(err, atende.ErrGravacaoIncerta) || errors.Is(err, atende.ErrGravacaoNaoAplicada) || aplicadas.Load() != 1 || operacao == "" || !strings.Contains(err.Error(), operacao) || strings.Contains(err.Error(), cliente.token) {
		t.Fatalf("resultado=%v aplicações=%d", err, aplicadas.Load())
	}
}

func TestTransporteTurnoLimitaPrazoEmContextoSemPrazo(t *testing.T) {
	var chamadas atomic.Int32
	liberar := make(chan struct{})
	servidor := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		chamadas.Add(1)
		<-liberar
	}))
	defer servidor.Close()
	defer close(liberar)
	cliente := &Cliente{url: servidor.URL, token: "teste", http: servidor.Client()}
	contato := escopo.Contato{CompanyID: "empresa", Telefone: "5511999999999"}
	inicio := time.Now()
	err := cliente.Gravar(context.Background(), atende.RegistroSessao{}, contato, pedidos.Carteira{}, nil)
	if !errors.Is(err, atende.ErrGravacaoIncerta) || chamadas.Load() != 1 || time.Since(inicio) > 5*time.Second {
		t.Fatalf("resultado=%v chamadas=%d duração=%s", err, chamadas.Load(), time.Since(inicio))
	}
}

func TestServidorDeDadosNaoFicaProntoAposFechar(t *testing.T) {
	servidor, _, err := Iniciar(gravadorFunc(func(context.Context, atende.RegistroSessao, escopo.Contato, pedidos.Carteira, []auditoria.Linha) error {
		return nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !servidor.Pronto() {
		t.Fatal("servidor não iniciou pronto")
	}
	if err := servidor.Fechar(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-servidor.fim:
	case <-time.After(time.Second):
		t.Fatal("servidor não parou")
	}
	if servidor.Pronto() {
		t.Fatal("servidor fechado continuou pronto")
	}
}

func registroValido(t *testing.T) (atende.RegistroSessao, escopo.Contato, pedidos.Carteira) {
	t.Helper()
	condo := 42
	contato := escopo.Contato{CompanyID: "empresa", CondoID: &condo, Telefone: "5511999999999"}
	r := atende.RegistroSessao{ID: "sessao", CompanyID: contato.CompanyID, ChannelID: uuid.New(), CondoID: &condo, Connector: "web", Protocolo: "protocolo", UserPhone: contato.Telefone, AtualizadaEm: time.Now().UTC().Truncate(time.Microsecond)}
	dados, err := json.Marshal(map[string]any{"id": r.ID, "company_id": r.CompanyID, "channel_id": r.ChannelID, "condo_id": r.CondoID, "protocolo": r.Protocolo, "contato": entrada.Contato{Phone: r.UserPhone}, "origem": entrada.Origem{Conector: r.Connector}, "escopo": contato, "atualizada_em": r.AtualizadaEm})
	if err != nil {
		t.Fatal(err)
	}
	r.Dados = dados
	cart := pedidos.Carteira{CompanyID: contato.CompanyID, Telefone: contato.Telefone}
	return r, contato, cart
}

func TestValidarTurnoRecusaEscoposDiferentes(t *testing.T) {
	r, contato, cart := registroValido(t)
	if err := validarTurno(r, contato, cart, nil); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		nome   string
		r      atende.RegistroSessao
		c      escopo.Contato
		cart   pedidos.Carteira
		linhas []auditoria.Linha
	}{
		{"outra empresa", r, escopo.Contato{CompanyID: "outra", Telefone: contato.Telefone, CondoID: contato.CondoID}, cart, nil},
		{"outra carteira", r, contato, pedidos.Carteira{CompanyID: "outra", Telefone: contato.Telefone}, nil},
		{"auditoria de outro canal", r, contato, cart, []auditoria.Linha{{ChannelID: uuid.NewString()}}},
		{"auditoria de outra sessão", r, contato, cart, []auditoria.Linha{{SessaoID: "outra"}}},
		{"pedido ausente", r, contato, pedidos.Carteira{CompanyID: contato.CompanyID, Telefone: contato.Telefone, Pedidos: []*pedidos.Pedido{nil}}, nil},
	} {
		t.Run(tc.nome, func(t *testing.T) {
			if err := validarTurno(tc.r, tc.c, tc.cart, tc.linhas); err == nil {
				t.Fatal("escopo deveria ser recusado")
			}
		})
	}
}
