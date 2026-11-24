package banco_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"go-concierge/internal/core/banco"
	"go-concierge/internal/core/banco/bancotest"
	"go-concierge/internal/core/banco/sqlc"
)

func TestLoteLeCadaResultadoNaOrdem(t *testing.T) {
	b := bancotest.Novo(t)
	ctx := t.Context()
	channel := uuid.New()
	_, err := b.Pool.Exec(ctx, `INSERT INTO channels(id,company_id,channel_type,provider,endpoint_identifier,feature_mode,enabled,root_agent,updated_at) VALUES($1,'company','WEB','test','test','INHERIT',true,'root',now())`, channel)
	if err != nil {
		t.Fatal(err)
	}
	agora := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	gravar := banco.NovoLote()
	gravar.Por(func(q *sqlc.Queries) func() error {
		res := q.SaveSession(ctx, []sqlc.SaveSessionParams{{ID: "s-1", CompanyID: "company", ChannelID: pgtype.UUID{Bytes: channel, Valid: true}, Connector: "test", Protocol: "p-1", PhoneNumber: "5511999999999", UpdatedAt: agora, Data: []byte(`{}`)}})
		return func() (err error) { res.QueryRow(func(_ int, _ string, e error) { err = e }); return err }
	})
	if err := gravar.Enviar(ctx, b.Pool); err != nil {
		t.Fatal(err)
	}
	var semPedidos, achouSessao, achouAusente bool
	ler := banco.NovoLote()
	ler.Por(func(q *sqlc.Queries) func() error {
		res := q.LoadRequests(ctx, []sqlc.LoadRequestsParams{{CompanyID: "company", PhoneNumber: "5511999999999", AllCondos: true}})
		return func() (err error) {
			res.Query(func(_ int, linhas []sqlc.LoadRequestsRow, e error) { err, semPedidos = e, len(linhas) == 0 })
			return err
		}
	})
	ler.Por(func(q *sqlc.Queries) func() error {
		res := q.LoadSession(ctx, []sqlc.LoadSessionParams{{ID: "s-1", CompanyID: "company", AllChannels: true}, {ID: "s-2", CompanyID: "company", AllChannels: true}})
		return func() (err error) {
			res.QueryRow(func(i int, s sqlc.Session, e error) {
				switch {
				case i == 0 && e == nil && s.ID == "s-1":
					achouSessao = true
				case i == 1 && errors.Is(e, pgx.ErrNoRows):
					achouAusente = true
				case e != nil:
					err = e
				}
			})
			return err
		}
	})
	if err := ler.Enviar(ctx, b.Pool); err != nil {
		t.Fatal(err)
	}
	if !semPedidos || !achouSessao || !achouAusente {
		t.Fatalf("leituras: %v %v %v", semPedidos, achouSessao, achouAusente)
	}
}

func TestLoteReverteQuandoValidacaoDoResultadoFalha(t *testing.T) {
	b := bancotest.Novo(t)
	ctx := t.Context()
	lote := banco.NovoLote()
	rejeitado := errors.New("validação rejeitada")
	lote.Por(func(q *sqlc.Queries) func() error {
		res := q.InserirAuditoria(ctx, []sqlc.InserirAuditoriaParams{{CriadoEm: []pgtype.Timestamptz{{Time: time.Now(), Valid: true}}, Origem: []string{"test"}, Evento: []string{"test"}, Gancho: []string{"test"}, RegraID: []string{""}, RegraVersao: []int32{0}, Decisao: []string{"test"}, Motivo: []string{"test"}, AdministradoraID: []string{""}, CondominioID: []string{""}, SessaoID: []string{""}, Protocolo: []string{""}, UsuarioID: []string{""}, Tela: []string{""}, Ferramenta: []string{""}, Quantidade: []int32{0}}})
		return func() error {
			var err error
			res.Exec(func(_ int, e error) { err = e })
			return errors.Join(err, rejeitado)
		}
	})
	if err := lote.Enviar(ctx, b.Pool); !errors.Is(err, rejeitado) {
		t.Fatalf("Enviar = %v", err)
	}
	var count int
	if err := b.Pool.QueryRow(ctx, `SELECT count(*) FROM hook_audit_logs`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("auditorias=%d (%v)", count, err)
	}
}
