package dados

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"go-concierge/internal/acesso"
	adminpg "go-concierge/internal/core/dados/postgres/admin"
	atende "go-concierge/internal/harness/atendimento"
	admin "go-concierge/internal/harness/configuracao"
	"go-concierge/internal/identidade"
)

const rotaEdicaoAdministracao = "/interno/dados/v1/administracao/editar"

type pedidoEdicaoAdministracao struct {
	Operacao   string
	Prova      string
	Acao       string
	Alvo       admin.Alvo
	Orientacao admin.Orientacao
	Recurso    string
	Payload    map[string]any
	Ajustes    map[string]map[string]any
}

func registrarEdicaoAdministracao(mux *http.ServeMux, store admin.Repositorio, acessos *AcessosPrivados, token string) {
	mux.HandleFunc("POST "+rotaEdicaoAdministracao, func(w http.ResponseWriter, r *http.Request) {
		var p pedidoEdicaoAdministracao
		if !lerPedido(w, r, token, limiteTurno, &p) {
			return
		}
		res := respostaAdministracao{Operacao: p.Operacao, Resultado: "recusada"}
		status := http.StatusForbidden
		defer func() {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(res)
		}()
		if _, err := uuid.Parse(p.Operacao); err != nil {
			return
		}
		ctx, cancelar := context.WithTimeout(r.Context(), prazoAdministracao)
		defer cancelar()
		principal, err := acessos.carregarPrincipal(ctx, p.Prova)
		if err != nil {
			if !errors.Is(err, atende.ErrEscopoSessao) {
				res.Resultado, status = "indisponivel", http.StatusServiceUnavailable
			}
			return
		}
		var condo *int
		if p.Alvo.Condominio != "" && p.Alvo.Condominio != admin.Padrao {
			n, e := strconv.Atoi(p.Alvo.Condominio)
			if e != nil || strconv.Itoa(n) != p.Alvo.Condominio {
				return
			}
			condo = &n
		}
		alvo, err := alvoAdministracao(p.Alvo.Dona, condo)
		permissao := acesso.CondominiosHabilitar
		if p.Acao == "payload" {
			permissao = acesso.FerramentasEditar
		}
		if err != nil || !acesso.Permitido(principal, permissao, alvo) {
			return
		}
		var m admin.Mapeamento
		switch p.Acao {
		case "orientacao":
			m, err = store.SalvarOrientacao(ctx, p.Alvo, p.Orientacao)
		case "payload":
			if strings.TrimSpace(p.Recurso) == "" {
				return
			}
			m, err = store.SalvarPayload(ctx, p.Alvo, p.Recurso, p.Payload)
		case "ajustes":
			for recurso := range p.Ajustes {
				if strings.TrimSpace(recurso) == "" {
					return
				}
			}
			m, err = store.SalvarAjustes(ctx, p.Alvo, p.Ajustes)
		default:
			return
		}
		if err != nil {
			switch {
			case errors.Is(err, admin.ErrInvalido):
				res.Resultado, status = "invalida", http.StatusUnprocessableEntity
			case errors.Is(err, admin.ErrConflitoConcorrente):
				res.Resultado, status = "conflito", http.StatusConflict
			case errors.Is(err, admin.ErrNaoEncontrado):
				res.Resultado, status = "ausente", http.StatusNotFound
			default:
				res.Resultado, status = "incerta", http.StatusInternalServerError
			}
			return
		}
		res.Resultado, res.Mapeamento, status = "aplicada", &m, http.StatusOK
	})
}

