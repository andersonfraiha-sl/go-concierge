package dados

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"go-concierge/internal/acesso"
	"go-concierge/internal/core/administracao/regras"
	atende "go-concierge/internal/harness/atendimento"
	"go-concierge/internal/identidade"
)

const rotaRegrasAdministrativas = "/interno/dados/v1/regras/administrar"

type RegrasAdministrativas interface {
	regras.FonteDoIndice
	Ler(context.Context, string) (regras.RegraGravada, error)
	Listar(context.Context, string, int) ([]regras.RegraGravada, error)
	LerVersao(context.Context, string, int) (regras.VersaoGravada, error)
	VersoesAtuais(context.Context, []string) (map[string]int, error)
	Salvar(context.Context, regras.RegraGravada, int) (regras.RegraGravada, error)
	SalvarVarias(context.Context, []regras.RegraGravada, []int) ([]regras.RegraGravada, error)
}
type RegrasHTTP struct{ cliente *Cliente }

func NovasRegrasAdministrativas(c *Cliente) *RegrasHTTP { return &RegrasHTTP{cliente: c} }
func (c *RegrasHTTP) Ativas(ctx context.Context) ([]regras.RegraGravada, error) {
	return c.cliente.Ativas(ctx)
}
func (c *RegrasHTTP) ObservarAlteracoes(f func([]regras.RegraGravada)) {
	c.cliente.ObservarAlteracoes(f)
}

type pedidoRegrasAdmin struct {
	Operacao, Prova, Acao, ID, Depois string
	Versao, Limite                    int
	IDs                               []string
	Regras                            []regras.RegraGravada
	Versoes                           []int
}
type respostaRegrasAdmin struct {
	Operacao, Resultado string
	Regras              []regras.RegraGravada
	Versao              *regras.VersaoGravada
	Versoes             map[string]int
}

func podeImportarRegras(a acesso.Acesso) bool {
	if acesso.Permitido(a, acesso.GanchosEditar, acesso.Escopo{Tipo: acesso.TipoSuperlogica}) {
		return true
	}
	fixa := a.Implantacao.AdministradoraFixa
	return fixa != "" && acesso.Permitido(a, acesso.StudioAvancado, acesso.Escopo{Tipo: acesso.TipoAdministradora, ID: fixa})
}
func podeAlvoRegra(a acesso.Acesso, r regras.RegraGravada, historico bool) bool {
	alvo := regras.Alvo{Administradora: r.AdministradoraID, Condominios: r.Condominios}
	alvos := []acesso.Escopo{}
	if len(alvo.Condominios) > 0 {
		for _, c := range alvo.Condominios {
			if _, err := strconv.Atoi(c); err != nil {
				return false
			}
			alvos = append(alvos, acesso.EscopoCondominio(alvo.Administradora, c))
		}
	} else if alvo.Administradora != "" {
		alvos = append(alvos, acesso.Escopo{Tipo: acesso.TipoAdministradora, ID: alvo.Administradora})
	} else if a.Implantacao.AdministradoraFixa != "" {
		alvos = append(alvos, acesso.Escopo{Tipo: acesso.TipoAdministradora, ID: a.Implantacao.AdministradoraFixa})
	} else {
		alvos = append(alvos, acesso.Escopo{Tipo: acesso.TipoSuperlogica})
	}
	for _, e := range alvos {
		if historico {
			if !acesso.Permitido(a, acesso.GanchosEditar, e) && !acesso.Permitido(a, acesso.GanchosHistorico, e) {
				return false
			}
		} else if !acesso.Permitido(a, acesso.GanchosEditar, e) && !(a.Implantacao.AdministradoraFixa != "" && acesso.Permitido(a, acesso.StudioAvancado, e)) {
			return false
		}
	}
	return true
}

