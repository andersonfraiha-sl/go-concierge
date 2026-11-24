package dados

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"go-concierge/internal/core/administracao/camadas"
	"go-concierge/internal/escopo"
	atende "go-concierge/internal/harness/atendimento"
	"go-concierge/internal/harness/canais"
)

const rotaConteudo = "/interno/dados/v1/conteudo/publicado"
const prazoConteudo = 3 * time.Second
const limiteConteudo = 64 << 20

type LeitorConteudo interface {
	LerRetratoDoEscopo(context.Context, string, *int) (camadas.Retrato, error)
}

type pedidoConteudo struct {
	Operacao   string    `json:"operacao"`
	Empresa    string    `json:"empresa"`
	Canal      uuid.UUID `json:"canal"`
	Condominio *int      `json:"condominio"`
}

type respostaConteudo struct {
	Operacao  string           `json:"operacao"`
	Resultado string           `json:"resultado"`
	Retrato   *camadas.Retrato `json:"retrato,omitempty"`
}

func registrarConteudo(mux *http.ServeMux, canaisRepo canais.Repositorio, store LeitorConteudo, token string) {
	mux.HandleFunc("POST "+rotaConteudo, func(w http.ResponseWriter, r *http.Request) {
		var p pedidoConteudo
		if !lerPedido(w, r, token, 16<<10, &p) {
			return
		}
		res := respostaConteudo{Operacao: p.Operacao}
		responder := func(resultado string, status int) {
			res.Resultado = resultado
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(res)
		}
		if _, err := uuid.Parse(p.Operacao); err != nil || strings.TrimSpace(p.Empresa) == "" || p.Canal == uuid.Nil || !escopo.CondominioValido(p.Condominio) {
			responder("recusada", http.StatusForbidden)
			return
		}
		ctx, cancelar := context.WithTimeout(r.Context(), prazoConteudo)
		defer cancelar()
		canal, err := canaisRepo.PorID(ctx, p.Empresa, p.Canal)
		if errors.Is(err, canais.ErrDesconhecido) || err == nil && (canal.CompanyID != p.Empresa || canal.ID != p.Canal || canal.CondoID != nil && !escopo.MesmoCondominio(canal.CondoID, p.Condominio)) {
			responder("recusada", http.StatusForbidden)
			return
		}
		if err != nil {
			responder("indisponivel", http.StatusServiceUnavailable)
			return
		}
		retrato, err := store.LerRetratoDoEscopo(ctx, p.Empresa, p.Condominio)
		if err != nil || validarRetrato(retrato, p.Empresa, p.Condominio) != nil {
			responder("indisponivel", http.StatusServiceUnavailable)
			return
		}
		res.Retrato = &retrato
		responder("lido", http.StatusOK)
	})
}

func validarRetrato(retrato camadas.Retrato, empresa string, condo *int) error {
	if retrato.Bases == nil || retrato.Administradoras == nil || retrato.Condominios == nil {
		return ErrDadosIndisponiveis
	}
	for _, donas := range retrato.Administradoras {
		for dona := range donas {
			if dona != empresa {
				return atende.ErrEscopoSessao
			}
		}
	}
	for _, recortes := range retrato.Condominios {
		for recorte := range recortes {
			if condo == nil || recorte != escopo.ChaveCondominio(empresa, strconv.Itoa(*condo)) {
				return atende.ErrEscopoSessao
			}
		}
	}
	return nil
}

func (c *Cliente) LerRetratoDoEscopo(ctx context.Context, empresa string, condo *int) (camadas.Retrato, error) {
	company, canal, err := escopoDaSessao(ctx)
	if err != nil || company != empresa {
		return camadas.Retrato{}, atende.ErrEscopoSessao
	}
	op, err := uuid.NewRandom()
	if err != nil {
		return camadas.Retrato{}, errors.Join(ErrDadosIndisponiveis, err)
	}
	p := pedidoConteudo{Operacao: op.String(), Empresa: empresa, Canal: canal, Condominio: condo}
	var res respostaConteudo
	status, _, err := requisitarJSON(ctx, c.http, c.base+rotaConteudo, c.token, p, &res, 16<<10, limiteConteudo, prazoConteudo)
	if err != nil || res.Operacao != p.Operacao {
		return camadas.Retrato{}, errors.Join(ErrDadosIndisponiveis, err)
	}
	if status == http.StatusForbidden && res.Resultado == "recusada" {
		return camadas.Retrato{}, atende.ErrEscopoSessao
	}
	if status != http.StatusOK || res.Resultado != "lido" || res.Retrato == nil {
		return camadas.Retrato{}, ErrDadosIndisponiveis
	}
	if err := validarRetrato(*res.Retrato, empresa, condo); err != nil {
		return camadas.Retrato{}, err
	}
	return *res.Retrato, nil
}
