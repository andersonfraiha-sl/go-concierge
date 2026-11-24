package dados

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"go-concierge/internal/core/banco"
	atende "go-concierge/internal/harness/atendimento"
	"go-concierge/internal/harness/canais"
	"go-concierge/internal/identidade"
)

const rotaCarregarSessao = "/interno/dados/v1/sessoes/carregar"
const rotaSalvarSessao = "/interno/dados/v1/sessoes/salvar"
const limiteSessao = 64 << 20
const prazoSessao = 3 * time.Second

type pedidoSessao struct {
	Operacao string                 `json:"operacao"`
	Empresa  string                 `json:"empresa"`
	Canal    uuid.UUID              `json:"canal"`
	ID       string                 `json:"id"`
	Registro *atende.RegistroSessao `json:"registro,omitempty"`
}

type respostaSessao struct {
	Operacao   string                 `json:"operacao"`
	Resultado  string                 `json:"resultado"`
	Encontrada bool                   `json:"encontrada"`
	Registro   *atende.RegistroSessao `json:"registro,omitempty"`
}

type SessoesRoteadas struct {
	cliente *Cliente
}

func NovasSessoes(cliente *Cliente) *SessoesRoteadas {
	return &SessoesRoteadas{cliente: cliente}
}

func (s *SessoesRoteadas) Salvar(ctx context.Context, r atende.RegistroSessao) error {
	if _, ok := identidade.Usuario(ctx); ok {
		_, err := s.cliente.sessaoAdmin(ctx, pedidoSessaoAdmin{Acao: "salvar", ID: r.ID, Registro: &r}, true)
		return err
	}
	return s.cliente.salvarSessao(ctx, r)
}

func (s *SessoesRoteadas) Carregar(ctx context.Context, id string) (atende.RegistroSessao, bool, error) {
	if _, ok := identidade.Usuario(ctx); ok {
		res, err := s.cliente.sessaoAdmin(ctx, pedidoSessaoAdmin{Acao: "carregar", ID: id}, false)
		if err != nil {
			return atende.RegistroSessao{}, false, err
		}
		if res.Encontrada != (res.Registro != nil) {
			return atende.RegistroSessao{}, false, ErrDadosIndisponiveis
		}
		if !res.Encontrada {
			return atende.RegistroSessao{}, false, nil
		}
		if res.Registro.ID != id || atende.ValidarEscopoSessao(ctx, *res.Registro) != nil || atende.ValidarRegistroSessao(*res.Registro) != nil {
			return atende.RegistroSessao{}, false, atende.ErrEscopoSessao
		}
		return *res.Registro, true, nil
	}
	return s.cliente.carregarSessao(ctx, id)
}

func (s *SessoesRoteadas) Listar(ctx context.Context, empresa string, limite int) ([]atende.RegistroSessao, error) {
	if _, ok := identidade.Usuario(ctx); !ok {
		return nil, atende.ErrEscopoSessao
	}
	res, err := s.cliente.sessaoAdmin(ctx, pedidoSessaoAdmin{Acao: "listar", Empresa: empresa, Limite: limite}, false)
	if err != nil {
		return nil, err
	}
	if res.Registros == nil {
		return nil, ErrDadosIndisponiveis
	}
	for _, r := range res.Registros {
		if r.CompanyID != empresa || atende.ValidarEscopoSessao(ctx, r) != nil || atende.ValidarRegistroSessao(r) != nil {
			return nil, atende.ErrEscopoSessao
		}
	}
	return res.Registros, nil
}

func (s *SessoesRoteadas) Remover(ctx context.Context, id string) error {
	if _, ok := identidade.Usuario(ctx); !ok {
		return atende.ErrEscopoSessao
	}
	_, err := s.cliente.sessaoAdmin(ctx, pedidoSessaoAdmin{Acao: "remover", ID: id}, true)
	return err
}

