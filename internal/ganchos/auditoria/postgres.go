package auditoria

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgtype"

	"go-concierge/internal/banco"
	"go-concierge/internal/banco/sqlc"
)

// NoLote põe as linhas no lote, num comando só. Sem linhas, não enfileira nada.
// O ctx do sqlc não chega ao banco aqui: quem envia é Lote.Enviar, com o ctx dele.
func NoLote(lote *banco.Lote, linhas []Linha) {
	if len(linhas) == 0 {
		return
	}
	lote.Por(func(q *sqlc.Queries) func() error {
		res := q.InserirAuditoria(context.Background(), []sqlc.InserirAuditoriaParams{paramsDe(linhas)})
		return func() (err error) {
			res.Exec(func(_ int, e error) { err = e })
			return err
		}
	})
}

// Gravar grava as linhas num lote. Se o lote falha, tenta linha a linha; a que ainda falhar
// vira log ERROR com a linha e aoFalhar(origem). Devolve os erros das linhas perdidas.
func Gravar(ctx context.Context, b banco.Banco, linhas []Linha, aoFalhar func(origem string)) error {
	if len(linhas) == 0 {
		return nil
	}
	if enviar(ctx, b, linhas) == nil {
		return nil
	}
	var perdidas []error
	for _, l := range linhas {
		err := enviar(ctx, b, []Linha{l})
		if err == nil {
			continue
		}
		slog.ErrorContext(ctx, "auditoria não gravada", "operacao", "gravar_auditoria", "origem", l.Origem,
			"evento", l.Evento, "gancho", l.Gancho, "regra_id", l.RegraID, "regra_versao", l.RegraVersao,
			"decisao", l.Decisao, "motivo", l.Motivo, "administradora_id", l.AdministradoraID,
			"condominio_id", l.CondominioID, "sessao", l.SessaoID, "erro", err)
		if aoFalhar != nil {
			aoFalhar(l.Origem)
		}
		perdidas = append(perdidas, err)
	}
	if err := errors.Join(perdidas...); err != nil {
		return fmt.Errorf("gravar auditoria: %w", err)
	}
	return nil
}

func enviar(ctx context.Context, b banco.Banco, linhas []Linha) error {
	lote := banco.NovoLote()
	NoLote(lote, linhas)
	return lote.Enviar(ctx, b.Pool)
}

func paramsDe(linhas []Linha) sqlc.InserirAuditoriaParams {
	n := len(linhas)
	p := sqlc.InserirAuditoriaParams{
		CriadoEm: make([]pgtype.Timestamptz, 0, n), Origem: make([]string, 0, n), Evento: make([]string, 0, n),
		Gancho: make([]string, 0, n), RegraID: make([]string, 0, n), RegraVersao: make([]int64, 0, n),
		Decisao: make([]string, 0, n), Motivo: make([]string, 0, n), AdministradoraID: make([]string, 0, n),
		CondominioID: make([]string, 0, n), SessaoID: make([]string, 0, n), Protocolo: make([]string, 0, n),
		UsuarioID: make([]string, 0, n), Tela: make([]string, 0, n), Ferramenta: make([]string, 0, n),
		Quantidade: make([]int64, 0, n),
	}
	for _, l := range linhas {
		p.CriadoEm = append(p.CriadoEm, pgtype.Timestamptz{Time: l.CriadoEm, Valid: true})
		p.Origem = append(p.Origem, l.Origem)
		p.Evento = append(p.Evento, l.Evento)
		p.Gancho = append(p.Gancho, l.Gancho)
		p.RegraID = append(p.RegraID, l.RegraID)
		p.RegraVersao = append(p.RegraVersao, int64(l.RegraVersao))
		p.Decisao = append(p.Decisao, l.Decisao)
		p.Motivo = append(p.Motivo, l.Motivo)
		p.AdministradoraID = append(p.AdministradoraID, l.AdministradoraID)
		p.CondominioID = append(p.CondominioID, l.CondominioID)
		p.SessaoID = append(p.SessaoID, l.SessaoID)
		p.Protocolo = append(p.Protocolo, l.Protocolo)
		p.UsuarioID = append(p.UsuarioID, l.UsuarioID)
		p.Tela = append(p.Tela, l.Tela)
		p.Ferramenta = append(p.Ferramenta, l.Ferramenta)
		p.Quantidade = append(p.Quantidade, int64(l.Quantidade))
	}
	return p
}
