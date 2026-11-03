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
		{nome: "hml com segredo e auth dev", config: Config{Ambiente: "hml", AuthMode: "dev", AtendeSecret: "s"}},
		{nome: "prd com segredo e sem auth dev", config: Config{Ambiente: "prd", AtendeSecret: "s"}},
		{nome: "prd com auth dev", config: Config{Ambiente: "prd", AuthMode: "dev", AtendeSecret: "s"}, querErr: errAuthDevEmProducao},
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
