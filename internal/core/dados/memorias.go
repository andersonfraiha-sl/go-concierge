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
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"go-concierge/internal/core/banco"
	memoriapg "go-concierge/internal/core/dados/postgres/memoria"
	"go-concierge/internal/escopo"
	atende "go-concierge/internal/harness/atendimento"
	"go-concierge/internal/harness/memoria"
	"go-concierge/internal/identidade"
)

const rotaLerMemoria = "/interno/dados/v1/memorias/ler"
const rotaGravarMemoria = "/interno/dados/v1/memorias/gravar"
const limiteMemoria = 64 << 20
const prazoMemoria = 3 * time.Second

type OperacoesMemoria interface {
	Ler(context.Context, string, uuid.UUID, escopo.Contato) (memoria.Memoria, error)
	Gravar(context.Context, string, uuid.UUID, escopo.Contato, memoria.Memoria) error
}

type MemoriasPostgres struct {
	banco banco.Banco
	store *memoriapg.Postgres
}

func NovasMemoriasPostgres(b banco.Banco) *MemoriasPostgres {
	return &MemoriasPostgres{banco: b, store: memoriapg.NovoPostgres(b)}
}

func validarContatoNoCanal(ctx context.Context, tx pgx.Tx, empresa string, canal uuid.UUID, c escopo.Contato) error {
	if err := c.Validar(); err != nil || c.CompanyID != empresa || c.Telefone != escopo.NormalizarTelefone(c.Telefone) || canal == uuid.Nil {
		return atende.ErrEscopoSessao
	}
	var condo pgtype.Int4
	err := tx.QueryRow(ctx, `SELECT condo_id FROM channels WHERE company_id=$1 AND id=$2 FOR SHARE`, empresa, canal).Scan(&condo)
	if errors.Is(err, pgx.ErrNoRows) {
		return atende.ErrEscopoSessao
	}
	if err != nil {
		return err
	}
	if condo.Valid && (c.CondoID == nil || int32(*c.CondoID) != condo.Int32) {
		return atende.ErrEscopoSessao
	}
	return nil
}

