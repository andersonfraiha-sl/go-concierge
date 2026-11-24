package dados

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"go-concierge/internal/acesso"
	acessopg "go-concierge/internal/core/dados/postgres/acesso"
	"go-concierge/internal/escopo"
	atende "go-concierge/internal/harness/atendimento"
	"go-concierge/internal/harness/memoria"
	"go-concierge/internal/harness/pedidos"
	"go-concierge/internal/identidade"
)

const rotaCarregarAcesso = "/interno/dados/v1/acessos/carregar"
const limiteAcesso = 64 << 20
const prazoAcesso = 3 * time.Second

type CarregadorAcesso interface {
	Carregar(context.Context, string, escopo.Implantacao) (acesso.Acesso, error)
}

type AcessosPrivados struct {
	verificador *identidade.Verificador
	store       CarregadorAcesso
	implantacao escopo.Implantacao
	memorias    memoria.Store
	pedidos     pedidos.Store
}

func NovosAcessosPrivados(verificador *identidade.Verificador, store CarregadorAcesso, implantacao escopo.Implantacao) *AcessosPrivados {
	return &AcessosPrivados{verificador: verificador, store: store, implantacao: implantacao}
}

func (a *AcessosPrivados) ComContatos(memorias memoria.Store, pedidos pedidos.Store) *AcessosPrivados {
	a.memorias, a.pedidos = memorias, pedidos
	return a
}

func (a *AcessosPrivados) carregarPrincipal(ctx context.Context, prova string) (acesso.Acesso, error) {
	usuario, ok := a.verificador.Verificar(prova)
	if !ok {
		return acesso.Acesso{}, atende.ErrEscopoSessao
	}
	permissoes, err := a.store.Carregar(ctx, usuario, a.implantacao)
	if errors.Is(err, acessopg.ErrUsuarioDesconhecido) {
		return acesso.Acesso{}, atende.ErrEscopoSessao
	}
	if err != nil {
		return acesso.Acesso{}, err
	}
	if permissoes.UsuarioID != usuario || permissoes.Implantacao != a.implantacao {
		return acesso.Acesso{}, atende.ErrEscopoSessao
	}
	return permissoes, nil
}

type pedidoAcesso struct {
	Operacao    string             `json:"operacao"`
	Usuario     string             `json:"usuario"`
	Prova       string             `json:"prova"`
	Implantacao escopo.Implantacao `json:"implantacao"`
}

type respostaAcesso struct {
	Operacao  string         `json:"operacao"`
	Resultado string         `json:"resultado"`
	Acesso    *acesso.Acesso `json:"acesso,omitempty"`
}

func registrarAcessos(mux *http.ServeMux, operacoes *AcessosPrivados, token string) {
	mux.HandleFunc("POST "+rotaCarregarAcesso, func(w http.ResponseWriter, r *http.Request) {
		var p pedidoAcesso
		if !lerPedido(w, r, token, limiteAcesso, &p) {
			return
		}
		usuario, valido := operacoes.verificador.Verificar(p.Prova)
		res := respostaAcesso{Operacao: p.Operacao}
		status := http.StatusOK
		if _, err := uuid.Parse(p.Operacao); err != nil || !valido || usuario != p.Usuario || p.Implantacao != operacoes.implantacao {
			res.Resultado, status = "recusada", http.StatusForbidden
		} else {
			ctx, cancelar := context.WithTimeout(r.Context(), prazoAcesso)
			defer cancelar()
			a, err := operacoes.store.Carregar(ctx, usuario, operacoes.implantacao)
			switch {
			case errors.Is(err, acessopg.ErrUsuarioDesconhecido):
				res.Resultado, status = "desconhecido", http.StatusNotFound
			case err != nil:
				res.Resultado, status = "indisponivel", http.StatusServiceUnavailable
			default:
				res.Resultado, res.Acesso = "carregado", &a
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(res)
	})
}

func (c *Cliente) Carregar(ctx context.Context, usuario string, implantacao escopo.Implantacao) (acesso.Acesso, error) {
	prova, ok := identidade.Prova(ctx)
	if !ok {
		return acesso.Acesso{}, ErrDadosIndisponiveis
	}
	op, err := uuid.NewRandom()
	if err != nil {
		return acesso.Acesso{}, errors.Join(ErrDadosIndisponiveis, err)
	}
	p := pedidoAcesso{Operacao: op.String(), Usuario: usuario, Prova: prova, Implantacao: implantacao}
	var res respostaAcesso
	status, _, err := requisitarJSON(ctx, c.http, c.base+rotaCarregarAcesso, c.token, p, &res, limiteAcesso, limiteAcesso, prazoAcesso)
	if err != nil || res.Operacao != p.Operacao {
		return acesso.Acesso{}, errors.Join(ErrDadosIndisponiveis, err)
	}
	if status == http.StatusNotFound && res.Resultado == "desconhecido" {
		return acesso.Acesso{}, acessopg.ErrUsuarioDesconhecido
	}
	if status != http.StatusOK || res.Resultado != "carregado" || res.Acesso == nil || res.Acesso.UsuarioID != usuario || res.Acesso.Implantacao != implantacao {
		return acesso.Acesso{}, ErrDadosIndisponiveis
	}
	return *res.Acesso, nil
}
