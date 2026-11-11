package banco_test

import (
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"go-concierge/internal/banco"
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

func TestMemoriaPadroesRecusaVazio(t *testing.T) {
	agora := []pgtype.Timestamptz{{Time: time.Now(), Valid: true}}
	tests := []struct {
		name                           string
		administradora, telefone, caso string
		condominio                     string
	}{
		{"caso vazio", "adm-a", "5511900000000", "", "c1"},
		{"condomínio vazio", "adm-a", "5511900000000", "segunda_via", ""},
		{"telefone vazio", "adm-a", "", "segunda_via", "c1"},
		{"administradora vazia", "", "5511900000000", "segunda_via", "c1"},
	}
	b := bancotest.Novo(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lote := banco.NovoLote()
			lote.Por(func(q *sqlc.Queries) func() error {
				res := q.GravarPadroes(t.Context(), []sqlc.GravarPadroesParams{{
					AdministradoraID: tt.administradora, Telefone: tt.telefone,
					Casos: []string{tt.caso}, Nomes: []string{""}, Condominios: []string{tt.condominio}, Unidades: []string{""},
					Valores: []float64{1}, Atualizados: agora, Suprimidos: []pgtype.Timestamptz{{}}, AtualizadaEm: agora[0],
				}})
				return func() error {
					var erro error
					res.Exec(func(_ int, err error) { erro = err })
					return erro
				}
			})
			err := lote.Enviar(t.Context(), b.Pool)
			if err == nil || !strings.Contains(err.Error(), "check constraint") {
				t.Fatalf("GravarPadroes = %v, want violação de check", err)
			}
		})
	}
}
