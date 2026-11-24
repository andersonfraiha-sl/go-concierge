package dados

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"go-concierge/internal/core/banco"
	auditoriapg "go-concierge/internal/core/dados/postgres/auditoria"
	"go-concierge/internal/escopo"
	atende "go-concierge/internal/harness/atendimento"
	"go-concierge/internal/harness/ganchos/auditoria"
	"go-concierge/internal/identidade"
)

const rotaGravarAuditoria = "/interno/dados/v1/auditoria/gravar"
const limiteAuditoria = 64 << 20
const prazoAuditoria = 2 * time.Second

type OperacoesAuditoria interface {
	Gravar(context.Context, string, uuid.UUID, []auditoria.Linha) error
}

type AuditoriaPostgres struct {
	banco    banco.Banco
	mu       sync.RWMutex
	aoFalhar func(string)
}

func NovaAuditoriaPostgres(b banco.Banco) *AuditoriaPostgres {
	return &AuditoriaPostgres{banco: b}
}

func (a *AuditoriaPostgres) AoFalhar(fn func(string)) {
	a.mu.Lock()
	a.aoFalhar = fn
	a.mu.Unlock()
}

func (a *AuditoriaPostgres) Gravar(ctx context.Context, empresa string, canal uuid.UUID, linhas []auditoria.Linha) error {
	if !escopo.EmpresaValida(empresa) || canal == uuid.Nil {
		return atende.ErrEscopoSessao
	}
	return a.gravar(ctx, linhas, func(ctx context.Context, tx pgx.Tx, linhas []auditoria.Linha) error {
		var condo pgtype.Int4
		err := tx.QueryRow(ctx, `SELECT condo_id FROM channels WHERE company_id=$1 AND id=$2 FOR SHARE`, empresa, canal).Scan(&condo)
		if errors.Is(err, pgx.ErrNoRows) {
			return atende.ErrEscopoSessao
		}
		if err != nil {
			return err
		}
		if err := validarLinhasAuditoria(empresa, canal, condo, linhas); err != nil {
			return err
		}
		for i := range linhas {
			if linhas[i].AdministradoraID == "" {
				linhas[i].AdministradoraID = empresa
			}
			if linhas[i].ChannelID == "" {
				linhas[i].ChannelID = canal.String()
			}
		}
		return nil
	})
}

func (a *AuditoriaPostgres) gravar(ctx context.Context, linhas []auditoria.Linha, validar func(context.Context, pgx.Tx, []auditoria.Linha) error) error {
	a.mu.RLock()
	aoFalhar := a.aoFalhar
	a.mu.RUnlock()
	aplicou := false
	enviar := func(ctx context.Context, linhas []auditoria.Linha) error {
		tx, err := a.banco.Pool.Begin(ctx)
		if err != nil {
			return err
		}
		defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
		linhas = slices.Clone(linhas)
		if err := validar(ctx, tx, linhas); err != nil {
			return err
		}
		lote := banco.NovoLote()
		auditoriapg.NoLote(lote, linhas)
		if err := lote.Executar(ctx, tx); err != nil {
			return err
		}
		err = banco.Confirmar(ctx, tx)
		if err == nil {
			aplicou = true
		}
		return err
	}
	err := auditoriapg.GravarCom(ctx, linhas, aoFalhar, enviar, func(err error) bool {
		return !errors.Is(err, atende.ErrEscopoSessao)
	})
	if err != nil && aplicou {
		return errors.Join(atende.ErrGravacaoIncerta, err)
	}
	return err
}

func validarLinhasAuditoria(empresa string, canal uuid.UUID, condo pgtype.Int4, linhas []auditoria.Linha) error {
	for _, linha := range linhas {
		if linha.AdministradoraID != "" && linha.AdministradoraID != empresa || linha.ChannelID != "" && linha.ChannelID != canal.String() {
			return atende.ErrEscopoSessao
		}
		if linha.CondominioID != "" {
			id, err := strconv.Atoi(linha.CondominioID)
			if err != nil || !escopo.CondominioValido(&id) || condo.Valid && int64(id) != int64(condo.Int32) {
				return atende.ErrEscopoSessao
			}
		}
	}
	return nil
}

type pedidoAuditoria struct {
	Operacao string            `json:"operacao"`
	Empresa  string            `json:"empresa"`
	Canal    uuid.UUID         `json:"canal"`
	Linhas   []auditoria.Linha `json:"linhas"`
}

type respostaAuditoria struct {
	Operacao  string `json:"operacao"`
	Resultado string `json:"resultado"`
}

func registrarAuditoria(mux *http.ServeMux, store OperacoesAuditoria, token string) {
	mux.HandleFunc("POST "+rotaGravarAuditoria, func(w http.ResponseWriter, r *http.Request) {
		var p pedidoAuditoria
		if !lerPedido(w, r, token, limiteAuditoria, &p) {
			return
		}
		resultado, status := "aplicada", http.StatusOK
		if _, err := uuid.Parse(p.Operacao); err != nil || !escopo.EmpresaValida(p.Empresa) || p.Canal == uuid.Nil {
			resultado, status = "recusada", http.StatusForbidden
		} else {
			ctx, cancelar := context.WithTimeout(r.Context(), prazoAuditoria)
			defer cancelar()
			err := store.Gravar(ctx, p.Empresa, p.Canal, p.Linhas)
			if errors.Is(err, atende.ErrEscopoSessao) && !errors.Is(err, atende.ErrGravacaoIncerta) && !errors.Is(err, banco.ErrCommitIncerto) {
				resultado, status = "recusada", http.StatusForbidden
			} else if err != nil {
				resultado, status = "incerta", http.StatusInternalServerError
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(respostaAuditoria{Operacao: p.Operacao, Resultado: resultado})
	})
}

func (c *Cliente) GravarAuditoria(ctx context.Context, linhas []auditoria.Linha) error {
	if len(linhas) == 0 {
		return nil
	}
	if _, ok := identidade.Usuario(ctx); ok {
		return c.gravarAuditoriaAdmin(ctx, linhas)
	}
	empresa, canal, err := escopoDaSessao(ctx)
	if err != nil {
		return errors.Join(atende.ErrGravacaoNaoAplicada, err)
	}
	op, err := uuid.NewRandom()
	if err != nil {
		return errors.Join(atende.ErrGravacaoNaoAplicada, err)
	}
	p := pedidoAuditoria{Operacao: op.String(), Empresa: empresa, Canal: canal, Linhas: linhas}
	var res respostaAuditoria
	status, enviada, err := requisitarJSON(ctx, c.http, c.base+rotaGravarAuditoria, c.token, p, &res, limiteAuditoria, limiteAuditoria, prazoAuditoria)
	if err != nil {
		if !enviada {
			return errors.Join(atende.ErrGravacaoNaoAplicada, err)
		}
		return erroIncerto(op, err)
	}
	if res.Operacao != p.Operacao {
		return erroIncerto(op, errors.New("resposta de auditoria inválida"))
	}
	if status == http.StatusOK && res.Resultado == "aplicada" {
		return nil
	}
	if status == http.StatusForbidden && res.Resultado == "recusada" {
		return errors.Join(atende.ErrGravacaoNaoAplicada, atende.ErrGravacaoRecusada, atende.ErrEscopoSessao)
	}
	return erroIncerto(op, fmt.Errorf("HTTP %d", status))
}