func (p *MemoriasPostgres) Ler(ctx context.Context, empresa string, canal uuid.UUID, c escopo.Contato) (memoria.Memoria, error) {
	tx, err := p.banco.Pool.Begin(ctx)
	if err != nil {
		return memoria.Memoria{}, err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	if err := validarContatoNoCanal(ctx, tx, empresa, canal, c); err != nil {
		return memoria.Memoria{}, err
	}
	var m memoria.Memoria
	lote := banco.NovoLote()
	p.store.LerNoLote(ctx, lote, c, func(lida memoria.Memoria) { m = lida })
	if err := lote.Executar(ctx, tx); err != nil {
		return memoria.Memoria{}, err
	}
	if err := banco.Confirmar(ctx, tx); err != nil {
		return memoria.Memoria{}, err
	}
	return m, nil
}

func (p *MemoriasPostgres) Gravar(ctx context.Context, empresa string, canal uuid.UUID, c escopo.Contato, m memoria.Memoria) error {
	tx, err := p.banco.Pool.Begin(ctx)
	if err != nil {
		return errors.Join(atende.ErrGravacaoNaoAplicada, err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	if err := validarContatoNoCanal(ctx, tx, empresa, canal, c); err != nil {
		if errors.Is(err, atende.ErrEscopoSessao) {
			return errors.Join(atende.ErrGravacaoNaoAplicada, atende.ErrGravacaoRecusada, err)
		}
		return errors.Join(atende.ErrGravacaoNaoAplicada, err)
	}
	if err := p.store.GravarNaTransacao(ctx, tx, c, m); err != nil {
		return errors.Join(atende.ErrGravacaoNaoAplicada, err)
	}
	if err := banco.Confirmar(ctx, tx); err != nil {
		if errors.Is(err, banco.ErrCommitIncerto) {
			return errors.Join(atende.ErrGravacaoIncerta, err)
		}
		return errors.Join(atende.ErrGravacaoNaoAplicada, err)
	}
	return nil
}

type pedidoMemoria struct {
	Operacao string           `json:"operacao"`
	Empresa  string           `json:"empresa"`
	Canal    uuid.UUID        `json:"canal"`
	Contato  escopo.Contato   `json:"contato"`
	Memoria  *memoria.Memoria `json:"memoria,omitempty"`
}

type respostaMemoria struct {
	Operacao  string           `json:"operacao"`
	Resultado string           `json:"resultado"`
	Memoria   *memoria.Memoria `json:"memoria,omitempty"`
}

type MemoriasRoteadas struct {
	cliente *Cliente
}

func NovasMemorias(cliente *Cliente) *MemoriasRoteadas {
	return &MemoriasRoteadas{cliente: cliente}
}

func (s *MemoriasRoteadas) Ler(ctx context.Context, c escopo.Contato) (memoria.Memoria, error) {
	if _, ok := identidade.Usuario(ctx); ok {
		res, err := s.cliente.contatoAdmin(ctx, pedidoContatoAdmin{Recurso: "memoria", Acao: "ler", Contato: c})
		if err != nil {
			return memoria.Memoria{}, err
		}
		if res.Memoria == nil || validarMemoriaAdministrativa(c, *res.Memoria, true) != nil {
			return memoria.Memoria{}, ErrDadosIndisponiveis
		}
		res.Memoria.AdministradoraID = res.Memoria.CompanyID
		return *res.Memoria, nil
	}
	return s.cliente.lerMemoria(ctx, c)
}

func (s *MemoriasRoteadas) Gravar(ctx context.Context, c escopo.Contato, m memoria.Memoria) error {
	if _, ok := identidade.Usuario(ctx); ok {
		normalizada, err := memoria.ParaTransporte(c, m)
		if err != nil {
			return errors.Join(atende.ErrGravacaoNaoAplicada, err)
		}
		_, err = s.cliente.contatoAdmin(ctx, pedidoContatoAdmin{Recurso: "memoria", Acao: "gravar", Contato: c, Memoria: &normalizada})
		return err
	}
	return s.cliente.gravarMemoria(ctx, c, m)
}

func (s *MemoriasRoteadas) Listar(ctx context.Context, empresa string) ([]memoria.Memoria, error) {
	if _, ok := identidade.Usuario(ctx); !ok {
		return nil, atende.ErrEscopoSessao
	}
	res, err := s.cliente.contatoAdmin(ctx, pedidoContatoAdmin{Recurso: "memoria", Acao: "listar", Empresa: empresa})
	if err != nil {
		return nil, err
	}
	if res.Memorias == nil {
		return nil, ErrDadosIndisponiveis
	}
	for i := range res.Memorias {
		m := &res.Memorias[i]
		c := escopo.Contato{CompanyID: empresa, Telefone: m.Telefone}
		if validarMemoriaAdministrativa(c, *m, true) != nil {
			return nil, ErrDadosIndisponiveis
		}
		m.AdministradoraID = m.CompanyID
	}
	return res.Memorias, nil
}

func registrarMemorias(mux *http.ServeMux, store OperacoesMemoria, token string) {
	mux.HandleFunc("POST "+rotaLerMemoria, func(w http.ResponseWriter, r *http.Request) {
		var p pedidoMemoria
		if !lerPedido(w, r, token, limiteMemoria, &p) {
			return
		}
		if !validarPedidoMemoria(w, p, false) {
			return
		}
		ctx, cancelar := context.WithTimeout(r.Context(), prazoMemoria)
		defer cancelar()
		m, err := store.Ler(ctx, p.Empresa, p.Canal, p.Contato)
		if err != nil {
			responderErroMemoria(w, p.Operacao, err, false)
			return
		}
		responderMemoria(w, respostaMemoria{Operacao: p.Operacao, Resultado: "lida", Memoria: &m}, http.StatusOK)
	})
	mux.HandleFunc("POST "+rotaGravarMemoria, func(w http.ResponseWriter, r *http.Request) {
		var p pedidoMemoria
		if !lerPedido(w, r, token, limiteMemoria, &p) {
			return
		}
		if !validarPedidoMemoria(w, p, true) {
			return
		}
		ctx, cancelar := context.WithTimeout(r.Context(), prazoMemoria)
		defer cancelar()
		if err := store.Gravar(ctx, p.Empresa, p.Canal, p.Contato, *p.Memoria); err != nil {
			responderErroMemoria(w, p.Operacao, err, true)
			return
		}
		responderMemoria(w, respostaMemoria{Operacao: p.Operacao, Resultado: "aplicada"}, http.StatusOK)
	})
}

func validarPedidoMemoria(w http.ResponseWriter, p pedidoMemoria, escrita bool) bool {
	invalido := p.Contato.Validar() != nil || p.Contato.CompanyID != p.Empresa || p.Contato.Telefone != escopo.NormalizarTelefone(p.Contato.Telefone) || strings.TrimSpace(p.Empresa) == "" || p.Canal == uuid.Nil || escrita != (p.Memoria != nil)
	if _, err := uuid.Parse(p.Operacao); err != nil {
		invalido = true
	}
	if p.Memoria != nil && (p.Memoria.CompanyID != "" && p.Memoria.CompanyID != p.Empresa || p.Memoria.Telefone != "" && p.Memoria.Telefone != p.Contato.Telefone) {
		invalido = true
	}
	if invalido {
		responderMemoria(w, respostaMemoria{Operacao: p.Operacao, Resultado: "recusada"}, http.StatusForbidden)
		return false
	}
	return true
}

func responderErroMemoria(w http.ResponseWriter, operacao string, err error, escrita bool) {
	switch {
	case errors.Is(err, atende.ErrGravacaoIncerta) || errors.Is(err, banco.ErrCommitIncerto) && escrita:
		responderMemoria(w, respostaMemoria{Operacao: operacao, Resultado: "incerta"}, http.StatusInternalServerError)
	case errors.Is(err, atende.ErrEscopoSessao) || errors.Is(err, atende.ErrGravacaoRecusada):
		responderMemoria(w, respostaMemoria{Operacao: operacao, Resultado: "recusada"}, http.StatusForbidden)
	case escrita && errors.Is(err, atende.ErrGravacaoNaoAplicada):
		responderMemoria(w, respostaMemoria{Operacao: operacao, Resultado: "nao_aplicada"}, http.StatusUnprocessableEntity)
	case escrita:
		responderMemoria(w, respostaMemoria{Operacao: operacao, Resultado: "incerta"}, http.StatusInternalServerError)
	default:
		responderMemoria(w, respostaMemoria{Operacao: operacao, Resultado: "indisponivel"}, http.StatusServiceUnavailable)
	}
}

func responderMemoria(w http.ResponseWriter, resposta respostaMemoria, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resposta)
}

func escopoDoContato(ctx context.Context, c escopo.Contato) (string, uuid.UUID, escopo.Contato, uuid.UUID, error) {
	empresa, canal, err := escopoDaSessao(ctx)
	if err != nil {
		return "", uuid.Nil, escopo.Contato{}, uuid.Nil, err
	}
	if err := c.Validar(); err != nil {
		return "", uuid.Nil, escopo.Contato{}, uuid.Nil, err
	}
	c = c.Canonico()
	if c.CompanyID != empresa {
		return "", uuid.Nil, escopo.Contato{}, uuid.Nil, atende.ErrEscopoSessao
	}
	op, err := uuid.NewRandom()
	if err != nil {
		return "", uuid.Nil, escopo.Contato{}, uuid.Nil, err
	}
	return empresa, canal, c, op, nil
}

func pedidoDeMemoria(ctx context.Context, c escopo.Contato) (pedidoMemoria, uuid.UUID, error) {
	empresa, canal, contato, op, err := escopoDoContato(ctx, c)
	if err != nil {
		return pedidoMemoria{}, uuid.Nil, err
	}
	return pedidoMemoria{Operacao: op.String(), Empresa: empresa, Canal: canal, Contato: contato}, op, nil
}

func (c *Cliente) lerMemoria(ctx context.Context, contato escopo.Contato) (memoria.Memoria, error) {
	p, _, err := pedidoDeMemoria(ctx, contato)
	if err != nil {
		return memoria.Memoria{}, err
	}
	var res respostaMemoria
	status, _, err := requisitarJSON(ctx, c.http, c.base+rotaLerMemoria, c.token, p, &res, limiteMemoria, limiteMemoria, prazoMemoria)
	if err != nil || res.Operacao != p.Operacao {
		return memoria.Memoria{}, errors.Join(ErrDadosIndisponiveis, err)
	}
	if status == http.StatusForbidden && res.Resultado == "recusada" {
		return memoria.Memoria{}, atende.ErrEscopoSessao
	}
	if status != http.StatusOK || res.Resultado != "lida" || res.Memoria == nil {
		return memoria.Memoria{}, fmt.Errorf("%w: resposta de memória inválida", ErrDadosIndisponiveis)
	}
	m := *res.Memoria
	if m.CompanyID != p.Contato.CompanyID || m.Telefone != p.Contato.Telefone {
		return memoria.Memoria{}, atende.ErrEscopoSessao
	}
	for _, f := range m.Fatos {
		if !p.Contato.Permite(f.CondoID) {
			return memoria.Memoria{}, atende.ErrEscopoSessao
		}
	}
	m.AdministradoraID = m.CompanyID
	return m, nil
}

func (c *Cliente) gravarMemoria(ctx context.Context, contato escopo.Contato, m memoria.Memoria) error {
	p, op, err := pedidoDeMemoria(ctx, contato)
	if err != nil {
		return errors.Join(atende.ErrGravacaoNaoAplicada, atende.ErrGravacaoRecusada, err)
	}
	m, err = memoria.ParaTransporte(p.Contato, m)
	if err != nil {
		return errors.Join(atende.ErrGravacaoNaoAplicada, err)
	}
	p.Memoria = &m
	var res respostaMemoria
	status, enviada, err := requisitarJSON(ctx, c.http, c.base+rotaGravarMemoria, c.token, p, &res, limiteMemoria, limiteMemoria, prazoMemoria)
	if err != nil {
		if !enviada {
			return errors.Join(atende.ErrGravacaoNaoAplicada, err)
		}
		return erroIncerto(op, err)
	}
	if res.Operacao != p.Operacao {
		return erroIncerto(op, errors.New("resposta de memória inválida"))
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
