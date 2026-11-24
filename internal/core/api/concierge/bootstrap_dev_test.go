package conciergeapi

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	"go-concierge/internal/core/banco/bancotest"
)

func TestBootstrapAtendeDevSemeiaCanalECasosNaPrimeiraCriacao(t *testing.T) {
	db := bancotest.Novo(t)
	for _, id := range []string{"caso_a", "caso_b"} {
		caminho := "agentes/concierge/agentes/" + id + "/caso.yaml"
		_, err := db.Pool.Exec(t.Context(), `INSERT INTO catalog_files(path,content,sha256,updated_at) VALUES($1,'','sha',now())`, caminho)
		if err != nil {
			t.Fatal(err)
		}
		_, err = db.Pool.Exec(t.Context(), `INSERT INTO use_cases(id,file_path,agent_path,name,summary,visibility,source_sha256,updated_at) VALUES($1,$2,'agentes/concierge',$1,'resumo','private','sha',now())`, id, caminho)
		if err != nil {
			t.Fatal(err)
		}
	}
	cfg := configBootstrapDev{CompanyID: "empresa-opaca/9", Tenant: "tenant-dev", Telefone: "5511999999999", Agente: "agentes/concierge", APIBaseURL: "https://atende.test/v2", APIKey: "api-key-test", WebhookSecret: "webhook-secret-test", GruviCompany: "gruvi-explicita", NexusAccount: "account-explicita"}
	if err := semearBootstrapDevAtende(t.Context(), "dev", true, db, cfg); err != nil {
		t.Fatal(err)
	}
	var channelID uuid.UUID
	var company string
	var condo *int
	var name, apiKey, webhookSecret, account, gruviCompany string
	err := db.Pool.QueryRow(t.Context(), `SELECT id,company_id,condo_id,name,api_key,webhook_secret,config_jsonb->>'nexus_account_id',config_jsonb->>'gruvi_company_id' FROM channels WHERE provider='ATENDE' AND channel_type='WHATSAPP' AND external_channel_id=$1 AND endpoint_identifier=$2`, cfg.Tenant, cfg.Telefone).Scan(&channelID, &company, &condo, &name, &apiKey, &webhookSecret, &account, &gruviCompany)
	if err != nil {
		t.Fatal(err)
	}
	if channelID == uuid.Nil || company != cfg.CompanyID || condo != nil || name != "Atende" || apiKey != cfg.APIKey || webhookSecret != cfg.WebhookSecret || account != cfg.NexusAccount || gruviCompany != cfg.GruviCompany {
		t.Fatalf("canal sem os valores explícitos esperados: id=%s company=%q condo=%v name=%q", channelID, company, condo, name)
	}
	var ativacoes, habilitadas, disponiveis, semRestricao int
	err = db.Pool.QueryRow(t.Context(), `SELECT count(*),count(*) FILTER (WHERE enabled),count(*) FILTER (WHERE default_available),count(*) FILTER (WHERE cardinality(roles)=0) FROM company_use_cases WHERE company_id=$1`, cfg.CompanyID).Scan(&ativacoes, &habilitadas, &disponiveis, &semRestricao)
	if err != nil {
		t.Fatal(err)
	}
	if ativacoes != 2 || habilitadas != 2 || disponiveis != 2 || semRestricao != 2 {
		t.Fatalf("ativações iniciais incorretas: total=%d habilitadas=%d disponíveis=%d roles vazios=%d", ativacoes, habilitadas, disponiveis, semRestricao)
	}
}

