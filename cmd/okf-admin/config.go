package main

import "errors"

// Config é o que o boot confere antes de abrir qualquer porta.
type Config struct {
	Ambiente     string // AMBIENTE: vazio ou dev, hml, prd
	AuthMode     string // AUTH_MODE: dev aceita o usuário do cabeçalho X-Dev-Usuario
	AtendeSecret string // ATENDE_API_SECRET: segredo do HMAC do webhook real
}

var (
	errAuthDevEmProducao    = errors.New("AUTH_MODE=dev não é aceito com AMBIENTE=prd")
	errSegredoAtendeAusente = errors.New("ATENDE_API_SECRET vazio fora de dev: o webhook real não teria como conferir a assinatura")
)

// validarConfig recusa combinações que abririam o servidor sem proteção.
func validarConfig(c Config) error {
	if c.Ambiente == "prd" && c.AuthMode == "dev" {
		return errAuthDevEmProducao
	}
	if !ehDesenvolvimento(c.Ambiente) && c.AtendeSecret == "" {
		return errSegredoAtendeAusente
	}
	return nil
}
