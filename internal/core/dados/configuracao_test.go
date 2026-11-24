package dados

import (
	"context"
	"errors"
	"testing"

	"go-concierge/internal/core/banco/bancotest"
	adminpg "go-concierge/internal/core/dados/postgres/admin"
	canaispg "go-concierge/internal/core/dados/postgres/canais"
	atende "go-concierge/internal/harness/atendimento"
	admin "go-concierge/internal/harness/configuracao"
)

func TestConfiguracaoHTTPHerdaEscopoEReleAlteracoes(t *testing.T) {
	b := bancotest.Novo(t)
	r, _, _ := registroValido(t)
	if _, err := b.Pool.Exec(t.Context(), `INSERT INTO channels(id,company_id,condo_id,channel_type,provider,endpoint_identifier,feature_mode,enabled,root_agent,updated_at) VALUES($1,$2,$3,'WEB','test',$4,'INHERIT',true,'root',now())`, r.ChannelID, r.CompanyID, r.CondoID, r.ChannelID.String()); err != nil {
		t.Fatal(err)
	}
	pg := adminpg.NovoPostgres(b.Ent)
	if _, err := pg.SalvarPayload(t.Context(), admin.Alvo{Condominio: admin.Padrao}, "recurso", map[string]any{"global": "sim"}); err != nil {
		t.Fatal(err)
	}
	alvo := admin.Alvo{Dona: r.CompanyID, Condominio: "42"}
	if _, err := pg.SalvarPayload(t.Context(), alvo, "recurso", map[string]any{"condo": "primeira"}); err != nil {
		t.Fatal(err)
	}
	servidor, cliente, _, err := IniciarConfigurado(Config{Turno: gravadorFunc(nil), Canais: canaispg.NovoPostgres(b.Pool), Configuracao: pg})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = servidor.Fechar(context.Background()) })
	ctx := atende.ComEscopoSessao(t.Context(), r.CompanyID, r.ChannelID)
	for _, valor := range []string{"primeira", "segunda"} {
		if valor == "segunda" {
			if _, err := pg.SalvarPayload(t.Context(), alvo, "recurso", map[string]any{"condo": valor}); err != nil {
				t.Fatal(err)
			}
		}
		m, err := cliente.Conteudo(ctx, r.CompanyID, r.CondoID)
		if err != nil || m.Payloads["42"]["recurso"]["condo"] != valor || m.Payloads[admin.Padrao]["recurso"]["global"] != "sim" {
			t.Fatalf("conteudo=%+v erro=%v", m, err)
		}
	}
	if _, err := cliente.Conteudo(ctx, r.CompanyID, nil); !errors.Is(err, atende.ErrEscopoSessao) {
		t.Fatalf("canal fixo ampliado: %v", err)
	}
	if _, err := cliente.Conteudo(ctx, "outra", r.CondoID); !errors.Is(err, atende.ErrEscopoSessao) {
		t.Fatalf("empresa cruzada: %v", err)
	}
	if _, err := b.Pool.Exec(t.Context(), `ALTER TABLE payloads RENAME TO payloads_indisponiveis`); err != nil {
		t.Fatal(err)
	}
	if _, err := cliente.Conteudo(ctx, r.CompanyID, r.CondoID); !errors.Is(err, ErrDadosIndisponiveis) {
		t.Fatalf("falha virou vazio: %v", err)
	}
}
