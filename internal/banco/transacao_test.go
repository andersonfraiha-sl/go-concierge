package banco_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"go-concierge/internal/banco"
	"go-concierge/internal/banco/bancotest"
	"go-concierge/internal/banco/sqlc"
)

var errDesistir = errors.New("desistir")

// gravarNosDois escreve pelo ent e pelo sqlc na mesma transação.
func gravarNosDois(ctx context.Context, t banco.Transacao, falhar bool) error {
	quando := time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)
	t.Ent.MemoriaContato.Create().SetAdministradoraID("adm-a").SetTelefone("5511900000000").SetAtualizadaEm(quando).SaveX(ctx)
	err := t.SQL(func(q *sqlc.Queries) error {
		return q.GravarFatos(ctx, sqlc.GravarFatosParams{
			AdministradoraID: "adm-a", Telefone: "5511900000000", Chaves: []string{"pets"},
			Condominios: []string{""}, Textos: []string{"gato"}, Fontes: []string{""},
			Quandos: []pgtype.Timestamptz{{Time: quando, Valid: true}}, Confiancas: []float32{0},
		})
	})
	if err != nil || !falhar {
		return err
	}
	return errDesistir
}

func TestEmTransacao(t *testing.T) {
	tests := []struct {
		name      string
		falhar    bool
		wantLinha int
	}{
		{"confirma ent e sqlc juntos", false, 1},
		{"desfaz ent e sqlc juntos", true, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := bancotest.Novo(t)

			err := b.EmTransacao(t.Context(), func(ctx context.Context, tx banco.Transacao) error {
				return gravarNosDois(ctx, tx, tt.falhar)
			})

			if tt.falhar != errors.Is(err, errDesistir) || (!tt.falhar && err != nil) {
				t.Fatalf("EmTransacao = %v", err)
			}
			contatos := b.Ent.MemoriaContato.Query().CountX(t.Context())
			fatos := b.Ent.MemoriaFato.Query().CountX(t.Context())
			if contatos != tt.wantLinha || fatos != tt.wantLinha {
				t.Fatalf("contatos=%d fatos=%d, want %d", contatos, fatos, tt.wantLinha)
			}
		})
	}
}

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
			})
			if err == nil || !strings.Contains(err.Error(), "check constraint") {
				t.Fatalf("GravarFatos = %v, want violação de check", err)
			}
		})
	}
}
