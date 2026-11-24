package dados

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"go-concierge/internal/core/banco"
	pedidospg "go-concierge/internal/core/dados/postgres/pedidos"
	"go-concierge/internal/escopo"
	atende "go-concierge/internal/harness/atendimento"
	"go-concierge/internal/harness/pedidos"
	"go-concierge/internal/identidade"
)

const rotaLerPedidos = "/interno/dados/v1/pedidos/ler"
const rotaGravarPedidos = "/interno/dados/v1/pedidos/gravar"
const limitePedidos = 64 << 20
const prazoPedidos = 3 * time.Second

type TrocaProtocolo struct {
	Antes  string `json:"antes"`
	Depois string `json:"depois"`
}

type OperacoesPedidos interface {
	Ler(context.Context, string, uuid.UUID, escopo.Contato) (pedidos.Carteira, error)
	Gravar(context.Context, string, uuid.UUID, escopo.Contato, pedidos.Carteira) ([]TrocaProtocolo, error)
}

type PedidosPostgres struct {
	banco banco.Banco
	store *pedidospg.Postgres
}

func NovosPedidosPostgres(b banco.Banco) *PedidosPostgres {
	return &PedidosPostgres{banco: b, store: pedidospg.NovoPostgres(b)}
}

func (p *PedidosPostgres) Ler(ctx context.Context, empresa string, canal uuid.UUID, c escopo.Contato) (pedidos.Carteira, error) {
	tx, err := p.banco.Pool.Begin(ctx)
	if err != nil {
		return pedidos.Carteira{}, err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	if err := validarContatoNoCanal(ctx, tx, empresa, canal, c); err != nil {
		return pedidos.Carteira{}, err
	}
	var cart pedidos.Carteira
	lote := banco.NovoLote()
	p.store.LerNoLote(ctx, lote, c, func(lida pedidos.Carteira) { cart = lida })
	if err := lote.Executar(ctx, tx); err != nil {
		return pedidos.Carteira{}, err
	}
	if err := banco.Confirmar(ctx, tx); err != nil {
		return pedidos.Carteira{}, err
	}
	return cart, nil
}

func (p *PedidosPostgres) Gravar(ctx context.Context, empresa string, canal uuid.UUID, c escopo.Contato, cart pedidos.Carteira) ([]TrocaProtocolo, error) {
	antes := make([]string, len(cart.Pedidos))
	for i, pedido := range cart.Pedidos {
		if pedido == nil {
			return nil, errors.Join(atende.ErrGravacaoNaoAplicada, atende.ErrGravacaoRecusada)
		}
		antes[i] = pedido.Protocolo
	}
	err := p.store.GravarAutorizado(ctx, c, cart, func(ctx context.Context, tx pgx.Tx) error {
		return validarContatoNoCanal(ctx, tx, empresa, canal, c)
	})
	if err != nil {
		if errors.Is(err, banco.ErrCommitIncerto) {
			return nil, errors.Join(atende.ErrGravacaoIncerta, err)
		}
		if errors.Is(err, atende.ErrEscopoSessao) {
			return nil, errors.Join(atende.ErrGravacaoNaoAplicada, atende.ErrGravacaoRecusada, err)
		}
		return nil, errors.Join(atende.ErrGravacaoNaoAplicada, err)
	}
	trocas := make([]TrocaProtocolo, len(cart.Pedidos))
	for i, pedido := range cart.Pedidos {
		trocas[i] = TrocaProtocolo{Antes: antes[i], Depois: pedido.Protocolo}
	}
	return trocas, nil
}

type pedidoCarteira struct {
	Operacao string            `json:"operacao"`
	Empresa  string            `json:"empresa"`
	Canal    uuid.UUID         `json:"canal"`
	Contato  escopo.Contato    `json:"contato"`
	Carteira *pedidos.Carteira `json:"carteira,omitempty"`
}

type respostaCarteira struct {
	Operacao  string            `json:"operacao"`
	Resultado string            `json:"resultado"`
	Carteira  *pedidos.Carteira `json:"carteira,omitempty"`
	Trocas    []TrocaProtocolo  `json:"trocas"`
}

type PedidosRoteados struct {
	cliente *Cliente
}

func NovosPedidos(cliente *Cliente) *PedidosRoteados {
	return &PedidosRoteados{cliente: cliente}
}

func (s *PedidosRoteados) Ler(ctx context.Context, c escopo.Contato) (pedidos.Carteira, error) {
	if _, ok := identidade.Usuario(ctx); ok {
		res, err := s.cliente.contatoAdmin(ctx, pedidoContatoAdmin{Recurso: "pedidos", Acao: "ler", Contato: c})
		if err != nil {
			return pedidos.Carteira{}, err
		}
		if res.Carteira == nil || validarCarteiraAdministrativa(c, *res.Carteira, true) != nil {
			return pedidos.Carteira{}, ErrDadosIndisponiveis
		}
		restaurarAliasesCarteira(res.Carteira)
		return *res.Carteira, nil
	}
	return s.cliente.lerPedidos(ctx, c)
}

func (s *PedidosRoteados) Gravar(ctx context.Context, c escopo.Contato, cart pedidos.Carteira) error {
	if _, ok := identidade.Usuario(ctx); ok {
		normalizada, err := pedidos.CarteiraParaTransporte(c, cart)
		if err != nil {
			return errors.Join(atende.ErrGravacaoNaoAplicada, err)
		}
		res, err := s.cliente.contatoAdmin(ctx, pedidoContatoAdmin{Recurso: "pedidos", Acao: "gravar", Contato: c, Carteira: &normalizada})
		if err != nil {
			return err
		}
		if err := adotarTrocas(cart, res.Trocas); err != nil {
			return errors.Join(atende.ErrGravacaoIncerta, err)
		}
		return nil
	}
	return s.cliente.gravarPedidos(ctx, c, cart)
}

func (s *PedidosRoteados) Listar(ctx context.Context, empresa string) ([]pedidos.Carteira, error) {
	if _, ok := identidade.Usuario(ctx); !ok {
		return nil, atende.ErrEscopoSessao
	}
	res, err := s.cliente.contatoAdmin(ctx, pedidoContatoAdmin{Recurso: "pedidos", Acao: "listar", Empresa: empresa})
	if err != nil {
		return nil, err
	}
	if res.Carteiras == nil {
		return nil, ErrDadosIndisponiveis
	}
	for i := range res.Carteiras {
		cart := &res.Carteiras[i]
		if validarCarteiraAdministrativa(escopo.Contato{CompanyID: empresa, Telefone: cart.Telefone}, *cart, true) != nil {
			return nil, ErrDadosIndisponiveis
		}
		restaurarAliasesCarteira(cart)
	}
	return res.Carteiras, nil
}

func registrarPedidos(mux *http.ServeMux, store OperacoesPedidos, token string) {
	mux.HandleFunc("POST "+rotaLerPedidos, func(w http.ResponseWriter, r *http.Request) {
		var p pedidoCarteira
		if !lerPedido(w, r, token, limitePedidos, &p) || !validarPedidoCarteira(w, p, false) {
			return
		}
		ctx, cancelar := context.WithTimeout(r.Context(), prazoPedidos)
		defer cancelar()
		cart, err := store.Ler(ctx, p.Empresa, p.Canal, p.Contato)
		if err != nil {
			responderErroPedidos(w, p.Operacao, err, false)
			return
		}
		responderPedidos(w, respostaCarteira{Operacao: p.Operacao, Resultado: "lida", Carteira: &cart}, http.StatusOK)
	})
	mux.HandleFunc("POST "+rotaGravarPedidos, func(w http.ResponseWriter, r *http.Request) {
		var p pedidoCarteira
		if !lerPedido(w, r, token, limitePedidos, &p) || !validarPedidoCarteira(w, p, true) {
			return
		}
		ctx, cancelar := context.WithTimeout(r.Context(), prazoPedidos)
		defer cancelar()
		trocas, err := store.Gravar(ctx, p.Empresa, p.Canal, p.Contato, *p.Carteira)
		if err != nil {
			responderErroPedidos(w, p.Operacao, err, true)
			return
		}
		if trocas == nil {
			trocas = []TrocaProtocolo{}
		}
		responderPedidos(w, respostaCarteira{Operacao: p.Operacao, Resultado: "aplicada", Trocas: trocas}, http.StatusOK)
	})
}

func validarPedidoCarteira(w http.ResponseWriter, p pedidoCarteira, escrita bool) bool {
	invalido := p.Contato.Validar() != nil || p.Contato.CompanyID != p.Empresa || p.Contato.Telefone != escopo.NormalizarTelefone(p.Contato.Telefone) || strings.TrimSpace(p.Empresa) == "" || p.Canal == uuid.Nil || escrita != (p.Carteira != nil)
	if _, err := uuid.Parse(p.Operacao); err != nil {
		invalido = true
	}
	if p.Carteira != nil {
		if p.Carteira.CompanyID != "" && p.Carteira.CompanyID != p.Empresa || p.Carteira.Telefone != "" && p.Carteira.Telefone != p.Contato.Telefone {
			invalido = true
		}
		for _, ped := range p.Carteira.Pedidos {
			if ped == nil || ped.CompanyID != "" && ped.CompanyID != p.Empresa || ped.Telefone != "" && ped.Telefone != p.Contato.Telefone || !p.Contato.Permite(ped.CondoID) {
				invalido = true
			}
		}
	}
	if invalido {
		responderPedidos(w, respostaCarteira{Operacao: p.Operacao, Resultado: "recusada"}, http.StatusForbidden)
		return false
	}
	return true
}

func responderErroPedidos(w http.ResponseWriter, operacao string, err error, escrita bool) {
	switch {
	case escrita && (errors.Is(err, atende.ErrGravacaoIncerta) || errors.Is(err, banco.ErrCommitIncerto)):
		responderPedidos(w, respostaCarteira{Operacao: operacao, Resultado: "incerta"}, http.StatusInternalServerError)
	case errors.Is(err, atende.ErrEscopoSessao) || errors.Is(err, atende.ErrGravacaoRecusada):
		responderPedidos(w, respostaCarteira{Operacao: operacao, Resultado: "recusada"}, http.StatusForbidden)
	case escrita && errors.Is(err, pedidos.ErrProtocoloEmUso) && errors.Is(err, atende.ErrGravacaoNaoAplicada):
		responderPedidos(w, respostaCarteira{Operacao: operacao, Resultado: "protocolo_em_uso"}, http.StatusConflict)
	case escrita && errors.Is(err, atende.ErrGravacaoNaoAplicada):
		responderPedidos(w, respostaCarteira{Operacao: operacao, Resultado: "nao_aplicada"}, http.StatusUnprocessableEntity)
	case escrita:
		responderPedidos(w, respostaCarteira{Operacao: operacao, Resultado: "incerta"}, http.StatusInternalServerError)
	default:
		responderPedidos(w, respostaCarteira{Operacao: operacao, Resultado: "indisponivel"}, http.StatusServiceUnavailable)
	}
}

func responderPedidos(w http.ResponseWriter, resposta respostaCarteira, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resposta)
}