func TestBootstrapAtendeDevReinicioPreservaAlteracoesERemocoes(t *testing.T) {
	db := bancotest.Novo(t)
	for _, id := range []string{"caso_a", "caso_b"} {
		caminho := "agentes/concierge/agentes/" + id + "/caso.yaml"
		_, err := db.Pool.Exec(t.Context(), `INSERT INTO catalog_files(path,content,sha256,updated_at) VALUES($1,'','sha',now())`, caminho)
		if err != nil {
			t.Fatal(err)
		}
		_, err = db.Pool.Exec(t.Context(), `INSERT INTO use_cases(id,file_path,agent_path,name,summary,visibility,source_sha256,updated_at) VALUES($1,$2,'agentes/concierge',$1,'resumo','private','sha',now())`, id, caminho)
		if err != nil {
			t.Fatal(err)
		}
	}
	cfg := configBootstrapDev{CompanyID: "empresa-dev", Tenant: "tenant-dev", Telefone: "5511999999999", APIBaseURL: "https://atende.test/v2", APIKey: "seed-key", WebhookSecret: "seed-secret"}
	if err := semearBootstrapDevAtende(t.Context(), "dev", true, db, cfg); err != nil {
		t.Fatal(err)
	}
	var channelID, casoMantido uuid.UUID
	var casoRemovido uuid.UUID
	if err := db.Pool.QueryRow(t.Context(), `SELECT id FROM channels WHERE company_id=$1`, cfg.CompanyID).Scan(&channelID); err != nil {
		t.Fatal(err)
	}
	if err := db.Pool.QueryRow(t.Context(), `SELECT id FROM company_use_cases WHERE company_id=$1 AND use_case_id='caso_a'`, cfg.CompanyID).Scan(&casoMantido); err != nil {
		t.Fatal(err)
	}
	if err := db.Pool.QueryRow(t.Context(), `SELECT id FROM company_use_cases WHERE company_id=$1 AND use_case_id='caso_b'`, cfg.CompanyID).Scan(&casoRemovido); err != nil {
		t.Fatal(err)
	}
	_, err := db.Pool.Exec(t.Context(), `UPDATE channels SET enabled=false,api_key='sql-key',webhook_secret='sql-secret' WHERE id=$1`, channelID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Pool.Exec(t.Context(), `UPDATE company_use_cases SET enabled=false,default_available=false,roles=ARRAY['operator'] WHERE id=$1`, casoMantido)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Pool.Exec(t.Context(), `DELETE FROM company_use_cases WHERE id=$1`, casoRemovido)
	if err != nil {
		t.Fatal(err)
	}
	overrideID := uuid.New()
	_, err = db.Pool.Exec(t.Context(), `INSERT INTO channel_use_cases(id,company_id,channel_id,company_use_case_id,enabled,updated_at) VALUES($1,$2,$3,$4,false,now())`, overrideID, cfg.CompanyID, channelID, casoMantido)
	if err != nil {
		t.Fatal(err)
	}
	cfg.APIKey, cfg.WebhookSecret = "new-key", "new-secret"
	if err := semearBootstrapDevAtende(t.Context(), "dev", true, db, cfg); err != nil {
		t.Fatal(err)
	}
	var habilitado bool
	var apiKey, webhookSecret string
	if err := db.Pool.QueryRow(t.Context(), `SELECT enabled,api_key,webhook_secret FROM channels WHERE id=$1`, channelID).Scan(&habilitado, &apiKey, &webhookSecret); err != nil {
		t.Fatal(err)
	}
	if habilitado || apiKey != "sql-key" || webhookSecret != "sql-secret" {
		t.Fatalf("canal gerenciado por SQL foi alterado: enabled=%v", habilitado)
	}
	var enabled, defaultAvailable bool
	var roles []string
	if err := db.Pool.QueryRow(t.Context(), `SELECT enabled,default_available,roles FROM company_use_cases WHERE id=$1`, casoMantido).Scan(&enabled, &defaultAvailable, &roles); err != nil {
		t.Fatal(err)
	}
	if enabled || defaultAvailable || len(roles) != 1 || roles[0] != "operator" {
		t.Fatalf("configuração empresarial gerenciada por SQL foi alterada: enabled=%v default=%v roles=%v", enabled, defaultAvailable, roles)
	}
	var count int
	if err := db.Pool.QueryRow(t.Context(), `SELECT count(*) FROM company_use_cases WHERE company_id=$1`, cfg.CompanyID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("ativação removida foi restaurada ou dados duplicados: count=%d", count)
	}
	var overrideEnabled bool
	if err := db.Pool.QueryRow(t.Context(), `SELECT enabled FROM channel_use_cases WHERE id=$1`, overrideID).Scan(&overrideEnabled); err != nil {
		t.Fatal(err)
	}
	if overrideEnabled {
		t.Fatal("override gerenciado por SQL foi alterado")
	}
}

func TestBootstrapAtendeDevSegundoCanalNaoRestauraAtivacaoDaEmpresa(t *testing.T) {
	db := bancotest.Novo(t)
	for _, id := range []string{"caso_a", "caso_b"} {
		caminho := "agentes/concierge/agentes/" + id + "/caso.yaml"
		_, err := db.Pool.Exec(t.Context(), `INSERT INTO catalog_files(path,content,sha256,updated_at) VALUES($1,'','sha',now())`, caminho)
		if err != nil {
			t.Fatal(err)
		}
		_, err = db.Pool.Exec(t.Context(), `INSERT INTO use_cases(id,file_path,agent_path,name,summary,visibility,source_sha256,updated_at) VALUES($1,$2,'agentes/concierge',$1,'resumo','private','sha',now())`, id, caminho)
		if err != nil {
			t.Fatal(err)
		}
	}
	cfg := configBootstrapDev{CompanyID: "empresa-dev", Tenant: "tenant-dev", Telefone: "5511999999999", APIBaseURL: "https://atende.test/v2", APIKey: "seed-key", WebhookSecret: "seed-secret"}
	if err := semearBootstrapDevAtende(t.Context(), "dev", true, db, cfg); err != nil {
		t.Fatal(err)
	}
	_, err := db.Pool.Exec(t.Context(), `DELETE FROM company_use_cases WHERE company_id=$1 AND use_case_id='caso_a'`, cfg.CompanyID)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Tenant = "tenant-dev-2"
	cfg.Telefone = "(11) 98888-7777"
	if err := semearBootstrapDevAtende(t.Context(), "dev", true, db, cfg); err != nil {
		t.Fatal(err)
	}
	var endpoint string
	if err := db.Pool.QueryRow(t.Context(), `SELECT endpoint_identifier FROM channels WHERE company_id=$1 AND external_channel_id=$2`, cfg.CompanyID, cfg.Tenant).Scan(&endpoint); err != nil {
		t.Fatal(err)
	}
	if endpoint != "5511988887777" {
		t.Fatalf("telefone não foi normalizado pelo runtime: %q", endpoint)
	}
	var count int
	if err := db.Pool.QueryRow(t.Context(), `SELECT count(*) FROM company_use_cases WHERE company_id=$1 AND use_case_id='caso_a'`, cfg.CompanyID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("segundo canal restaurou ativação removida da empresa: count=%d", count)
	}
}

func TestBootstrapAtendeDevExigeEmpresaEnaoExpõeSegredos(t *testing.T) {
	db := bancotest.Novo(t)
	cfg := configBootstrapDev{Tenant: "tenant-dev", Telefone: "5511999999999", APIBaseURL: "https://atende.test/v2", APIKey: "api-key-secreto", WebhookSecret: "webhook-secreto"}
	err := semearBootstrapDevAtende(t.Context(), "dev", true, db, cfg)
	if err == nil || !strings.Contains(err.Error(), "NEXUS_COMPANY_ID") {
		t.Fatalf("esperava erro claro para NEXUS_COMPANY_ID ausente; recebido %v", err)
	}
	if strings.Contains(err.Error(), cfg.APIKey) || strings.Contains(err.Error(), cfg.WebhookSecret) {
		t.Fatal("erro do bootstrap expôs credencial")
	}
	var count int
	if err := db.Pool.QueryRow(t.Context(), `SELECT count(*) FROM channels`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("canal cadastrado sem empresa explícita: count=%d", count)
	}
}

func TestBootstrapAtendeDevNaoSemeiaForaDeDev(t *testing.T) {
	db := bancotest.Novo(t)
	err := semearBootstrapDevAtende(t.Context(), "prd", true, db, configBootstrapDev{})
	if err != nil {
		t.Fatal(err)
	}
	var channels, companyUseCases int
	if err := db.Pool.QueryRow(t.Context(), `SELECT (SELECT count(*) FROM channels),(SELECT count(*) FROM company_use_cases)`).Scan(&channels, &companyUseCases); err != nil {
		t.Fatal(err)
	}
	if channels != 0 || companyUseCases != 0 {
		t.Fatalf("seed ocorreu fora de dev: channels=%d company_use_cases=%d", channels, companyUseCases)
	}
}
