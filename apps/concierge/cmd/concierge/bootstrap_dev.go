package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"go-concierge/internal/banco"
	"go-concierge/internal/canais"
	"go-concierge/internal/escopo"
)

const namespaceBootstrapDev = "go-concierge/bootstrap-dev/v2"

type configBootstrapDev struct {
	CompanyID     string
	Tenant        string
	Telefone      string
	Agente        string
	APIBaseURL    string
	APIKey        string
	WebhookSecret string
	GruviCompany  string
	NexusAccount  string
}

func semearBootstrapDevAtende(ctx context.Context, ambiente string, habilitado bool, db banco.Banco, cfg configBootstrapDev) error {
	if !habilitado || !ehDesenvolvimento(strings.ToLower(strings.TrimSpace(ambiente))) {
		return nil
	}
	cfg.CompanyID = strings.TrimSpace(cfg.CompanyID)
	cfg.Tenant = strings.TrimSpace(cfg.Tenant)
	telefoneOriginal := strings.TrimSpace(cfg.Telefone)
	cfg.Telefone = escopo.NormalizarTelefone(telefoneOriginal)
	if telefoneOriginal != "" && cfg.Telefone == "" {
		return errors.New("bootstrap DEV do ATENDE: ATENDE_TENANT_TELEFONE inválido")
	}
	cfg.Agente = strings.TrimSpace(cfg.Agente)
	cfg.APIBaseURL = strings.TrimSpace(cfg.APIBaseURL)
	cfg.GruviCompany = strings.TrimSpace(cfg.GruviCompany)
	cfg.NexusAccount = strings.TrimSpace(cfg.NexusAccount)
	var faltando []string
	for nome, valor := range map[string]string{
		"NEXUS_COMPANY_ID":       cfg.CompanyID,
		"ATENDE_TENANT_ID":       cfg.Tenant,
		"ATENDE_TENANT_TELEFONE": cfg.Telefone,
		"ATENDE_API_BASE_URL":    cfg.APIBaseURL,
		"ATENDE_API_KEY":         cfg.APIKey,
		"ATENDE_API_SECRET":      cfg.WebhookSecret,
	} {
		if strings.TrimSpace(valor) == "" {
			faltando = append(faltando, nome)
		}
	}
	if len(faltando) > 0 {
		return fmt.Errorf("bootstrap DEV do ATENDE exige configuração: %s", strings.Join(faltando, ", "))
	}
	if cfg.Agente == "" {
		cfg.Agente = "agentes/concierge"
	}
	return semearAtendeEUsos(ctx, db, cfg)
}

func semearAtendeEUsos(ctx context.Context, db banco.Banco, cfg configBootstrapDev) (err error) {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("bootstrap DEV do ATENDE: iniciar transação: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, cfg.CompanyID); err != nil {
		return fmt.Errorf("bootstrap DEV do ATENDE: serializar seed da empresa: %w", err)
	}
	var empresaSemCadastro bool
	if err := tx.QueryRow(ctx, `SELECT NOT EXISTS (SELECT 1 FROM channels WHERE company_id=$1)
		AND NOT EXISTS (SELECT 1 FROM company_use_cases WHERE company_id=$1)`, cfg.CompanyID).Scan(&empresaSemCadastro); err != nil {
		return fmt.Errorf("bootstrap DEV do ATENDE: conferir cadastro empresarial: %w", err)
	}

	channelID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(namespaceBootstrapDev+"/channel/"+cfg.CompanyID+"/"+cfg.Tenant+"/"+cfg.Telefone))
	configJSON, err := json.Marshal(canais.Integracoes{NexusAccountID: cfg.NexusAccount, GruviCompanyID: cfg.GruviCompany})
	if err != nil {
		return fmt.Errorf("bootstrap DEV do ATENDE: serializar integrações do canal: %w", err)
	}
	canalInserido, err := tx.Exec(ctx, `INSERT INTO channels (
		id, company_id, condo_id, name, channel_type, provider, endpoint_identifier,
		external_channel_id, feature_mode, enabled, root_agent, api_base_url, api_key,
		webhook_secret, config_jsonb, updated_at
	) VALUES ($1,$2,NULL,'Atende','WHATSAPP','ATENDE',$3,$4,'INHERIT',true,$5,$6,$7,$8,$9::jsonb,$10)
	ON CONFLICT DO NOTHING`, channelID, cfg.CompanyID, cfg.Telefone, cfg.Tenant,
		cfg.Agente, cfg.APIBaseURL, cfg.APIKey, cfg.WebhookSecret, string(configJSON), time.Now().UTC())
	if err != nil {
		return fmt.Errorf("bootstrap DEV do ATENDE: cadastrar canal: %w", err)
	}
	inseridos := canalInserido.RowsAffected()
	var companyID string
	err = tx.QueryRow(ctx, `SELECT company_id FROM channels
		WHERE provider='ATENDE' AND channel_type='WHATSAPP'
		AND external_channel_id IS NOT DISTINCT FROM $1 AND endpoint_identifier=$2`, cfg.Tenant, cfg.Telefone).Scan(&companyID)
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("bootstrap DEV do ATENDE: conflito no identificador do canal; confira channels por SQL")
	}
	if err != nil {
		return fmt.Errorf("bootstrap DEV do ATENDE: consultar canal: %w", err)
	}
	if companyID != cfg.CompanyID {
		return fmt.Errorf("bootstrap DEV do ATENDE: o identificador do canal já pertence a outra empresa")
	}
	if inseridos == 0 || !empresaSemCadastro {
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("bootstrap DEV do ATENDE: confirmar transação: %w", err)
		}
		return nil
	}

	rows, err := tx.Query(ctx, `SELECT id FROM use_cases ORDER BY id`)
	if err != nil {
		return fmt.Errorf("bootstrap DEV do ATENDE: listar casos do catálogo: %w", err)
	}
	var casos []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return fmt.Errorf("bootstrap DEV do ATENDE: ler caso do catálogo: %w", err)
		}
		casos = append(casos, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("bootstrap DEV do ATENDE: ler casos do catálogo: %w", err)
	}
	rows.Close()
	if len(casos) == 0 {
		return errors.New("bootstrap DEV do ATENDE: catálogo sem casos; confira a semente")
	}
	for _, useCaseID := range casos {
		id := uuid.NewSHA1(uuid.NameSpaceOID, []byte(namespaceBootstrapDev+"/company-use-case/"+cfg.CompanyID+"/"+useCaseID))
		_, err := tx.Exec(ctx, `INSERT INTO company_use_cases (
			id, company_id, use_case_id, enabled, default_available, roles, updated_at
		) VALUES ($1,$2,$3,true,true,'{}'::text[],$4) ON CONFLICT (company_id,use_case_id) DO NOTHING`,
			id, cfg.CompanyID, useCaseID, time.Now().UTC())
		if err != nil {
			return fmt.Errorf("bootstrap DEV do ATENDE: ativar caso do catálogo: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("bootstrap DEV do ATENDE: confirmar transação: %w", err)
	}
	return nil
}