func pedidoDeCarteira(ctx context.Context, c escopo.Contato) (pedidoCarteira, uuid.UUID, error) {
	empresa, canal, contato, op, err := escopoDoContato(ctx, c)
	if err != nil {
		return pedidoCarteira{}, uuid.Nil, err
	}
	return pedidoCarteira{Operacao: op.String(), Empresa: empresa, Canal: canal, Contato: contato}, op, nil
}

func (c *Cliente) lerPedidos(ctx context.Context, contato escopo.Contato) (pedidos.Carteira, error) {
	p, _, err := pedidoDeCarteira(ctx, contato)
	if err != nil {
		return pedidos.Carteira{}, err
	}
	var res respostaCarteira
	status, _, err := requisitarJSON(ctx, c.http, c.base+rotaLerPedidos, c.token, p, &res, limitePedidos, limitePedidos, prazoPedidos)
	if err != nil || res.Operacao != p.Operacao {
		return pedidos.Carteira{}, errors.Join(ErrDadosIndisponiveis, err)
	}
	if status == http.StatusForbidden && res.Resultado == "recusada" {
		return pedidos.Carteira{}, atende.ErrEscopoSessao
	}
	if status != http.StatusOK || res.Resultado != "lida" || res.Carteira == nil {
		return pedidos.Carteira{}, fmt.Errorf("%w: resposta de pedidos inválida", ErrDadosIndisponiveis)
	}
	cart := *res.Carteira
	if cart.CompanyID != p.Empresa || cart.Telefone != p.Contato.Telefone {
		return pedidos.Carteira{}, atende.ErrEscopoSessao
	}
	cart.AdministradoraID = cart.CompanyID
	for _, ped := range cart.Pedidos {
		if ped == nil || ped.CompanyID != p.Empresa || ped.Telefone != p.Contato.Telefone || !p.Contato.Permite(ped.CondoID) {
			return pedidos.Carteira{}, atende.ErrEscopoSessao
		}
		ped.AdministradoraID = ped.CompanyID
		if ped.CondoID != nil {
			ped.CondominioID = strconv.Itoa(*ped.CondoID)
		}
	}
	return cart, nil
}

