package dados

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"go-concierge/internal/acesso"
	"go-concierge/internal/core/banco"
	atende "go-concierge/internal/harness/atendimento"
	"go-concierge/internal/identidade"
)

const rotaSessaoAdmin = "/interno/dados/v1/sessoes/administrar"

type sessoesAutorizadas interface {
	SalvarAutorizado(context.Context, atende.RegistroSessao, func(atende.RegistroSessao, bool) error) error
	RemoverAutorizado(context.Context, string, func(atende.RegistroSessao, bool) error) error
}

type pedidoSessaoAdmin struct {
	Operacao  string                 `json:"operacao"`
	Acao      string                 `json:"acao"`
	Prova     string                 `json:"prova"`
	Permissao acesso.Permissao       `json:"permissao"`
	Empresa   string                 `json:"empresa"`
	ID        string                 `json:"id,omitempty"`
	Limite    int                    `json:"limite,omitempty"`
	Registro  *atende.RegistroSessao `json:"registro,omitempty"`
}

type respostaSessaoAdmin struct {
	Operacao   string                  `json:"operacao"`
	Resultado  string                  `json:"resultado"`
	Encontrada bool                    `json:"encontrada,omitempty"`
	Registro   *atende.RegistroSessao  `json:"registro,omitempty"`
	Registros  []atende.RegistroSessao `json:"registros"`
}

func permissaoSessaoValida(acao string, p acesso.Permissao) bool {
	switch acao {
	case "carregar":
		return p == acesso.ConversasVer || p == acesso.ConversasAtender || p == acesso.PedidosEditar || p == acesso.StudioAvancado
	case "listar":
		return p == acesso.ConversasVer
	case "salvar":
		return p == acesso.ConversasAtender || p == acesso.PedidosEditar
	case "remover":
		return p == acesso.ConversasAtender
	}
	return false
}

func alvoSessao(r atende.RegistroSessao) acesso.Escopo {
	if r.CondoID != nil {
		return acesso.EscopoCondominio(r.CompanyID, strconv.Itoa(*r.CondoID))
	}
	return acesso.Escopo{Tipo: acesso.TipoAdministradora, ID: r.CompanyID}
}

func registrarSessoesAdmin(mux *http.ServeMux, store atende.SessaoStore, acessos *AcessosPrivados, token string) {
	mux.HandleFunc("POST "+rotaSessaoAdmin, func(w http.ResponseWriter, r *http.Request) {
		var p pedidoSessaoAdmin
		if !lerPedido(w, r, token, limiteSessao, &p) {
			return
		}
		res := respostaSessaoAdmin{Operacao: p.Operacao}
		if _, err := uuid.Parse(p.Operacao); err != nil || strings.TrimSpace(p.Empresa) == "" || !permissaoSessaoValida(p.Acao, p.Permissao) {
			responderSessaoAdmin(w, res, "recusada", http.StatusForbidden)
			return
		}
		ctx, cancelar := context.WithTimeout(r.Context(), prazoSessao)
		defer cancelar()
		ctx = atende.ComEscopoSessao(ctx, p.Empresa, uuid.Nil)
		a, err := acessos.carregarPrincipal(ctx, p.Prova)
		if err != nil {
			escrita := p.Acao == "salvar" || p.Acao == "remover"
			if escrita {
				err = errors.Join(atende.ErrGravacaoNaoAplicada, err)
			}
			responderErroSessaoAdmin(w, res, err, escrita)
			return
		}
		permitir := func(reg atende.RegistroSessao) error {
			if reg.CompanyID != p.Empresa || !acesso.Permitido(a, p.Permissao, alvoSessao(reg)) {
				return atende.ErrEscopoSessao
			}
			return nil
		}
		switch p.Acao {
		case "carregar":
			if strings.TrimSpace(p.ID) == "" || p.Registro != nil {
				responderSessaoAdmin(w, res, "recusada", http.StatusForbidden)
				return
			}
			reg, achou, err := store.Carregar(ctx, p.ID)
			if err == nil && achou && permitir(reg) != nil {
				achou = false
			}
			if err != nil {
				responderErroSessaoAdmin(w, res, err, false)
				return
			}
			res.Encontrada = achou
			if achou {
				res.Registro = &reg
			}
			responderSessaoAdmin(w, res, "lida", http.StatusOK)
		case "listar":
			if p.ID != "" || p.Registro != nil || p.Limite < 0 || p.Limite > 1000 || !acesso.Permitido(a, p.Permissao, acesso.Escopo{Tipo: acesso.TipoAdministradora, ID: p.Empresa}) {
				responderSessaoAdmin(w, res, "recusada", http.StatusForbidden)
				return
			}
			regs, err := store.Listar(ctx, p.Empresa, p.Limite)
			if err != nil {
				responderErroSessaoAdmin(w, res, err, false)
				return
			}
			for _, reg := range regs {
				if err := permitir(reg); err != nil {
					responderErroSessaoAdmin(w, res, err, false)
					return
				}
			}
			if regs == nil {
				regs = []atende.RegistroSessao{}
			}
			res.Registros = regs
			responderSessaoAdmin(w, res, "listada", http.StatusOK)
		case "salvar", "remover":
			transacional, ok := store.(sessoesAutorizadas)
			if !ok {
				responderErroSessaoAdmin(w, res, errors.New("store transacional indisponível"), true)
				return
			}
			if strings.TrimSpace(p.ID) == "" || p.Acao == "remover" && p.Registro != nil || p.Acao == "salvar" && (p.Registro == nil || p.ID != p.Registro.ID) {
				responderSessaoAdmin(w, res, "recusada", http.StatusForbidden)
				return
			}
			if p.Registro != nil {
				if err := permitir(*p.Registro); err != nil {
					responderErroSessaoAdmin(w, res, err, true)
					return
				}
			}
			autorizar := func(atual atende.RegistroSessao, achou bool) error {
				if !achou {
					return nil
				}
				return permitir(atual)
			}
			if p.Acao == "salvar" {
				err = transacional.SalvarAutorizado(ctx, *p.Registro, autorizar)
			} else {
				err = transacional.RemoverAutorizado(ctx, p.ID, autorizar)
			}
			if err != nil {
				responderErroSessaoAdmin(w, res, err, true)
				return
			}
			responderSessaoAdmin(w, res, "aplicada", http.StatusOK)
		}
	})
}

