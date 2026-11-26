package dados

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/google/uuid"

	"go-concierge/internal/core/administracao/regras"
	"go-concierge/internal/escopo"
	atende "go-concierge/internal/harness/atendimento"
	"go-concierge/internal/harness/canais"
)

const rotaRegrasAtivas = "/interno/dados/v1/regras/ativas"
const rotaRegrasDoEscopo = "/interno/dados/v1/regras/escopo"
const prazoRegras = 5 * time.Second

type pedidoRegrasAtivas struct {
	Operacao string `json:"operacao"`
}

type respostaRegrasAtivas struct {
	Operacao  string                `json:"operacao"`
	Resultado string                `json:"resultado"`
	Regras    []regras.RegraGravada `json:"regras"`
}

func registrarRegrasAtivas(mux *http.ServeMux, fonte regras.FonteDoIndice, repo canais.Repositorio, token string) {
	mux.HandleFunc("POST "+rotaRegrasAtivas, func(w http.ResponseWriter, r *http.Request) {
		var p pedidoRegrasAtivas
		if !lerPedido(w, r, token, 16<<10, &p) {
			return
		}
		if _, err := uuid.Parse(p.Operacao); err != nil {
			http.Error(w, "operação inválida", http.StatusBadRequest)
			return
		}
		ctx, cancelar := context.WithTimeout(r.Context(), prazoRegras)
		defer cancelar()
		gravadas, err := fonte.Ativas(ctx)
		res := respostaRegrasAtivas{Operacao: p.Operacao, Resultado: "lidas", Regras: gravadas}
		status := http.StatusOK
		if err != nil {
			res.Resultado, res.Regras, status = "indisponivel", nil, http.StatusServiceUnavailable
		} else if res.Regras == nil {
			res.Regras = []regras.RegraGravada{}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(res)
	})
	if repo == nil {
		return
	}
	mux.HandleFunc("POST "+rotaRegrasDoEscopo, func(w http.ResponseWriter, r *http.Request) {
		var p pedidoConteudo
		if !lerPedido(w, r, token, 16<<10, &p) {
			return
		}
		res := respostaRegrasAtivas{Operacao: p.Operacao, Resultado: "recusada"}
		status := http.StatusForbidden
		ctx, cancelar := context.WithTimeout(r.Context(), prazoRegras)
		defer cancelar()
		if _, err := uuid.Parse(p.Operacao); err == nil && escopo.EmpresaValida(p.Empresa) && p.Canal != uuid.Nil && escopo.CondominioValido(p.Condominio) {
			canal, err := repo.PorID(ctx, p.Empresa, p.Canal)
			if err != nil && !errors.Is(err, canais.ErrDesconhecido) {
				res.Resultado, status = "indisponivel", http.StatusServiceUnavailable
			} else if err == nil && canal.Habilitado && canal.CompanyID == p.Empresa && canal.ID == p.Canal && (canal.CondoID == nil || escopo.MesmoCondominio(canal.CondoID, p.Condominio)) {
				gravadas, err := fonte.Ativas(ctx)
				if err != nil {
					res.Resultado, status = "indisponivel", http.StatusServiceUnavailable
				} else {
					res.Regras = make([]regras.RegraGravada, 0, len(gravadas))
					for _, regra := range gravadas {
						if regra.AdministradoraID != "" && regra.AdministradoraID != p.Empresa {
							continue
						}
						if p.Condominio != nil && len(regra.Condominios) > 0 && !slices.Contains(regra.Condominios, strconv.Itoa(*p.Condominio)) {
							continue
						}
						res.Regras = append(res.Regras, regra)
					}
					res.Resultado, status = "lidas", http.StatusOK
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(res)
	})
}

func (c *Cliente) AtivasDoEscopo(ctx context.Context, empresa string, condo *int) ([]regras.RegraGravada, error) {
	company, canal, err := escopoDaSessao(ctx)
	if err != nil || company != empresa {
		return nil, atende.ErrEscopoSessao
	}
	op, err := uuid.NewRandom()
	if err != nil {
		return nil, errors.Join(ErrDadosIndisponiveis, err)
	}
	p := pedidoConteudo{Operacao: op.String(), Empresa: empresa, Canal: canal, Condominio: condo}
	var res respostaRegrasAtivas
	status, _, err := requisitarJSON(ctx, c.http, c.base+rotaRegrasDoEscopo, c.token, p, &res, 16<<10, limiteConteudo, prazoRegras)
	if err != nil || res.Operacao != p.Operacao {
		return nil, errors.Join(ErrDadosIndisponiveis, err)
	}
	if status == http.StatusForbidden && res.Resultado == "recusada" {
		return nil, atende.ErrEscopoSessao
	}
	if status != http.StatusOK || res.Resultado != "lidas" || res.Regras == nil {
		return nil, ErrDadosIndisponiveis
	}
	return res.Regras, nil
}

func (c *Cliente) Ativas(ctx context.Context) ([]regras.RegraGravada, error) {
	op, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}
	p := pedidoRegrasAtivas{Operacao: op.String()}
	var res respostaRegrasAtivas
	status, _, err := requisitarJSON(ctx, c.http, c.base+rotaRegrasAtivas, c.token, p, &res, 16<<10, 64<<20, prazoRegras)
	if err != nil || status != http.StatusOK || res.Operacao != p.Operacao || res.Resultado != "lidas" || res.Regras == nil {
		return nil, errors.Join(ErrDadosIndisponiveis, err)
	}
	return res.Regras, nil
}

func (c *Cliente) ObservarAlteracoes(observar func([]regras.RegraGravada)) {
	c.regrasMu.Lock()
	defer c.regrasMu.Unlock()
	c.aoAlterarRegras = observar
}

func (c *Cliente) notificarRegras(gravadas []regras.RegraGravada) {
	c.regrasMu.Lock()
	observar := c.aoAlterarRegras
	c.regrasMu.Unlock()
	if observar != nil {
		observar(gravadas)
	}
}
