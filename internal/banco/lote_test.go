package banco_test

import (
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"go-concierge/internal/banco"
	"go-concierge/internal/banco/bancotest"
	"go-concierge/internal/banco/sqlc"
)

// Queries diferentes no mesmo Lote vão numa ida, e cada leitura recebe o resultado da sua.
func TestLoteLeCadaResultadoNaOrdem(t *testing.T) {
	b := bancotest.Novo(t)
	ctx := t.Context()
	agora := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	gravar := banco.NovoLote()
	gravar.Por(func(q *sqlc.Queries) func() error {
		res := q.SalvarSessao(ctx, []sqlc.SalvarSessaoParams{{ID: "s-1", AtualizadaEm: agora, Dados: []byte(`{}`)}})
		return func() (err error) { res.Exec(func(_ int, e error) { err = e }); return err }
	})
	if err := gravar.Enviar(ctx, b.Pool); err != nil {
		t.Fatalf("gravar sessão: %v", err)
	}

	var semPedidos, achouSessao, achouAusente bool
	ler := banco.NovoLote()
	ler.Por(func(q *sqlc.Queries) func() error {
		res := q.LerPedidos(ctx, []sqlc.LerPedidosParams{{AdministradoraID: "adm-a", Telefone: "5511999999999"}})
		return func() (err error) {
			res.Query(func(_ int, linhas []sqlc.LerPedidosRow, e error) { err, semPedidos = e, len(linhas) == 0 })
			return err
		}
	})
	ler.Por(func(q *sqlc.Queries) func() error {
		res := q.CarregarSessao(ctx, []string{"s-1", "s-2"})
		return func() (err error) {
			res.QueryRow(func(i int, s sqlc.SessaoGravada, e error) {
				switch {
				case i == 0 && e == nil && s.ID == "s-1":
					achouSessao = true
				case i == 1 && errors.Is(e, pgx.ErrNoRows):
					achouAusente = true
				case e != nil && !errors.Is(e, pgx.ErrNoRows):
					err = e
				}
			})
			return err
		}
	})
	if err := ler.Enviar(ctx, b.Pool); err != nil {
		t.Fatalf("Enviar: %v", err)
	}
	if !semPedidos || !achouSessao || !achouAusente {
		t.Fatalf("leituras: sem pedidos %v, sessão s-1 %v, s-2 ausente %v", semPedidos, achouSessao, achouAusente)
	}
}