func registrarRegrasAdministrativas(mux *http.ServeMux, repo RegrasAdministrativas, acessos *AcessosPrivados, token string) {
	mux.HandleFunc("POST "+rotaRegrasAdministrativas, func(w http.ResponseWriter, r *http.Request) {
		var p pedidoRegrasAdmin
		if !lerPedido(w, r, token, limiteTurno, &p) {
			return
		}
		res := respostaRegrasAdmin{Operacao: p.Operacao, Resultado: "recusada"}
		status := http.StatusForbidden
		defer func() {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(res)
		}()
		if _, err := uuid.Parse(p.Operacao); err != nil {
			return
		}
		ctx, cancelar := context.WithTimeout(r.Context(), prazoRegras)
		defer cancelar()
		a, err := acessos.carregarPrincipal(ctx, p.Prova)
		if err != nil {
			if !errors.Is(err, atende.ErrEscopoSessao) {
				res.Resultado, status = "indisponivel", http.StatusServiceUnavailable
			}
			return
		}
		global := acesso.Permitido(a, acesso.GanchosEditar, acesso.Escopo{Tipo: acesso.TipoSuperlogica})
		escrita := p.Acao == "salvar"
		switch p.Acao {
		case "ler":
			if !global {
				return
			}
			var v regras.RegraGravada
			v, err = repo.Ler(ctx, p.ID)
			res.Regras = []regras.RegraGravada{v}
		case "listar":
			if !global || p.Limite < 1 || p.Limite > 200 {
				return
			}
			res.Regras, err = repo.Listar(ctx, p.Depois, p.Limite)
		case "versao":
			if p.Versao < 1 {
				return
			}
			var v regras.VersaoGravada
			v, err = repo.LerVersao(ctx, p.ID, p.Versao)
			if err == nil {
				var def regras.Regra
				def, err = regras.Decodificar(v.Definicao)
				if err == nil && !podeAlvoRegra(a, regras.RegraGravada{AdministradoraID: def.Alvo.Administradora, Condominios: def.Alvo.Condominios}, true) {
					return
				}
				res.Versao = &v
			}
		case "versoes":
			if !podeImportarRegras(a) || len(p.IDs) > 1000 {
				return
			}
			for _, id := range p.IDs {
				v, e := repo.Ler(ctx, id)
				if e != nil && !errors.Is(e, regras.ErrRegraDesconhecida) {
					err = e
					break
				}
				if e == nil && !podeAlvoRegra(a, v, false) {
					return
				}
			}
			if err == nil {
				res.Versoes, err = repo.VersoesAtuais(ctx, p.IDs)
			}
		case "salvar":
			if !podeImportarRegras(a) || len(p.Regras) == 0 || len(p.Regras) > 1000 || len(p.Regras) != len(p.Versoes) {
				return
			}
			for i, v := range p.Regras {
				if !podeAlvoRegra(a, v, false) {
					return
				}
				anterior, e := repo.Ler(ctx, v.ID)
				if e != nil && !errors.Is(e, regras.ErrRegraDesconhecida) {
					err = e
					break
				}
				if e == nil && !podeAlvoRegra(a, anterior, false) {
					return
				}
				p.Regras[i].AlteradaPor = a.UsuarioID
			}
			if err != nil {
				res.Resultado, status = "nao_aplicada", http.StatusServiceUnavailable
				return
			}
			res.Regras, err = repo.SalvarVarias(ctx, p.Regras, p.Versoes)
		default:
			return
		}
		if err != nil {
			switch {
			case errors.Is(err, regras.ErrRegraDesconhecida):
				res.Resultado, status = "ausente", http.StatusNotFound
			case errors.Is(err, regras.ErrVersaoVelha):
				res.Resultado, status = "conflito", http.StatusConflict
			case escrita:
				res.Resultado, status = "incerta", http.StatusInternalServerError
			default:
				res.Resultado, status = "indisponivel", http.StatusServiceUnavailable
			}
			return
		}
		if res.Regras == nil {
			res.Regras = []regras.RegraGravada{}
		}
		res.Resultado, status = "concluida", http.StatusOK
	})
}