func registrarSessoes(mux *http.ServeMux, repo canais.Repositorio, store atende.SessaoStore, token string) {
	mux.HandleFunc("POST "+rotaCarregarSessao, func(w http.ResponseWriter, r *http.Request) {
		var p pedidoSessao
		if !lerPedido(w, r, token, limiteSessao, &p) {
			return
		}
		if !autorizarCanalSessao(w, r, repo, p, false) {
			return
		}
		if strings.TrimSpace(p.ID) == "" || p.Registro != nil {
			responderSessao(w, respostaSessao{Operacao: p.Operacao, Resultado: "invalida"}, http.StatusBadRequest)
			return
		}
		ctx := atende.ComEscopoSessao(r.Context(), p.Empresa, p.Canal)
		registro, achou, err := store.Carregar(ctx, p.ID)
		if err != nil {
			responderErroSessao(w, p.Operacao, err, false)
			return
		}
		res := respostaSessao{Operacao: p.Operacao, Resultado: "lida", Encontrada: achou}
		if achou {
			if atende.ValidarEscopoSessao(ctx, registro) != nil {
				responderSessao(w, respostaSessao{Operacao: p.Operacao, Resultado: "recusada"}, http.StatusForbidden)
				return
			}
			res.Registro = &registro
		}
		responderSessao(w, res, http.StatusOK)
	})
	mux.HandleFunc("POST "+rotaSalvarSessao, func(w http.ResponseWriter, r *http.Request) {
		var p pedidoSessao
		if !lerPedido(w, r, token, limiteSessao, &p) {
			return
		}
		if !autorizarCanalSessao(w, r, repo, p, true) {
			return
		}
		ctx := atende.ComEscopoSessao(r.Context(), p.Empresa, p.Canal)
		if p.Registro == nil || p.ID != p.Registro.ID || atende.ValidarEscopoSessao(ctx, *p.Registro) != nil || atende.ValidarRegistroSessao(*p.Registro) != nil {
			responderSessao(w, respostaSessao{Operacao: p.Operacao, Resultado: "recusada"}, http.StatusForbidden)
			return
		}
		if err := store.Salvar(ctx, *p.Registro); err != nil {
			responderErroSessao(w, p.Operacao, err, true)
			return
		}
		responderSessao(w, respostaSessao{Operacao: p.Operacao, Resultado: "aplicada"}, http.StatusOK)
	})
}

func autorizarCanalSessao(w http.ResponseWriter, r *http.Request, repo canais.Repositorio, p pedidoSessao, escrita bool) bool {
	if _, err := uuid.Parse(p.Operacao); err != nil || strings.TrimSpace(p.Empresa) == "" || p.Canal == uuid.Nil {
		responderSessao(w, respostaSessao{Operacao: p.Operacao, Resultado: "recusada"}, http.StatusForbidden)
		return false
	}
	_, err := repo.PorID(r.Context(), p.Empresa, p.Canal)
	if err != nil {
		if escrita {
			err = errors.Join(atende.ErrGravacaoNaoAplicada, err)
		}
		responderErroSessao(w, p.Operacao, err, escrita)
		return false
	}
	return true
}

func responderErroSessao(w http.ResponseWriter, operacao string, err error, escrita bool) {
	switch {
	case errors.Is(err, banco.ErrCommitIncerto) || errors.Is(err, atende.ErrGravacaoIncerta):
		responderSessao(w, respostaSessao{Operacao: operacao, Resultado: "incerta"}, http.StatusInternalServerError)
	case errors.Is(err, canais.ErrDesconhecido) || errors.Is(err, atende.ErrEscopoSessao) || errors.Is(err, atende.ErrRegistroSessao) || errors.Is(err, atende.ErrGravacaoRecusada):
		responderSessao(w, respostaSessao{Operacao: operacao, Resultado: "recusada"}, http.StatusForbidden)
	case escrita && errors.Is(err, atende.ErrGravacaoNaoAplicada):
		responderSessao(w, respostaSessao{Operacao: operacao, Resultado: "nao_aplicada"}, http.StatusUnprocessableEntity)
	case escrita:
		responderSessao(w, respostaSessao{Operacao: operacao, Resultado: "incerta"}, http.StatusInternalServerError)
	default:
		responderSessao(w, respostaSessao{Operacao: operacao, Resultado: "indisponivel"}, http.StatusServiceUnavailable)
	}
}

