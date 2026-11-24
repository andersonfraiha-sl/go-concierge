package dados

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"go-concierge/internal/acesso"
	"go-concierge/internal/core/banco"
	auditoriapg "go-concierge/internal/core/dados/postgres/auditoria"
	atende "go-concierge/internal/harness/atendimento"
	"go-concierge/internal/harness/canais"
	"go-concierge/internal/harness/ganchos/auditoria"
	"go-concierge/internal/identidade"
)

const rotaConsultasAdministrativas = "/interno/dados/v1/administracao/consultas"

type MetadadosCanais interface {
	ListarMetadados(context.Context, string) ([]canais.Resumo, error)
}
type ConsultasPostgres struct {
	banco  banco.Banco
	canais MetadadosCanais
}

func NovasConsultasPostgres(b banco.Banco, canais MetadadosCanais) *ConsultasPostgres {
	return &ConsultasPostgres{banco: b, canais: canais}
}

type ConsultasHTTP struct{ cliente *Cliente }

func NovasConsultas(c *Cliente) *ConsultasHTTP { return &ConsultasHTTP{cliente: c} }

type pedidoConsultas struct {
	Operacao, Prova, Acao, Empresa string
	Filtro                         auditoriapg.Filtro
	Depois                         *auditoriapg.Cursor
	Limite                         int
	Desde                          time.Time
	IDs                            []string
}
type respostaConsultas struct {
	Operacao, Resultado string
	Linhas              []auditoria.Linha
	Proximo             *auditoriapg.Cursor
	Disparos            map[string]int
	Canais              []canais.Resumo
}

func escopoAuditoria(a acesso.Acesso) auditoriapg.Escopo {
	if acesso.Permitido(a, acesso.GanchosHistorico, acesso.Escopo{Tipo: acesso.TipoSuperlogica}) {
		return auditoriapg.Escopo{Tudo: true}
	}
	out := auditoriapg.Escopo{}
	for _, at := range a.Atribuicoes {
		e := at.Escopo
		if !acesso.Permitido(a, acesso.GanchosHistorico, e) {
			continue
		}
		switch e.Tipo {
		case acesso.TipoAdministradora:
			out.Administradoras = append(out.Administradoras, e.ID)
		case acesso.TipoCondominio:
			company, condo, canonico := acesso.CondominioDoEscopo(e)
			if !canonico {
				company, condo = a.AdministradoraDe[e.ID], e.ID
			}
			if company != "" {
				out.Condominios = append(out.Condominios, acesso.EscopoCondominio(company, condo).ID)
			}
		}
	}
	return out
}
func ocultarProtocolos(a acesso.Acesso, linhas []auditoria.Linha) {
	for i, l := range linhas {
		alvo := acesso.Escopo{Tipo: acesso.TipoSuperlogica}
		if l.CondominioID != "" {
			alvo = acesso.EscopoCondominio(l.AdministradoraID, l.CondominioID)
		} else if l.AdministradoraID != "" {
			alvo = acesso.Escopo{Tipo: acesso.TipoAdministradora, ID: l.AdministradoraID}
		}
		if !acesso.Permitido(a, acesso.ConversasVer, alvo) {
			linhas[i].Protocolo = ""
		}
	}
}
func registrarConsultasAdministrativas(mux *http.ServeMux, store *ConsultasPostgres, acessos *AcessosPrivados, token string) {
	mux.HandleFunc("POST "+rotaConsultasAdministrativas, func(w http.ResponseWriter, r *http.Request) {
		var p pedidoConsultas
		if !lerPedido(w, r, token, 64<<10, &p) {
			return
		}
		res := respostaConsultas{Operacao: p.Operacao, Resultado: "recusada"}
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
		a, err := acessos.carregarPrincipal(ctx, p.Prova)
		if err != nil {
			if !errors.Is(err, atende.ErrEscopoSessao) {
				res.Resultado, status = "indisponivel", http.StatusServiceUnavailable
			}
			return
		}
		switch p.Acao {
		case "historico":
			if !acesso.PermitidoEmAlgum(a, acesso.GanchosHistorico) || p.Limite < 1 || p.Limite > 200 || p.Filtro.Desde.IsZero() || p.Filtro.Ate.Before(p.Filtro.Desde) || p.Filtro.Ate.After(p.Filtro.Desde.AddDate(0, 12, 0)) {
				return
			}
			if p.Depois != nil && uuid.Validate(p.Depois.ID) != nil {
				return
			}
			res.Linhas, res.Proximo, err = auditoriapg.Listar(ctx, store.banco.Ent, escopoAuditoria(a), p.Filtro, p.Depois, p.Limite)
			if err == nil {
				ocultarProtocolos(a, res.Linhas)
			}
		case "disparos":
			if !acesso.Permitido(a, acesso.GanchosEditar, acesso.Escopo{Tipo: acesso.TipoSuperlogica}) || len(p.IDs) > 200 || p.Desde.IsZero() {
				return
			}
			res.Disparos, err = auditoriapg.DisparosPorRegra(ctx, store.banco.Ent, p.Desde, p.IDs)
		case "canais":
			alvo, e := alvoAdministracao(p.Empresa, nil)
			if e != nil || !acesso.Permitido(a, acesso.StudioAvancado, alvo) || store.canais == nil {
				return
			}
			res.Canais, err = store.canais.ListarMetadados(ctx, p.Empresa)
		default:
			return
		}
		if err != nil {
			res.Resultado, status = "indisponivel", http.StatusServiceUnavailable
			return
		}
		res.Resultado, status = "lida", http.StatusOK
	})
}
func (c *ConsultasHTTP) consultar(ctx context.Context, p pedidoConsultas) (respostaConsultas, error) {
	if c == nil {
		return respostaConsultas{}, ErrDadosIndisponiveis
	}
	prova, ok := identidade.Prova(ctx)
	if !ok {
		return respostaConsultas{}, atende.ErrEscopoSessao
	}
	op, err := uuid.NewRandom()
	if err != nil {
		return respostaConsultas{}, err
	}
	p.Operacao, p.Prova = op.String(), prova
	var res respostaConsultas
	cli := c.cliente
	status, _, err := requisitarJSON(ctx, cli.http, cli.base+rotaConsultasAdministrativas, cli.token, p, &res, 64<<10, limiteTurno, prazoAdministracao)
	if err == nil && res.Operacao == p.Operacao {
		if status == http.StatusForbidden && res.Resultado == "recusada" {
			return res, atende.ErrEscopoSessao
		}
		if status == http.StatusOK && res.Resultado == "lida" {
			return res, nil
		}
	}
	return res, errors.Join(ErrDadosIndisponiveis, err)
}
func (c *ConsultasHTTP) Historico(ctx context.Context, f auditoriapg.Filtro, depois *auditoriapg.Cursor, limite int) ([]auditoria.Linha, *auditoriapg.Cursor, error) {
	r, e := c.consultar(ctx, pedidoConsultas{Acao: "historico", Filtro: f, Depois: depois, Limite: limite})
	return r.Linhas, r.Proximo, e
}
func (c *ConsultasHTTP) Disparos(ctx context.Context, desde time.Time, ids []string) (map[string]int, error) {
	r, e := c.consultar(ctx, pedidoConsultas{Acao: "disparos", Desde: desde, IDs: ids})
	return r.Disparos, e
}
func (c *ConsultasHTTP) ListarMetadados(ctx context.Context, empresa string) ([]canais.Resumo, error) {
	r, e := c.consultar(ctx, pedidoConsultas{Acao: "canais", Empresa: empresa})
	return r.Canais, e
}
