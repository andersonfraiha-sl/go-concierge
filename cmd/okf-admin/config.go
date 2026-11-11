package main

import (
	"errors"
	"strings"
)

// Config é o que o boot confere antes de abrir qualquer porta.
type Config struct {
	Ambiente     string // AMBIENTE: vazio ou dev, hml, prd
	AuthMode     string // AUTH_MODE: vazio (API fechada), dev (cabeçalho X-Dev-Usuario) ou proxy (X-Authenticated-User-Email)
	AtendeSecret string // ATENDE_API_SECRET: segredo do HMAC do webhook real
	// GANCHOS_SEGREDO_EXPORTACAO: segredo do HMAC que assina a exportação de regras
	SegredoExportacao string
	// INTEGRACAO_CHAVE: chave de serviço da rota de integração com o Nexus; vazia desliga a rota
	IntegracaoChave string
	// ACESSO_ADMIN_INICIAL: e-mail que o boot garante como admin da Superlógica; vazio não cria nada
	AdminInicial string
}

const (
	authModeDev   = "dev"
	authModeProxy = "proxy"
)

var (
	errAuthModeDesconhecido = errors.New("AUTH_MODE desconhecido: use vazio, dev ou proxy")
	errAdminInicialInvalido = errors.New("ACESSO_ADMIN_INICIAL não é um e-mail válido")
	errAuthDevForaDeDev     = errors.New("AUTH_MODE=dev só é aceito com AMBIENTE vazio ou dev")
	errSegredoAtendeAusente = errors.New("ATENDE_API_SECRET vazio fora de dev: o webhook real não teria como conferir a assinatura")
	errSegredoExportacao    = errors.New("GANCHOS_SEGREDO_EXPORTACAO vazio fora de dev: a exportação de regras não teria assinatura")
)

// normalizarConfig tira espaços e caixa de AMBIENTE e AUTH_MODE (" PRD" é prd).
func normalizarConfig(c Config) Config {
	c.Ambiente = strings.ToLower(strings.TrimSpace(c.Ambiente))
	c.AuthMode = strings.ToLower(strings.TrimSpace(c.AuthMode))
	c.AdminInicial = strings.TrimSpace(c.AdminInicial)
	return c
}

// validarConfig recusa combinações que abririam o servidor sem proteção. O modo dev é uma
// lista de permissão: só com AMBIENTE vazio ou dev; qualquer outro ambiente derruba o boot.
func validarConfig(c Config) error {
	c = normalizarConfig(c)
	if c.AuthMode != "" && c.AuthMode != authModeDev && c.AuthMode != authModeProxy {
		return errAuthModeDesconhecido
	}
	if _, ok := normalizarEmail(c.AdminInicial); c.AdminInicial != "" && !ok {
		return errAdminInicialInvalido
	}
	if c.AuthMode == authModeDev && !ehDesenvolvimento(c.Ambiente) {
		return errAuthDevForaDeDev
	}
	if !ehDesenvolvimento(c.Ambiente) && c.AtendeSecret == "" {
		return errSegredoAtendeAusente
	}
	if !ehDesenvolvimento(c.Ambiente) && c.SegredoExportacao == "" {
		return errSegredoExportacao
	}
	if c.IntegracaoChave != "" && len(c.IntegracaoChave) < minimoDaChave {
		return errChaveDeIntegracaoCurta
	}
	return nil
}

// telefoneMockB2C: o token B2C é de um telefone só. Com AMBIENTE=dev (e só nele), o webhook
// real troca o telefone de todo evento por esse, para testar com qualquer número. Fora de
// dev devolve vazio: misturaria os moradores num contato só.
func telefoneMockB2C(ambiente, telefoneB2C string) string {
	if ambiente != "dev" {
		return ""
	}
	return telefoneB2C
}