func (c *RegrasHTTP) executar(ctx context.Context, p pedidoRegrasAdmin) (respostaRegrasAdmin, error) {
	prova, ok := identidade.Prova(ctx)
	if !ok {
		return respostaRegrasAdmin{}, atende.ErrEscopoSessao
	}
	op, err := uuid.NewRandom()
	if err != nil {
		return respostaRegrasAdmin{}, err
	}
	p.Prova, p.Operacao = prova, op.String()
	var res respostaRegrasAdmin
	cli := c.cliente
	status, enviada, err := requisitarJSON(ctx, cli.http, cli.base+rotaRegrasAdministrativas, cli.token, p, &res, limiteTurno, limiteTurno, prazoRegras)
	if err == nil && res.Operacao == p.Operacao {
		if status == http.StatusOK && res.Resultado == "concluida" {
			return res, nil
		}
		switch res.Resultado {
		case "recusada":
			return res, errors.Join(atende.ErrGravacaoNaoAplicada, atende.ErrEscopoSessao)
		case "ausente":
			return res, regras.ErrRegraDesconhecida
		case "conflito":
			return res, regras.ErrVersaoVelha
		case "nao_aplicada":
			return res, errors.Join(atende.ErrGravacaoNaoAplicada, ErrDadosIndisponiveis)
		}
	}
	if p.Acao != "salvar" {
		return res, errors.Join(ErrDadosIndisponiveis, err)
	}
	if !enviada {
		return res, errors.Join(atende.ErrGravacaoNaoAplicada, err)
	}
	return res, erroIncerto(op, err)
}
func (c *RegrasHTTP) Ler(ctx context.Context, id string) (regras.RegraGravada, error) {
	r, e := c.executar(ctx, pedidoRegrasAdmin{Acao: "ler", ID: id})
	if e != nil {
		return regras.RegraGravada{}, e
	}
	if len(r.Regras) != 1 {
		return regras.RegraGravada{}, ErrDadosIndisponiveis
	}
	return r.Regras[0], nil
}
func (c *RegrasHTTP) Listar(ctx context.Context, depois string, limite int) ([]regras.RegraGravada, error) {
	r, e := c.executar(ctx, pedidoRegrasAdmin{Acao: "listar", Depois: depois, Limite: limite})
	return r.Regras, e
}
func (c *RegrasHTTP) LerVersao(ctx context.Context, id string, versao int) (regras.VersaoGravada, error) {
	r, e := c.executar(ctx, pedidoRegrasAdmin{Acao: "versao", ID: id, Versao: versao})
	if e != nil {
		return regras.VersaoGravada{}, e
	}
	if r.Versao == nil {
		return regras.VersaoGravada{}, ErrDadosIndisponiveis
	}
	return *r.Versao, nil
}
func (c *RegrasHTTP) VersoesAtuais(ctx context.Context, ids []string) (map[string]int, error) {
	r, e := c.executar(ctx, pedidoRegrasAdmin{Acao: "versoes", IDs: ids})
	return r.Versoes, e
}
func (c *RegrasHTTP) Salvar(ctx context.Context, v regras.RegraGravada, versao int) (regras.RegraGravada, error) {
	rs, e := c.SalvarVarias(ctx, []regras.RegraGravada{v}, []int{versao})
	if e != nil {
		return regras.RegraGravada{}, e
	}
	return rs[0], nil
}
func (c *RegrasHTTP) SalvarVarias(ctx context.Context, rs []regras.RegraGravada, versoes []int) ([]regras.RegraGravada, error) {
	r, e := c.executar(ctx, pedidoRegrasAdmin{Acao: "salvar", Regras: rs, Versoes: versoes})
	if e == nil && len(r.Regras) != len(rs) {
		return nil, atende.ErrGravacaoIncerta
	}
	return r.Regras, e
}
