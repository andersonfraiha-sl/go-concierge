package dados

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	corecatalogo "go-concierge/internal/core/catalogo"
	"go-concierge/internal/escopo"
	atende "go-concierge/internal/harness/atendimento"
	"go-concierge/internal/harness/canais"
	"go-concierge/internal/harness/catalogo"
	"go-concierge/internal/harness/conteudo"
)

const rotaCatalogoExecucao = "/interno/dados/v1/catalogo/execucao"
const prazoCatalogoExecucao = prazoCatalogo + 6*time.Second

type respostaCatalogoExecucao struct {
	Operacao  string            `json:"operacao"`
	Resultado string            `json:"resultado"`
	Retrato   *catalogo.Retrato `json:"retrato,omitempty"`
}

func registrarCatalogoExecucao(mux *http.ServeMux, store RepositorioCatalogo, repo canais.Repositorio, contaPadrao, token string, recursosNexus func(context.Context) ([]conteudo.Recurso, error)) {
	mux.HandleFunc("POST "+rotaCatalogoExecucao, func(w http.ResponseWriter, r *http.Request) {
		var pedido pedidoConteudo
		if !lerPedido(w, r, token, 16<<10, &pedido) {
			return
		}
		res := respostaCatalogoExecucao{Operacao: pedido.Operacao, Resultado: "recusada"}
		status := http.StatusForbidden
		ctx, cancelar := context.WithTimeout(r.Context(), prazoCatalogo)
		defer cancelar()
		if _, err := uuid.Parse(pedido.Operacao); err == nil && escopo.EmpresaValida(pedido.Empresa) && pedido.Canal != uuid.Nil && escopo.CondominioValido(pedido.Condominio) {
			canal, err := repo.PorID(ctx, pedido.Empresa, pedido.Canal)
			if err != nil && !errors.Is(err, canais.ErrDesconhecido) {
				res.Resultado, status = "indisponivel", http.StatusServiceUnavailable
			} else if err == nil && canal.Habilitado && canal.CompanyID == pedido.Empresa && canal.ID == pedido.Canal && (canal.CondoID == nil || escopo.MesmoCondominio(canal.CondoID, pedido.Condominio)) {
				conta := contaPadrao
				arquivos, err := store.ListarParaExecucao(ctx, canal.Agente, conta)
				if err == nil {
					var retrato catalogo.Retrato
					retrato, err = retratoCatalogo(arquivos, canal.Agente, conta)
					if err == nil {
						cancelar()
						if recursosNexus != nil {
							lista, err := recursosNexus(r.Context())
							_, temFixture := retrato.Arquivos["nexus/"+conta+".yaml"]
							if err == nil || !temFixture {
								if lista == nil {
									lista = []conteudo.Recurso{}
								}
								retrato.RecursosNexus = &lista
							}
						}
						res.Retrato, res.Resultado, status = &retrato, "lido", http.StatusOK
					}
				}
				if err != nil {
					res.Resultado, status = "indisponivel", http.StatusServiceUnavailable
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(res)
	})
}

func retratoCatalogo(arquivos []corecatalogo.Arquivo, agente, conta string) (catalogo.Retrato, error) {
	selecao, err := corecatalogo.SelecionarExecucao(agente, conta)
	if err != nil {
		return catalogo.Retrato{}, err
	}
	agente = selecao.Raiz
	retrato := catalogo.Retrato{Agente: agente, Conta: conta, Arquivos: map[string][]byte{}}
	for _, a := range arquivos {
		if err := corecatalogo.ValidarCaminho(a.Caminho); err != nil {
			return catalogo.Retrato{}, err
		}
		if selecao.Inclui(a.Caminho) {
			retrato.Arquivos[a.Caminho] = a.Conteudo
		}
	}
	if _, ok := retrato.Arquivos[agente+"/agent.yaml"]; !ok {
		if _, ok := retrato.Arquivos[agente+"/workflows/principal.md"]; !ok {
			return catalogo.Retrato{}, errors.New("agente ausente")
		}
	}
	return retrato, nil
}

func (c *Cliente) CatalogoDoEscopo(ctx context.Context, empresa string, condo *int) (catalogo.Retrato, error) {
	company, canal, err := escopoDaSessao(ctx)
	if err != nil || company != empresa {
		return catalogo.Retrato{}, atende.ErrEscopoSessao
	}
	op, err := uuid.NewRandom()
	if err != nil {
		return catalogo.Retrato{}, errors.Join(ErrDadosIndisponiveis, err)
	}
	pedido := pedidoConteudo{Operacao: op.String(), Empresa: empresa, Canal: canal, Condominio: condo}
	var res respostaCatalogoExecucao
	status, _, err := requisitarJSON(ctx, c.http, c.base+rotaCatalogoExecucao, c.token, pedido, &res, 16<<10, limiteConteudo, prazoCatalogoExecucao)
	if err != nil || res.Operacao != pedido.Operacao {
		return catalogo.Retrato{}, errors.Join(ErrDadosIndisponiveis, err)
	}
	if status == http.StatusForbidden && res.Resultado == "recusada" {
		return catalogo.Retrato{}, atende.ErrEscopoSessao
	}
	if status != http.StatusOK || res.Resultado != "lido" || res.Retrato == nil || res.Retrato.Arquivos == nil {
		return catalogo.Retrato{}, ErrDadosIndisponiveis
	}
	return *res.Retrato, nil
}
