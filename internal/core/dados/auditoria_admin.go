package dados

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"go-concierge/internal/acesso"
	"go-concierge/internal/escopo"
	atende "go-concierge/internal/harness/atendimento"
	"go-concierge/internal/harness/ganchos/auditoria"
	"go-concierge/internal/identidade"
)

const rotaAuditoriaAdmin = "/interno/dados/v1/auditoria/administrar"

type auditoriaAdministrativa interface {
	GravarAdministrativa(context.Context, acesso.Acesso, acesso.Permissao, []auditoria.Linha) error
}

type pedidoAuditoriaAdmin struct {
	Operacao  string            `json:"operacao"`
	Prova     string            `json:"prova"`
	Permissao acesso.Permissao  `json:"permissao"`
	Linhas    []auditoria.Linha `json:"linhas"`
}

func (a *AuditoriaPostgres) GravarAdministrativa(ctx context.Context, principal acesso.Acesso, permissao acesso.Permissao, linhas []auditoria.Linha) error {
	if permissao != acesso.ConversasAtender && permissao != acesso.PedidosEditar {
		return atende.ErrEscopoSessao
	}
	return a.gravar(ctx, linhas, func(ctx context.Context, tx pgx.Tx, linhas []auditoria.Linha) error {
		for i := range linhas {
			l := &linhas[i]
			if !escopo.EmpresaValida(l.AdministradoraID) {
				return atende.ErrEscopoSessao
			}
			var condo pgtype.Int4
			if l.ChannelID != "" {
				canal, err := uuid.Parse(l.ChannelID)
				if err != nil {
					return atende.ErrEscopoSessao
				}
				err = tx.QueryRow(ctx, `SELECT condo_id FROM channels WHERE company_id=$1 AND id=$2 FOR SHARE`, l.AdministradoraID, canal).Scan(&condo)
				if errors.Is(err, pgx.ErrNoRows) {
					return atende.ErrEscopoSessao
				}
				if err != nil {
					return err
				}
				if err := validarLinhasAuditoria(l.AdministradoraID, canal, condo, []auditoria.Linha{*l}); err != nil {
					return err
				}
			}
			alvo := acesso.Escopo{Tipo: acesso.TipoAdministradora, ID: l.AdministradoraID}
			if l.CondominioID != "" {
				id, err := strconv.Atoi(l.CondominioID)
				if err != nil || !escopo.CondominioValido(&id) {
					return atende.ErrEscopoSessao
				}
				alvo = acesso.EscopoCondominio(l.AdministradoraID, strconv.Itoa(id))
			} else if condo.Valid {
				alvo = acesso.EscopoCondominio(l.AdministradoraID, strconv.Itoa(int(condo.Int32)))
				l.CondominioID = strconv.Itoa(int(condo.Int32))
			}
			if !acesso.Permitido(principal, permissao, alvo) {
				return atende.ErrEscopoSessao
			}
		}
		return nil
	})
}

func registrarAuditoriaAdmin(mux *http.ServeMux, store auditoriaAdministrativa, acessos *AcessosPrivados, token string) {
	mux.HandleFunc("POST "+rotaAuditoriaAdmin, func(w http.ResponseWriter, r *http.Request) {
		var p pedidoAuditoriaAdmin
		if !lerPedido(w, r, token, limiteAuditoria, &p) {
			return
		}
		res := respostaAuditoria{Operacao: p.Operacao, Resultado: "recusada"}
		status := http.StatusForbidden
		ctx, cancelar := context.WithTimeout(r.Context(), prazoAuditoria)
		defer cancelar()
		if _, err := uuid.Parse(p.Operacao); err == nil && (p.Permissao == acesso.ConversasAtender || p.Permissao == acesso.PedidosEditar) {
			principal, err := acessos.carregarPrincipal(ctx, p.Prova)
			if err == nil {
				err = store.GravarAdministrativa(ctx, principal, p.Permissao, p.Linhas)
				if err == nil {
					res.Resultado, status = "aplicada", http.StatusOK
				} else if !errors.Is(err, atende.ErrEscopoSessao) {
					res.Resultado, status = "incerta", http.StatusInternalServerError
				}
			} else if !errors.Is(err, atende.ErrEscopoSessao) {
				res.Resultado, status = "nao_aplicada", http.StatusServiceUnavailable
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(res)
	})
}

func (c *Cliente) gravarAuditoriaAdmin(ctx context.Context, linhas []auditoria.Linha) error {
	prova, ok := identidade.Prova(ctx)
	if !ok {
		return errors.Join(atende.ErrGravacaoNaoAplicada, atende.ErrEscopoSessao)
	}
	permissao, _ := identidade.Permissao(ctx)
	op, err := uuid.NewRandom()
	if err != nil {
		return errors.Join(atende.ErrGravacaoNaoAplicada, err)
	}
	p := pedidoAuditoriaAdmin{Operacao: op.String(), Prova: prova, Permissao: acesso.Permissao(permissao), Linhas: linhas}
	var res respostaAuditoria
	status, enviada, err := requisitarJSON(ctx, c.http, c.base+rotaAuditoriaAdmin, c.token, p, &res, limiteAuditoria, limiteAuditoria, prazoAuditoria)
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
	if status == http.StatusServiceUnavailable && res.Resultado == "nao_aplicada" {
		return errors.Join(atende.ErrGravacaoNaoAplicada, ErrDadosIndisponiveis)
	}
	return erroIncerto(op, errors.New("gravação administrativa sem confirmação"))
}
