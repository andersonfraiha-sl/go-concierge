package conciergeapi

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"go-concierge/internal/core/api/execucao"
	"go-concierge/internal/core/banco/bancotest"
	"go-concierge/internal/core/catalogo"
	adminpg "go-concierge/internal/core/dados/postgres/admin"
	"go-concierge/internal/harness/canais"
)

type canalExecucaoTeste struct {
	canal    canais.Canal
	chamadas int
}

func (c *canalExecucaoTeste) PorID(_ context.Context, empresa string, id uuid.UUID) (canais.Canal, error) {
	c.chamadas++
	if c.canal.CompanyID != empresa || c.canal.ID != id {
		return canais.Canal{}, canais.ErrDesconhecido
	}
	return c.canal, nil
}
func (*canalExecucaoTeste) Casos(context.Context, canais.Canal) ([]canais.Caso, error) {
	return []canais.Caso{}, nil
}

func TestConfiguracaoDeExecucaoExigeAcessoGlobalEOmiteCredenciais(t *testing.T) {
	amb := novoAmbienteRotas(t)
	canal := &canalExecucaoTeste{canal: canais.Canal{ID: uuid.New(), CompanyID: "adm-a", Habilitado: true, Agente: "agentes/concierge", APIKey: "segredo-api-teste", WebhookSecret: "segredo-webhook-teste", APIBaseURL: "https://nao-expor.example"}}
	amb.s.atende.Canais = canal
	url := execucao.RotaConfiguracao + "?company_id=adm-a&channel_id=" + canal.canal.ID.String()
	for _, usuario := range []string{"operador-a", "gestor-a", "construtor-a", "gestor-av"} {
		if w := amb.pedir(t, usuario, http.MethodGet, url, ""); w.Code != http.StatusForbidden {
			t.Fatalf("usuário local %s recebeu catálogo: %d", usuario, w.Code)
		}
	}
	if canal.chamadas != 0 {
		t.Fatal("usuário sem acesso global chegou aos dados")
	}
	amb.s.banco = bancotest.Novo(t)
	amb.s.conteudoExecucao = adminpg.NovoPostgres(amb.s.banco.Ent)
	amb.s.catalogoExecucao = catalogo.NovoPostgres(amb.s.banco.Ent)
	w := amb.pedir(t, "admin", http.MethodGet, url, "")
	if w.Code != http.StatusOK {
		t.Fatalf("administrador global: %d %s", w.Code, w.Body)
	}
	for _, segredo := range []string{canal.canal.APIKey, canal.canal.WebhookSecret, canal.canal.APIBaseURL} {
		if strings.Contains(w.Body.String(), segredo) {
			t.Fatal("resposta pública expôs configuração de entrega")
		}
	}
	config := corpoDe[execucao.Configuracao](t, w)
	if config.Canal.CompanyID != canal.canal.CompanyID || config.Arquivos == nil || config.Casos == nil {
		t.Fatalf("configuração incompleta: %+v", config)
	}
	if w := amb.pedir(t, "admin", http.MethodGet, strings.Replace(url, "adm-a", "adm-b", 1), ""); w.Code != http.StatusNotFound {
		t.Fatalf("canal em outra empresa: %d", w.Code)
	}
}
