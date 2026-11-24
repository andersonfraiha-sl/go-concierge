package dados

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"go-concierge/internal/acesso"
	"go-concierge/internal/escopo"
	atende "go-concierge/internal/harness/atendimento"
	admin "go-concierge/internal/harness/configuracao"
	"go-concierge/internal/identidade"
)

const rotaAdministracao = "/interno/dados/v1/administracao/consulta"
const prazoAdministracao = 5 * time.Second

type ConsultaAdministracao interface {
	Ler(context.Context) (admin.Mapeamento, error)
	ConteudoConfigurado(context.Context, string, *int) (admin.Mapeamento, error)
}

type AdministracaoHTTP struct {
	cliente *Cliente
}

func NovaAdministracao(c *Cliente) *AdministracaoHTTP {
	return &AdministracaoHTTP{cliente: c}
}

type pedidoAdministracao struct {
	Operacao   string
	Prova      string
	Acao       string
	Empresa    string
	Condominio *int
}

type respostaAdministracao struct {
	Operacao   string
	Resultado  string
	Mapeamento *admin.Mapeamento
}

func alvoAdministracao(empresa string, condo *int) (acesso.Escopo, error) {
	if !escopo.CondominioValido(condo) || empresa != "" && !escopo.EmpresaValida(empresa) || empresa == "" && condo != nil {
		return acesso.Escopo{}, atende.ErrEscopoSessao
	}
	if empresa == "" {
		return acesso.Escopo{Tipo: acesso.TipoSuperlogica}, nil
	}
	if condo != nil {
		return acesso.EscopoCondominio(empresa, strconv.Itoa(*condo)), nil
	}
	return acesso.Escopo{Tipo: acesso.TipoAdministradora, ID: empresa}, nil
}

func recortarMetadadosAdministrativos(a acesso.Acesso, m admin.Mapeamento, permissao acesso.Permissao) admin.Mapeamento {
	out := admin.Mapeamento{}.Normalizado()
	empresas := map[string]bool{}
	for _, c := range m.Condominios {
		if acesso.Permitido(a, permissao, acesso.EscopoCondominio(c.Administradora, c.ID)) {
			out.Condominios = append(out.Condominios, c)
			empresas[c.Administradora] = true
		}
	}
	for _, e := range m.Administradoras {
		if empresas[e.ID] || acesso.Permitido(a, permissao, acesso.Escopo{Tipo: acesso.TipoAdministradora, ID: e.ID}) {
			out.Administradoras = append(out.Administradoras, e)
		}
	}
	return out
}

func registrarConsultaAdministracao(mux *http.ServeMux, store ConsultaAdministracao, acessos *AcessosPrivados, token string) {
	mux.HandleFunc("POST "+rotaAdministracao, func(w http.ResponseWriter, r *http.Request) {
		var p pedidoAdministracao
		if !lerPedido(w, r, token, 16<<10, &p) {
			return
		}
		res := respostaAdministracao{Operacao: p.Operacao, Resultado: "recusada"}
		status := http.StatusForbidden
		defer func() {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(res)
		}()
		if _, err := uuid.Parse(p.Operacao); err != nil {
			return
		}
		ctx, cancelar := context.WithTimeout(r.Context(), prazoAdministracao)
		defer cancelar()
		principal, err := acessos.carregarPrincipal(ctx, p.Prova)
		if err != nil {
			if !errors.Is(err, atende.ErrEscopoSessao) {
				res.Resultado, status = "indisponivel", http.StatusServiceUnavailable
			}
			return
		}
		var m admin.Mapeamento
		switch p.Acao {
		case "metadados", "escopos_historico":
			permissao := acesso.SkillsLer
			if p.Acao == "escopos_historico" {
				permissao = acesso.GanchosHistorico
			}
			if !acesso.PermitidoEmAlgum(principal, permissao) {
				return
			}
			m, err = store.Ler(ctx)
			if err == nil {
				m = recortarMetadadosAdministrativos(principal, m, permissao)
			}
		case "conteudo":
			alvo, e := alvoAdministracao(p.Empresa, p.Condominio)
			if e != nil || !acesso.Permitido(principal, acesso.SkillsLer, alvo) {
				return
			}
			m, err = store.ConteudoConfigurado(ctx, p.Empresa, p.Condominio)
		default:
			return
		}
		if err != nil {
			res.Resultado, status = "indisponivel", http.StatusServiceUnavailable
			return
		}
		res.Resultado, res.Mapeamento, status = "lida", &m, http.StatusOK
	})
}

func (a *AdministracaoHTTP) consultar(ctx context.Context, p pedidoAdministracao) (admin.Mapeamento, error) {
	prova, ok := identidade.Prova(ctx)
	if !ok {
		return admin.Mapeamento{}, atende.ErrEscopoSessao
	}
	op, err := uuid.NewRandom()
	if err != nil {
		return admin.Mapeamento{}, err
	}
	p.Prova, p.Operacao = prova, op.String()
	var res respostaAdministracao
	c := a.cliente
	status, _, err := requisitarJSON(ctx, c.http, c.base+rotaAdministracao, c.token, p, &res, 16<<10, limiteTurno, prazoAdministracao)
	if err == nil && res.Operacao == p.Operacao {
		if status == http.StatusForbidden && res.Resultado == "recusada" {
			return admin.Mapeamento{}, atende.ErrEscopoSessao
		}
		if status == http.StatusOK && res.Resultado == "lida" && res.Mapeamento != nil {
			return *res.Mapeamento, nil
		}
	}
	return admin.Mapeamento{}, errors.Join(ErrDadosIndisponiveis, err)
}

func (a *AdministracaoHTTP) Ler(ctx context.Context) (admin.Mapeamento, error) {
	acao := "metadados"
	if p, _ := identidade.Permissao(ctx); acesso.Permissao(p) == acesso.GanchosHistorico {
		acao = "escopos_historico"
	}
	return a.consultar(ctx, pedidoAdministracao{Acao: acao})
}

func (a *AdministracaoHTTP) ConteudoConfigurado(ctx context.Context, empresa string, condo *int) (admin.Mapeamento, error) {
	return a.consultar(ctx, pedidoAdministracao{Acao: "conteudo", Empresa: empresa, Condominio: condo})
}
