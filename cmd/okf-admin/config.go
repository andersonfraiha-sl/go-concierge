package main

import (
	"errors"
	"strings"
)

// Config é o que o boot confere antes de abrir qualquer porta.
type Config struct {
	Ambiente     string // AMBIENTE: vazio ou dev, hml, prd
	AuthMode     string // AUTH_MODE: vazio (API fechada) ou dev (usuário do cabeçalho X-Dev-Usuario)
	AtendeSecret string // ATENDE_API_SECRET: segredo do HMAC do webhook real
}

const authModeDev = "dev"

var (
	errAuthModeDesconhecido = errors.New("AUTH_MODE desconhecido: use vazio ou dev")
	errAuthDevForaDeDev     = errors.New("AUTH_MODE=dev só é aceito com AMBIENTE vazio ou dev")
	errSegredoAtendeAusente = errors.New("ATENDE_API_SECRET vazio fora de dev: o webhook real não teria como conferir a assinatura")
)

// normalizarConfig tira espaços e caixa de AMBIENTE e AUTH_MODE (" PRD" é prd).
func normalizarConfig(c Config) Config {
	c.Ambiente = strings.ToLower(strings.TrimSpace(c.Ambiente))
	c.AuthMode = strings.ToLower(strings.TrimSpace(c.AuthMode))
	return c
}

// validarConfig recusa combinações que abririam o servidor sem proteção. O modo dev é uma
// lista de permissão: só com AMBIENTE vazio ou dev; qualquer outro ambiente derruba o boot.
func validarConfig(c Config) error {
	c = normalizarConfig(c)
	if c.AuthMode != "" && c.AuthMode != authModeDev {
		return errAuthModeDesconhecido
	}
	if c.AuthMode == authModeDev && !ehDesenvolvimento(c.Ambiente) {
		return errAuthDevForaDeDev
	}
	if !ehDesenvolvimento(c.Ambiente) && c.AtendeSecret == "" {
		return errSegredoAtendeAusente
	}
	return nil
}