func (a *AdministracaoHTTP) editar(ctx context.Context, p pedidoEdicaoAdministracao) (admin.Mapeamento, error) {
	prova, ok := identidade.Prova(ctx)
	if !ok {
		return admin.Mapeamento{}, atende.ErrEscopoSessao
	}
	op, err := uuid.NewRandom()
	if err != nil {
		return admin.Mapeamento{}, errors.Join(atende.ErrGravacaoNaoAplicada, err)
	}
	p.Prova, p.Operacao = prova, op.String()
	var res respostaAdministracao
	c := a.cliente
	status, enviada, err := requisitarJSON(ctx, c.http, c.base+rotaEdicaoAdministracao, c.token, p, &res, limiteTurno, limiteTurno, prazoAdministracao)
	if err == nil && res.Operacao == p.Operacao {
		if status == http.StatusOK && res.Resultado == "aplicada" && res.Mapeamento != nil {
			return *res.Mapeamento, nil
		}
		switch res.Resultado {
		case "recusada":
			return admin.Mapeamento{}, errors.Join(atende.ErrGravacaoNaoAplicada, atende.ErrEscopoSessao)
		case "invalida":
			return admin.Mapeamento{}, admin.ErrInvalido
		case "conflito":
			return admin.Mapeamento{}, admin.ErrConflitoConcorrente
		case "ausente":
			return admin.Mapeamento{}, admin.ErrNaoEncontrado
		case "indisponivel":
			return admin.Mapeamento{}, errors.Join(atende.ErrGravacaoNaoAplicada, ErrDadosIndisponiveis)
		}
	}
	if !enviada {
		return admin.Mapeamento{}, errors.Join(atende.ErrGravacaoNaoAplicada, err)
	}
	return admin.Mapeamento{}, erroIncerto(op, err)
}

func (a *AdministracaoHTTP) SalvarOrientacao(ctx context.Context, alvo admin.Alvo, o admin.Orientacao) (admin.Mapeamento, error) {
	return a.editar(ctx, pedidoEdicaoAdministracao{Acao: "orientacao", Alvo: alvo, Orientacao: o})
}
func (a *AdministracaoHTTP) SalvarPayload(ctx context.Context, alvo admin.Alvo, recurso string, payload map[string]any) (admin.Mapeamento, error) {
	return a.editar(ctx, pedidoEdicaoAdministracao{Acao: "payload", Alvo: alvo, Recurso: recurso, Payload: payload})
}
func (a *AdministracaoHTTP) SalvarAjustes(ctx context.Context, alvo admin.Alvo, ajustes map[string]map[string]any) (admin.Mapeamento, error) {
	return a.editar(ctx, pedidoEdicaoAdministracao{Acao: "ajustes", Alvo: alvo, Ajustes: ajustes})
}

func (*AdministracaoHTTP) SalvarAdministradora(context.Context, admin.Administradora) (admin.Mapeamento, error) {
	return admin.Mapeamento{}, adminpg.ErrCadastroExterno
}
func (*AdministracaoHTTP) CriarAdministradora(context.Context, admin.Administradora) (admin.Mapeamento, error) {
	return admin.Mapeamento{}, adminpg.ErrCadastroExterno
}
func (*AdministracaoHTTP) EditarAdministradora(context.Context, admin.Administradora) (admin.Mapeamento, error) {
	return admin.Mapeamento{}, adminpg.ErrCadastroExterno
}
func (*AdministracaoHTTP) RemoverAdministradora(context.Context, string) (admin.Mapeamento, error) {
	return admin.Mapeamento{}, adminpg.ErrCadastroExterno
}
func (*AdministracaoHTTP) SalvarCondominio(context.Context, admin.Condominio) (admin.Mapeamento, error) {
	return admin.Mapeamento{}, adminpg.ErrCadastroExterno
}
func (*AdministracaoHTTP) CriarCondominio(context.Context, admin.Condominio) (admin.Mapeamento, error) {
	return admin.Mapeamento{}, adminpg.ErrCadastroExterno
}
func (*AdministracaoHTTP) EditarCondominio(context.Context, admin.Condominio, admin.Chaves) (admin.Mapeamento, error) {
	return admin.Mapeamento{}, adminpg.ErrCadastroExterno
}
func (*AdministracaoHTTP) RemoverCondominio(context.Context, admin.Alvo) (admin.Mapeamento, error) {
	return admin.Mapeamento{}, adminpg.ErrCadastroExterno
}
func (*AdministracaoHTTP) SalvarHabilitacao(context.Context, admin.Alvo, string, admin.Habilitacao, bool) (admin.Mapeamento, error) {
	return admin.Mapeamento{}, adminpg.ErrCadastroExterno
}