func responderSessao(w http.ResponseWriter, resposta respostaSessao, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resposta)
}

func escopoDaSessao(ctx context.Context) (string, uuid.UUID, error) {
	empresa, canal, ok := atende.EscopoSessaoDoContexto(ctx)
	if !ok || canal == uuid.Nil {
		return "", uuid.Nil, atende.ErrEscopoSessao
	}
	return empresa, canal, nil
}

func (c *Cliente) carregarSessao(ctx context.Context, id string) (atende.RegistroSessao, bool, error) {
	empresa, canal, err := escopoDaSessao(ctx)
	if err != nil {
		return atende.RegistroSessao{}, false, err
	}
	op, err := uuid.NewRandom()
	if err != nil {
		return atende.RegistroSessao{}, false, errors.Join(ErrDadosIndisponiveis, err)
	}
	p := pedidoSessao{Operacao: op.String(), Empresa: empresa, Canal: canal, ID: id}
	var res respostaSessao
	status, _, err := requisitarJSON(ctx, c.http, c.base+rotaCarregarSessao, c.token, p, &res, limiteSessao, limiteSessao, prazoSessao)
	if err != nil || res.Operacao != p.Operacao {
		return atende.RegistroSessao{}, false, errors.Join(ErrDadosIndisponiveis, err)
	}
	if status == http.StatusForbidden && res.Resultado == "recusada" {
		return atende.RegistroSessao{}, false, atende.ErrEscopoSessao
	}
	if status != http.StatusOK || res.Resultado != "lida" || res.Encontrada != (res.Registro != nil) {
		return atende.RegistroSessao{}, false, fmt.Errorf("%w: resposta de sessão inválida", ErrDadosIndisponiveis)
	}
	if !res.Encontrada {
		return atende.RegistroSessao{}, false, nil
	}
	if res.Registro.ID != id || atende.ValidarEscopoSessao(ctx, *res.Registro) != nil {
		return atende.RegistroSessao{}, false, atende.ErrEscopoSessao
	}
	if err := atende.ValidarRegistroSessao(*res.Registro); err != nil {
		return atende.RegistroSessao{}, false, err
	}
	return *res.Registro, true, nil
}

func (c *Cliente) salvarSessao(ctx context.Context, r atende.RegistroSessao) error {
	empresa, canal, err := escopoDaSessao(ctx)
	if err != nil {
		return errors.Join(atende.ErrGravacaoNaoAplicada, atende.ErrGravacaoRecusada, err)
	}
	op, err := uuid.NewRandom()
	if err != nil {
		return errors.Join(atende.ErrGravacaoNaoAplicada, err)
	}
	p := pedidoSessao{Operacao: op.String(), Empresa: empresa, Canal: canal, ID: r.ID, Registro: &r}
	var res respostaSessao
	status, enviada, err := requisitarJSON(ctx, c.http, c.base+rotaSalvarSessao, c.token, p, &res, limiteSessao, limiteSessao, prazoSessao)
	if err != nil {
		if !enviada {
			return errors.Join(atende.ErrGravacaoNaoAplicada, err)
		}
		return erroIncerto(op, err)
	}
	if res.Operacao != p.Operacao {
		return erroIncerto(op, errors.New("resposta de sessão inválida"))
	}
	switch {
	case status == http.StatusOK && res.Resultado == "aplicada":
		return nil
	case status == http.StatusForbidden && res.Resultado == "recusada":
		return errors.Join(atende.ErrGravacaoNaoAplicada, atende.ErrGravacaoRecusada, atende.ErrEscopoSessao)
	case status == http.StatusUnprocessableEntity && res.Resultado == "nao_aplicada":
		return atende.ErrGravacaoNaoAplicada
	default:
		return erroIncerto(op, fmt.Errorf("HTTP %d", status))
	}
}
