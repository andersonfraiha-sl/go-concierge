package dados

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"

	"go-concierge/internal/escopo"
	atende "go-concierge/internal/harness/atendimento"
	"go-concierge/internal/harness/canais"
	"go-concierge/internal/harness/ganchos/auditoria"
	"go-concierge/internal/harness/pedidos"
)

type repositorioCanalTeste struct {
	canal    canais.Canal
	casos    []canais.Caso
	errCasos error
	chamadas atomic.Int32
}

func (r *repositorioCanalTeste) Resolver(_ context.Context, id canais.Identidade) (canais.Canal, error) {
	r.chamadas.Add(1)
	if id.Provider != r.canal.Provider || id.Tipo != r.canal.Tipo || id.Tenant != r.canal.Tenant || id.Endpoint != r.canal.Endpoint {
		return canais.Canal{}, canais.ErrDesconhecido
	}
	return r.canal, nil
}

func (r *repositorioCanalTeste) PorID(_ context.Context, company string, id uuid.UUID) (canais.Canal, error) {
	r.chamadas.Add(1)
	if company != r.canal.CompanyID || id != r.canal.ID {
		return canais.Canal{}, canais.ErrDesconhecido
	}
	return r.canal, nil
}

func (r *repositorioCanalTeste) Casos(context.Context, canais.Canal) ([]canais.Caso, error) {
	r.chamadas.Add(1)
	return r.casos, r.errCasos
}

func TestCanaisHTTPComCredenciaisReservadas(t *testing.T) {
	repo := &repositorioCanalTeste{canal: canais.Canal{ID: uuid.New(), CompanyID: "empresa", Provider: "ATENDE", Tipo: "WHATSAPP", Tenant: "tenant", Endpoint: "55119000", Habilitado: true, APIBaseURL: "https://segredo.invalid", APIKey: "segredo-api", WebhookSecret: "segredo-hmac"}, casos: []canais.Caso{{ID: "reserva", Habilitado: true}}}
	gravador := gravadorFunc(func(context.Context, atende.RegistroSessao, escopo.Contato, pedidos.Carteira, []auditoria.Linha) error {
		return nil
	})
	servidor, cliente, entrada, err := IniciarComCanais(gravador, repo)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = servidor.Fechar(context.Background()) })
	canal, err := cliente.PorID(t.Context(), repo.canal.CompanyID, repo.canal.ID)
	if err != nil || canal.ID != repo.canal.ID || canal.APIBaseURL != "" || canal.APIKey != "" || canal.WebhookSecret != "" {
		t.Fatalf("canal=%+v erro=%v", canal, err)
	}
	if _, err := cliente.PorID(t.Context(), "outra", repo.canal.ID); !errors.Is(err, canais.ErrDesconhecido) {
		t.Fatalf("empresa diferente: %v", err)
	}
	casos, err := cliente.Casos(t.Context(), canal)
	if err != nil || len(casos) != 1 || casos[0].ID != "reserva" {
		t.Fatalf("casos=%+v erro=%v", casos, err)
	}
	corpo := []byte(`{"event":"teste"}`)
	mac := hmac.New(sha256.New, []byte(repo.canal.WebhookSecret))
	_, _ = mac.Write(append([]byte("123."), corpo...))
	assinatura := "t=123,v1=" + hex.EncodeToString(mac.Sum(nil))
	id := canais.Identidade{Provider: repo.canal.Provider, Tipo: repo.canal.Tipo, Tenant: repo.canal.Tenant, Endpoint: repo.canal.Endpoint}
	canal, err = entrada.VerificarWebhook(t.Context(), id, corpo, assinatura)
	if err != nil || canal.ID != repo.canal.ID || canal.WebhookSecret != "" || canal.APIKey != "" || canal.APIBaseURL != "" {
		t.Fatalf("webhook: canal=%+v erro=%v", canal, err)
	}
	if _, err := entrada.VerificarWebhook(t.Context(), id, corpo, "inválida"); !errors.Is(err, ErrWebhookNaoAutorizado) {
		t.Fatalf("assinatura inválida: %v", err)
	}
	local := NovaEntradaLocal(repo)
	canal, err = local.VerificarWebhook(t.Context(), id, corpo, assinatura)
	if err != nil || canal.ID != repo.canal.ID || canal.WebhookSecret != "" || canal.APIKey != "" || canal.APIBaseURL != "" {
		t.Fatalf("webhook local: canal=%+v erro=%v", canal, err)
	}
	if _, err := local.VerificarWebhook(t.Context(), id, corpo, "inválida"); !errors.Is(err, ErrWebhookNaoAutorizado) {
		t.Fatalf("assinatura local inválida: %v", err)
	}
	id.Endpoint = "canal-divergente"
	if _, err := local.VerificarWebhook(t.Context(), id, corpo, assinatura); !errors.Is(err, ErrWebhookNaoAutorizado) {
		t.Fatalf("canal local divergente: %v", err)
	}
	antes := repo.chamadas.Load()
	for _, tc := range []struct{ rota, token string }{{rotaWebhook, cliente.token}, {rotaCanal, entrada.token}} {
		req, err := http.NewRequest(http.MethodPost, cliente.base+tc.rota, strings.NewReader(`{}`))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+tc.token)
		resp, err := cliente.http.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("token de outra função: %d", resp.StatusCode)
		}
	}
	if repo.chamadas.Load() != antes {
		t.Fatal("token de outra função chegou ao repositório")
	}
}

func TestCanaisHTTPDistingueVazioDeIndisponivel(t *testing.T) {
	for _, tc := range []struct {
		nome string
		err  error
	}{
		{"vazio", nil},
		{"indisponível", errors.New("banco indisponível")},
	} {
		t.Run(tc.nome, func(t *testing.T) {
			repo := &repositorioCanalTeste{canal: canais.Canal{ID: uuid.New(), CompanyID: "empresa"}, errCasos: tc.err}
			gravador := gravadorFunc(func(context.Context, atende.RegistroSessao, escopo.Contato, pedidos.Carteira, []auditoria.Linha) error {
				return nil
			})
			servidor, cliente, _, err := IniciarComCanais(gravador, repo)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = servidor.Fechar(context.Background()) })
			casos, err := cliente.Casos(t.Context(), repo.canal)
			if tc.err == nil && (err != nil || casos == nil || len(casos) != 0) {
				t.Fatalf("vazio: casos=%v erro=%v", casos, err)
			}
			if tc.err != nil && (!errors.Is(err, ErrDadosIndisponiveis) || casos != nil) {
				t.Fatalf("indisponível: casos=%v erro=%v", casos, err)
			}
		})
	}
}
