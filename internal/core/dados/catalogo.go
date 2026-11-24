package dados

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"go-concierge/internal/core/catalogo"
	"go-concierge/internal/harness/persistencia"
)

const rotaCatalogo = "/interno/dados/v1/catalogo/materializar"
const prazoCatalogo = 5 * time.Second

type RepositorioCatalogo interface {
	Listar(context.Context) ([]catalogo.Arquivo, error)
	Substituir(context.Context, []catalogo.Arquivo, ...string) (catalogo.Mudanca, error)
}

type CatalogoHTTP struct {
	base, token string
	http        *http.Client
}

func (s *Servidor) Catalogo() *CatalogoHTTP { return s.catalogo }

type pedidoCatalogo struct {
	Operacao string             `json:"operacao"`
	Acao     string             `json:"acao"`
	Arquivos []catalogo.Arquivo `json:"arquivos,omitempty"`
	Remover  []string           `json:"remover,omitempty"`
}

type respostaCatalogo struct {
	Operacao  string             `json:"operacao"`
	Resultado string             `json:"resultado"`
	Arquivos  []catalogo.Arquivo `json:"arquivos"`
	Mudanca   catalogo.Mudanca   `json:"mudanca"`
}

func registrarCatalogo(mux *http.ServeMux, repo RepositorioCatalogo, token string) {
	mux.HandleFunc("POST "+rotaCatalogo, func(w http.ResponseWriter, r *http.Request) {
		var p pedidoCatalogo
		if !lerPedido(w, r, token, limiteTurno, &p) {
			return
		}
		res := respostaCatalogo{Operacao: p.Operacao, Resultado: "recusada"}
		status := http.StatusForbidden
		ctx, cancelar := context.WithTimeout(r.Context(), prazoCatalogo)
		defer cancelar()
		if _, err := uuid.Parse(p.Operacao); err == nil {
			switch p.Acao {
			case "listar":
				if len(p.Arquivos) == 0 && len(p.Remover) == 0 {
					res.Arquivos, err = repo.Listar(ctx)
					if err != nil {
						res.Resultado, status = "indisponivel", http.StatusServiceUnavailable
					} else {
						if res.Arquivos == nil {
							res.Arquivos = []catalogo.Arquivo{}
						}
						res.Resultado, status = "lido", http.StatusOK
					}
				}
			case "substituir":
				res.Mudanca, err = repo.Substituir(ctx, p.Arquivos, p.Remover...)
				if errors.Is(err, persistencia.ErrGravacaoIncerta) {
					res.Resultado, status = "incerta", http.StatusInternalServerError
				} else if err != nil {
					res.Resultado, status = "nao_aplicada", http.StatusUnprocessableEntity
				} else {
					res.Resultado, status = "aplicada", http.StatusOK
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(res)
	})
}

func (c *CatalogoHTTP) executar(ctx context.Context, p pedidoCatalogo) (respostaCatalogo, error) {
	op, err := uuid.NewRandom()
	if err != nil {
		return respostaCatalogo{}, errors.Join(persistencia.ErrGravacaoNaoAplicada, err)
	}
	p.Operacao = op.String()
	var res respostaCatalogo
	status, enviada, err := requisitarJSON(ctx, c.http, c.base+rotaCatalogo, c.token, p, &res, limiteTurno, limiteTurno, prazoCatalogo)
	if err == nil && res.Operacao == p.Operacao {
		if status == http.StatusOK && (p.Acao == "listar" && res.Resultado == "lido" && res.Arquivos != nil || p.Acao == "substituir" && res.Resultado == "aplicada") {
			return res, nil
		}
		if status == http.StatusUnprocessableEntity && res.Resultado == "nao_aplicada" {
			return res, persistencia.ErrGravacaoNaoAplicada
		}
		if status == http.StatusForbidden && res.Resultado == "recusada" {
			return res, errors.Join(persistencia.ErrGravacaoNaoAplicada, persistencia.ErrGravacaoRecusada)
		}
	}
	if p.Acao == "listar" {
		return res, errors.Join(ErrDadosIndisponiveis, err)
	}
	if !enviada {
		return res, errors.Join(persistencia.ErrGravacaoNaoAplicada, err)
	}
	return res, erroIncerto(op, err)
}

func (c *CatalogoHTTP) Listar(ctx context.Context) ([]catalogo.Arquivo, error) {
	res, err := c.executar(ctx, pedidoCatalogo{Acao: "listar"})
	return res.Arquivos, err
}
func (c *CatalogoHTTP) Substituir(ctx context.Context, arquivos []catalogo.Arquivo, remover ...string) (catalogo.Mudanca, error) {
	res, err := c.executar(ctx, pedidoCatalogo{Acao: "substituir", Arquivos: arquivos, Remover: remover})
	return res.Mudanca, err
}
