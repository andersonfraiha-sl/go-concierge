package bancotest

import (
	"os"
	"strings"
	"testing"
)

func TestSessaoDoProcesso(t *testing.T) {
	tests := []struct {
		nome    string
		antes   string
		confere func(t *testing.T, depois string)
	}{
		{"sem sessão ganha uma própria", "", func(t *testing.T, depois string) {
			if !strings.HasPrefix(depois, "concierge-") {
				t.Fatalf("sessão = %q, quer uma própria do processo", depois)
			}
		}},
		{"sessão escolhida por quem roda fica", "do-ci", func(t *testing.T, depois string) {
			if depois != "do-ci" {
				t.Fatalf("sessão = %q, quer a escolhida do-ci", depois)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.nome, func(t *testing.T) {
			t.Setenv(variavelSessao, tt.antes)
			if err := sessaoDoProcesso(); err != nil {
				t.Fatal(err)
			}
			tt.confere(t, os.Getenv(variavelSessao))
		})
	}
}