func (c *Cliente) gravarPedidos(ctx context.Context, contato escopo.Contato, cart pedidos.Carteira) error {
	p, op, err := pedidoDeCarteira(ctx, contato)
	if err != nil {
		return errors.Join(atende.ErrGravacaoNaoAplicada, atende.ErrGravacaoRecusada, err)
	}
	normalizada, err := pedidos.CarteiraParaTransporte(p.Contato, cart)
	if err != nil {
		return errors.Join(atende.ErrGravacaoNaoAplicada, err)
	}
	for _, ped := range normalizada.Pedidos {
		if ped == nil {
			return errors.Join(atende.ErrGravacaoNaoAplicada, atende.ErrGravacaoRecusada)
		}
	}
	p.Carteira = &normalizada
	var res respostaCarteira
	status, enviada, err := requisitarJSON(ctx, c.http, c.base+rotaGravarPedidos, c.token, p, &res, limitePedidos, limitePedidos, prazoPedidos)
	if err != nil {
		if !enviada {
			return errors.Join(atende.ErrGravacaoNaoAplicada, err)
		}
		return erroIncerto(op, err)
	}
	if res.Operacao != p.Operacao {
		return erroIncerto(op, errors.New("resposta de pedidos inválida"))
	}
	switch {
	case status == http.StatusOK && res.Resultado == "aplicada":
		if err := adotarTrocas(cart, res.Trocas); err != nil {
			return erroIncerto(op, err)
		}
		return nil
	case status == http.StatusForbidden && res.Resultado == "recusada":
		return errors.Join(atende.ErrGravacaoNaoAplicada, atende.ErrGravacaoRecusada, atende.ErrEscopoSessao)
	case status == http.StatusConflict && res.Resultado == "protocolo_em_uso":
		return errors.Join(atende.ErrGravacaoNaoAplicada, pedidos.ErrProtocoloEmUso)
	case status == http.StatusUnprocessableEntity && res.Resultado == "nao_aplicada":
		return atende.ErrGravacaoNaoAplicada
	default:
		return erroIncerto(op, fmt.Errorf("HTTP %d", status))
	}
}
