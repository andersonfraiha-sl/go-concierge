package banco_test

import (
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"go-concierge/internal/banco/bancotest"
	"go-concierge/internal/banco/sqlc"
)

func TestMemoriaRecusaEmpresaVazia(t *testing.T) {
	b := bancotest.Novo(t)
	for _, company := range []string{"", " \t "} {
		err := b.SQL.SaveMemory(t.Context(), sqlc.SaveMemoryParams{CompanyID: company, PhoneNumber: "5511900000000", Facts: []byte(`[]`), UpdatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}})
		if err == nil || !strings.Contains(err.Error(), "check constraint") {
			t.Fatalf("SaveMemory empresa %q = %v", company, err)
		}
	}
}
