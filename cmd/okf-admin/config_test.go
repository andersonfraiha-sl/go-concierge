package main

import (
	"errors"
	"testing"
)

func TestValidarConfig(t *testing.T) {
	casos := []struct {
		nome    string
		config  Config
		querErr error
	}{
		{nome: "dev sem segredo", config: Config{Ambiente: "dev", AuthMode: "dev"}},
		{nome: "ambiente vazio sem segredo", config: Config{AuthMode: "dev"}},
		{nome: "caixa e espaços normalizados", config: Config{Ambiente: " DEV ", AuthMode: "Dev"}},
		{nome: "prd sem auth dev e com segredo", config: Config{Ambiente: "prd", AtendeSecret: "s"}},
		{nome: "auth dev em PRD", config: Config{Ambiente: "PRD", AuthMode: "dev", AtendeSecret: "s"}, querErr: errAuthDevForaDeDev},
		{nome: "auth dev em prd com espaço", config: Config{Ambiente: " prd", AuthMode: "dev", AtendeSecret: "s"}, querErr: errAuthDevForaDeDev},
		{nome: "auth dev em prod", config: Config{Ambiente: "prod", AuthMode: "dev", AtendeSecret: "s"}, querErr: errAuthDevForaDeDev},
		{nome: "auth dev em producao", config: Config{Ambiente: "producao", AuthMode: "dev", AtendeSecret: "s"}, querErr: errAuthDevForaDeDev},
		{nome: "auth dev em hml", config: Config{Ambiente: "hml", AuthMode: "dev", AtendeSecret: "s"}, querErr: errAuthDevForaDeDev},
		{nome: "auth mode desconhecido", config: Config{Ambiente: "dev", AuthMode: "oauth"}, querErr: errAuthModeDesconhecido},
		{nome: "hml sem segredo do ATENDE", config: Config{Ambiente: "hml"}, querErr: errSegredoAtendeAusente},
		{nome: "prd sem segredo do ATENDE", config: Config{Ambiente: "prd"}, querErr: errSegredoAtendeAusente},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			err := validarConfig(c.config)
			if c.querErr == nil && err != nil {
				t.Fatalf("quero nil, veio %v", err)
			}
			if c.querErr != nil && !errors.Is(err, c.querErr) {
				t.Fatalf("quero %v, veio %v", c.querErr, err)
			}
		})
	}
}

func TestTelefoneMockB2CSoEmDev(t *testing.T) {
	tests := []struct {
		ambiente, telefone, want string
	}{
		{"dev", "5511900000000", "5511900000000"},
		{"dev", "", ""},
		{"", "5511900000000", ""},
		{"hml", "5511900000000", ""},
		{"prd", "5511900000000", ""},
	}
	for _, tt := range tests {
		if got := telefoneMockB2C(tt.ambiente, tt.telefone); got != tt.want {
			t.Errorf("telefoneMockB2C(%q, %q) = %q, want %q", tt.ambiente, tt.telefone, got, tt.want)
		}
	}
}
