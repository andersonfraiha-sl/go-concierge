package banco_test

import (
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"go-concierge/internal/banco/bancotest"
	"go-concierge/internal/banco/sqlc"
)

func TestMemoriaFatosRecusaVazio(t *testing.T) {
	quando := []pgtype.Timestamptz{{Time: time.Now(), Valid: true}}
	tests := []struct {
		name                     string
		administradora, telefone string
		chave                    string
	}{
		{"chave vazia", "adm-a", "5511900000000", ""},
		{"telefone vazio", "adm-a", "", "pets"},
		{"administradora vazia", "", "5511900000000", "pets"},
	}
	b := bancotest.Novo(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := b.SQL.GravarFatos(t.Context(), sqlc.GravarFatosParams{
				AdministradoraID: tt.administradora, Telefone: tt.telefone, Chaves: []string{tt.chave},
				Condominios: []string{""}, Textos: []string{"x"}, Fontes: []string{""}, Quandos: quando, Confiancas: []float32{0},
				AtualizadaEm: quando[0],
			})
			if err == nil || !strings.Contains(err.Error(), "check constraint") {
				t.Fatalf("GravarFatos = %v, want violação de check", err)
			}
		})
	}
}