func responderSessaoAdmin(w http.ResponseWriter, r respostaSessaoAdmin, resultado string, status int) {
	r.Resultado = resultado
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(r)
}

func responderErroSessaoAdmin(w http.ResponseWriter, r respostaSessaoAdmin, err error, escrita bool) {
	if escrita && (errors.Is(err, banco.ErrCommitIncerto) || errors.Is(err, atende.ErrGravacaoIncerta)) {
		responderSessaoAdmin(w, r, "incerta", http.StatusInternalServerError)
	} else if errors.Is(err, atende.ErrEscopoSessao) || errors.Is(err, atende.ErrRegistroSessao) || errors.Is(err, atende.ErrGravacaoRecusada) {
		responderSessaoAdmin(w, r, "recusada", http.StatusForbidden)
	} else if escrita && errors.Is(err, atende.ErrGravacaoNaoAplicada) {
		responderSessaoAdmin(w, r, "nao_aplicada", http.StatusUnprocessableEntity)
	} else if escrita {
		responderSessaoAdmin(w, r, "incerta", http.StatusInternalServerError)
	} else {
		responderSessaoAdmin(w, r, "indisponivel", http.StatusServiceUnavailable)
	}
}

func (c *Cliente) sessaoAdmin(ctx context.Context, p pedidoSessaoAdmin, escrita bool) (respostaSessaoAdmin, error) {
	prova, ok := identidade.Prova(ctx)
	if !ok {
		return respostaSessaoAdmin{}, errors.Join(atende.ErrGravacaoNaoAplicada, atende.ErrEscopoSessao)
	}
	permissao, ok := identidade.Permissao(ctx)
	if !ok {
		return respostaSessaoAdmin{}, errors.Join(atende.ErrGravacaoNaoAplicada, atende.ErrEscopoSessao)
	}
	if p.Empresa == "" {
		var err error
		p.Empresa, _, ok = atende.EscopoSessaoDoContexto(ctx)
		if !ok {
			err = atende.ErrEscopoSessao
		}
		if err != nil {
			return respostaSessaoAdmin{}, errors.Join(atende.ErrGravacaoNaoAplicada, err)
		}
	}
	op, err := uuid.NewRandom()
	if err != nil {
		return respostaSessaoAdmin{}, errors.Join(atende.ErrGravacaoNaoAplicada, err)
	}
	p.Operacao, p.Prova, p.Permissao = op.String(), prova, acesso.Permissao(permissao)
	var res respostaSessaoAdmin
	status, enviada, err := requisitarJSON(ctx, c.http, c.base+rotaSessaoAdmin, c.token, p, &res, limiteSessao, limiteSessao, prazoSessao)
	if err != nil {
		if escrita && enviada {
			return res, erroIncerto(op, err)
		}
		if escrita {
			return res, errors.Join(atende.ErrGravacaoNaoAplicada, err)
		}
		return res, errors.Join(ErrDadosIndisponiveis, err)
	}
	if res.Operacao != p.Operacao {
		if escrita {
			return res, erroIncerto(op, errors.New("resposta administrativa inválida"))
		}
		return res, ErrDadosIndisponiveis
	}
	if status == http.StatusForbidden && res.Resultado == "recusada" {
		return res, errors.Join(atende.ErrGravacaoNaoAplicada, atende.ErrGravacaoRecusada, atende.ErrEscopoSessao)
	}
	if escrita && status == http.StatusUnprocessableEntity && res.Resultado == "nao_aplicada" {
		return res, atende.ErrGravacaoNaoAplicada
	}
	if status != http.StatusOK {
		if escrita {
			return res, erroIncerto(op, fmt.Errorf("HTTP %d", status))
		}
		return res, ErrDadosIndisponiveis
	}
	esperado := map[string]string{"carregar": "lida", "listar": "listada", "salvar": "aplicada", "remover": "aplicada"}[p.Acao]
	if esperado == "" || res.Resultado != esperado {
		if escrita {
			return res, erroIncerto(op, errors.New("resultado administrativo inválido"))
		}
		return res, ErrDadosIndisponiveis
	}
	return res, nil
}
