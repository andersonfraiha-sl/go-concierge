package conciergeapi

import (
	"context"
	atendeapi "go-concierge/internal/core/api/atende"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"go-concierge/internal/core/banco/bancotest"
	"go-concierge/internal/core/dados"
	canaispg "go-concierge/internal/core/dados/postgres/canais"
	sessoespg "go-concierge/internal/core/dados/postgres/sessoes"
	atende "go-concierge/internal/harness/atendimento"
)

func TestWebhookV2ResolveCanalAntesDaAssinatura(t *testing.T) {
	b := bancotest.Novo(t)
	ctx := t.Context()
	a, outro := uuid.New(), uuid.New()
	for i, id := range []uuid.UUID{a, outro} {
		empresa := []string{"Nexus-A", "Nexus-B"}[i]
		tenant := []string{"tenant-a", "tenant-b"}[i]
		secret := []string{"segredo-a", "segredo-b"}[i]
		if _, err := b.Pool.Exec(ctx, `INSERT INTO channels(id,company_id,name,channel_type,provider,endpoint_identifier,external_channel_id,feature_mode,enabled,root_agent,webhook_secret,updated_at) VALUES($1,$2,'Canal','WHATSAPP','ATENDE','551940402710',$3,'INHERIT',true,'agentes/concierge',$4,now())`, id, empresa, tenant, secret); err != nil {
			t.Fatal(err)
		}
	}
	dadosServidor, cliente, entrada, err := dados.IniciarComCanais(dados.NovoGravadorPostgres(b), canaispg.NovoPostgres(b.Pool))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = dadosServidor.Fechar(context.Background()) })
	s := &servidor{atende: &atende.Servico{Canais: cliente, Sessoes: atende.NewSessoes(sessoespg.NovoSessaoPostgres(b))}, entradaDados: entrada, atendeAPISecret: "global-nao-deve-valer"}
	t.Cleanup(s.turnos.Wait)
	for _, evento := range []string{atendeapi.EventoMensagemAtendente, atendeapi.EventoProtocoloFechadoSistema} {
		for _, tc := range []struct {
			nome, tenant, segredo string
			status                int
		}{
			{"canal A", "tenant-a", "segredo-a", 202},
			{"canal B", "tenant-b", "segredo-b", 202},
			{"segredo de B nao autoriza A", "tenant-a", "segredo-b", 401},
			{"segredo global nao autoriza A", "tenant-a", "global-nao-deve-valer", 401},
			{"tenant desconhecido", "tenant-x", "segredo-a", 401},
		} {
			t.Run(evento+"/"+tc.nome, func(t *testing.T) {
				corpo := corpoEventoTeste(evento, tc.tenant, "551940402710")
				req := httptest.NewRequest(http.MethodPost, "/webhook/atende?company_id=Forjada", strings.NewReader(corpo))
				req.Header.Set("X-Atende-Signature", assinaturaComSegredo(tc.segredo, corpo))
				w := httptest.NewRecorder()
				s.webhookAtende(w, req)
				if w.Code != tc.status {
					t.Fatalf("status=%d want=%d corpo=%s", w.Code, tc.status, w.Body.String())
				}
			})
		}
	}
	if _, err := b.Pool.Exec(ctx, `UPDATE channels SET webhook_secret='segredo-novo' WHERE id=$1`, a); err != nil {
		t.Fatal(err)
	}
	corpo := corpoEventoTeste(atendeapi.EventoMensagemAtendente, "tenant-a", "551940402710")
	for _, tc := range []struct {
		segredo string
		status  int
	}{{"segredo-a", 401}, {"segredo-novo", 202}} {
		req := httptest.NewRequest(http.MethodPost, "/webhook/atende", strings.NewReader(corpo))
		req.Header.Set("X-Atende-Signature", assinaturaComSegredo(tc.segredo, corpo))
		w := httptest.NewRecorder()
		s.webhookAtende(w, req)
		if w.Code != tc.status {
			t.Fatalf("recarga SQL: status=%d want=%d corpo=%s", w.Code, tc.status, w.Body.String())
		}
	}
	if _, err := b.Pool.Exec(ctx, `UPDATE channels SET enabled=false WHERE id=$1`, a); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/webhook/atende", strings.NewReader(corpo))
	req.Header.Set("X-Atende-Signature", assinaturaComSegredo("segredo-novo", corpo))
	w := httptest.NewRecorder()
	s.webhookAtende(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("canal desabilitado: %d %s", w.Code, w.Body.String())
	}
	req = httptest.NewRequest(http.MethodPost, "/emulador/atende/webhook?company_id=Nexus-A&channel_id="+a.String(), strings.NewReader(corpo))
	w = httptest.NewRecorder()
	s.webhookEmulado(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("canal desabilitado emulado: %d %s", w.Code, w.Body.String())
	}

}
