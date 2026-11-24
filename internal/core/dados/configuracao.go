package dados

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"go-concierge/internal/escopo"
	atende "go-concierge/internal/harness/atendimento"
	"go-concierge/internal/harness/canais"
	admin "go-concierge/internal/harness/configuracao"
)

const rotaConfiguracao = "/interno/dados/v1/conteudo/configuracao"

type LeitorConfiguracao interface {
	Conteudo(context.Context, string, *int) (admin.Mapeamento, error)
}

type respostaConfiguracao struct {
	Operacao   string            `json:"operacao"`
	Resultado  string            `json:"resultado"`
	Mapeamento *admin.Mapeamento `json:"mapeamento,omitempty"`
}

func registrarConfiguracao(mux *http.ServeMux, repo canais.Repositorio, store LeitorConfiguracao, token string) {
	mux.HandleFunc("POST "+rotaConfiguracao, func(w http.ResponseWriter, r *http.Request) {
		var p pedidoConteudo
		if !lerPedido(w, r, token, 16<<10, &p) {
			return
		}
		res := respostaConfiguracao{Operacao: p.Operacao, Resultado: "recusada"}
		status := http.StatusForbidden
		ctx, cancelar := context.WithTimeout(r.Context(), prazoConteudo)
		defer cancelar()
		if _, err := uuid.Parse(p.Operacao); err == nil && escopo.EmpresaValida(p.Empresa) && p.Canal != uuid.Nil && escopo.CondominioValido(p.Condominio) {
			canal, err := repo.PorID(ctx, p.Empresa, p.Canal)
			if err != nil && !errors.Is(err, canais.ErrDesconhecido) {
				res.Resultado, status = "indisponivel", http.StatusServiceUnavailable
			} else if err == nil && canal.CompanyID == p.Empresa && canal.ID == p.Canal && (canal.CondoID == nil || escopo.MesmoCondominio(canal.CondoID, p.Condominio)) {
				m, err := store.Conteudo(ctx, p.Empresa, p.Condominio)
				if err != nil {
					res.Resultado, status = "indisponivel", http.StatusServiceUnavailable
				} else {
					res.Mapeamento, res.Resultado, status = &m, "lida", http.StatusOK
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(res)
	})
}

func (c *Cliente) Conteudo(ctx context.Context, empresa string, condo *int) (admin.Mapeamento, error) {
	company, canal, err := escopoDaSessao(ctx)
	if err != nil || company != empresa {
		return admin.Mapeamento{}, atende.ErrEscopoSessao
	}
	op, err := uuid.NewRandom()
	if err != nil {
		return admin.Mapeamento{}, errors.Join(ErrDadosIndisponiveis, err)
	}
	var res respostaConfiguracao
	p := pedidoConteudo{Operacao: op.String(), Empresa: empresa, Canal: canal, Condominio: condo}
	status, _, err := requisitarJSON(ctx, c.http, c.base+rotaConfiguracao, c.token, p, &res, 16<<10, limiteConteudo, prazoConteudo)
	if err != nil || res.Operacao != p.Operacao {
		return admin.Mapeamento{}, errors.Join(ErrDadosIndisponiveis, err)
	}
	if status == http.StatusForbidden && res.Resultado == "recusada" {
		return admin.Mapeamento{}, atende.ErrEscopoSessao
	}
	if status != http.StatusOK || res.Resultado != "lida" || res.Mapeamento == nil {
		return admin.Mapeamento{}, ErrDadosIndisponiveis
	}
	return *res.Mapeamento, nil
}

func (c *Cliente) Ler(ctx context.Context) (admin.Mapeamento, error) {
	empresa, id, err := escopoDaSessao(ctx)
	if err != nil {
		return admin.Mapeamento{}, err
	}
	canal, err := c.PorID(ctx, empresa, id)
	if err != nil {
		return admin.Mapeamento{}, err
	}
	return c.Conteudo(ctx, empresa, canal.CondoID)
}
