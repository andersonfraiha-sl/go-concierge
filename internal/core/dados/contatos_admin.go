package dados

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"go-concierge/internal/acesso"
	"go-concierge/internal/escopo"
	atende "go-concierge/internal/harness/atendimento"
	"go-concierge/internal/harness/memoria"
	"go-concierge/internal/harness/pedidos"
	"go-concierge/internal/identidade"
)

const rotaContatosAdmin = "/interno/dados/v1/contatos/administrar"

type pedidoContatoAdmin struct {
	Operacao  string            `json:"operacao"`
	Prova     string            `json:"prova"`
	Recurso   string            `json:"recurso"`
	Acao      string            `json:"acao"`
	Permissao acesso.Permissao  `json:"permissao"`
	Empresa   string            `json:"empresa"`
	Contato   escopo.Contato    `json:"contato"`
	Memoria   *memoria.Memoria  `json:"memoria,omitempty"`
	Carteira  *pedidos.Carteira `json:"carteira,omitempty"`
}

type respostaContatoAdmin struct {
	Operacao  string             `json:"operacao"`
	Resultado string             `json:"resultado"`
	Memoria   *memoria.Memoria   `json:"memoria,omitempty"`
	Carteira  *pedidos.Carteira  `json:"carteira,omitempty"`
	Memorias  []memoria.Memoria  `json:"memorias"`
	Carteiras []pedidos.Carteira `json:"carteiras"`
	Trocas    []TrocaProtocolo   `json:"trocas"`
}

func permissaoContato(p pedidoContatoAdmin) (acesso.Permissao, bool) {
	var leitura, escrita acesso.Permissao
	switch p.Recurso {
	case "memoria":
		leitura, escrita = acesso.MemoriaVer, acesso.MemoriaEsquecer
	case "pedidos":
		leitura, escrita = acesso.PedidosVer, acesso.PedidosEditar
	default:
		return "", false
	}
	if p.Permissao == "" {
		if p.Acao == "gravar" {
			return escrita, true
		}
		if p.Acao == "ler" || p.Acao == "listar" {
			return leitura, true
		}
	}
	switch p.Acao {
	case "ler":
		return p.Permissao, p.Permissao == leitura || p.Permissao == escrita
	case "listar":
		return p.Permissao, p.Permissao == leitura
	case "gravar":
		return p.Permissao, p.Permissao == escrita
	}
	return "", false
}

func (a *AcessosPrivados) autorizarContato(ctx context.Context, p pedidoContatoAdmin) error {
	permissao, ok := permissaoContato(p)
	if !ok || strings.TrimSpace(p.Empresa) == "" {
		return atende.ErrEscopoSessao
	}
	alvo := acesso.Escopo{Tipo: acesso.TipoAdministradora, ID: p.Empresa}
	if p.Acao != "listar" {
		if p.Contato.Validar() != nil || p.Contato.CompanyID != p.Empresa || p.Contato.Telefone != escopo.NormalizarTelefone(p.Contato.Telefone) {
			return atende.ErrEscopoSessao
		}
		if p.Contato.CondoID != nil {
			alvo = acesso.EscopoCondominio(p.Empresa, strconv.Itoa(*p.Contato.CondoID))
		}
	}
	if a.verificador.VerificarIntegracao(p.Prova) {
		if a.implantacao.Permite(p.Empresa) && (p.Recurso == "memoria" || p.Recurso == "pedidos" && p.Acao != "gravar") {
			return nil
		}
		return atende.ErrEscopoSessao
	}
	principal, err := a.carregarPrincipal(ctx, p.Prova)
	if err != nil {
		return err
	}
	if !acesso.Permitido(principal, permissao, alvo) {
		return atende.ErrEscopoSessao
	}
	return nil
}

func registrarContatosAdmin(mux *http.ServeMux, a *AcessosPrivados, token string) {
	mux.HandleFunc("POST "+rotaContatosAdmin, func(w http.ResponseWriter, r *http.Request) {
		var p pedidoContatoAdmin
		if !lerPedido(w, r, token, limiteMemoria, &p) {
			return
		}
		res := respostaContatoAdmin{Operacao: p.Operacao}
		responder := func(resultado string, status int) {
			res.Resultado = resultado
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(res)
		}
		if _, err := uuid.Parse(p.Operacao); err != nil {
			responder("recusada", http.StatusForbidden)
			return
		}
		ctx, cancelar := context.WithTimeout(r.Context(), prazoMemoria)
		defer cancelar()
		if err := a.autorizarContato(ctx, p); err != nil {
			if errors.Is(err, atende.ErrEscopoSessao) {
				responder("recusada", http.StatusForbidden)
			} else if p.Acao == "gravar" {
				responder("nao_aplicada", http.StatusUnprocessableEntity)
			} else {
				responder("indisponivel", http.StatusServiceUnavailable)
			}
			return
		}
		if p.Acao != "gravar" && (p.Memoria != nil || p.Carteira != nil) || p.Recurso == "memoria" && p.Carteira != nil || p.Recurso == "pedidos" && p.Memoria != nil {
			responder("recusada", http.StatusForbidden)
			return
		}
		var err error
		switch p.Recurso {
		case "memoria":
			if a.memorias == nil {
				responder("indisponivel", http.StatusServiceUnavailable)
				return
			}
			switch p.Acao {
			case "ler":
				var m memoria.Memoria
				m, err = a.memorias.Ler(ctx, p.Contato)
				res.Memoria = &m
			case "listar":
				res.Memorias, err = a.memorias.Listar(ctx, p.Empresa)
				if res.Memorias == nil {
					res.Memorias = []memoria.Memoria{}
				}
			case "gravar":
				if p.Memoria == nil {
					responder("recusada", http.StatusForbidden)
					return
				}
				var normalizada memoria.Memoria
				normalizada, err = memoria.ParaTransporte(p.Contato, *p.Memoria)
				if err != nil || validarMemoriaAdministrativa(p.Contato, normalizada, false) != nil {
					responder("recusada", http.StatusForbidden)
					return
				}
				err = a.memorias.Gravar(ctx, p.Contato, normalizada)
			}
		case "pedidos":
			if a.pedidos == nil {
				responder("indisponivel", http.StatusServiceUnavailable)
				return
			}
			switch p.Acao {
			case "ler":
				var cart pedidos.Carteira
				cart, err = a.pedidos.Ler(ctx, p.Contato)
				res.Carteira = &cart
			case "listar":
				res.Carteiras, err = a.pedidos.Listar(ctx, p.Empresa)
				if res.Carteiras == nil {
					res.Carteiras = []pedidos.Carteira{}
				}
			case "gravar":
				if p.Carteira == nil {
					responder("recusada", http.StatusForbidden)
					return
				}
				var normalizada pedidos.Carteira
				normalizada, err = pedidos.CarteiraParaTransporte(p.Contato, *p.Carteira)
				if err != nil || validarCarteiraAdministrativa(p.Contato, normalizada, false) != nil {
					responder("recusada", http.StatusForbidden)
					return
				}
				res.Trocas = make([]TrocaProtocolo, len(normalizada.Pedidos))
				for i, ped := range normalizada.Pedidos {
					if ped == nil {
						responder("recusada", http.StatusForbidden)
						return
					}
					res.Trocas[i].Antes = ped.Protocolo
				}
				err = a.pedidos.Gravar(ctx, p.Contato, normalizada)
				if err == nil {
					for i, ped := range normalizada.Pedidos {
						res.Trocas[i].Depois = ped.Protocolo
					}
				}
			}
		}
		if err != nil {
			res = respostaContatoAdmin{Operacao: p.Operacao}
			if p.Acao == "gravar" {
				if errors.Is(err, atende.ErrGravacaoNaoAplicada) {
					responder("nao_aplicada", http.StatusUnprocessableEntity)
				} else {
					responder("incerta", http.StatusInternalServerError)
				}
			} else {
				responder("indisponivel", http.StatusServiceUnavailable)
			}
			return
		}
		responder("concluida", http.StatusOK)
	})
}

func (c *Cliente) contatoAdmin(ctx context.Context, p pedidoContatoAdmin) (respostaContatoAdmin, error) {
	prova, ok := identidade.Prova(ctx)
	if !ok {
		return respostaContatoAdmin{}, errors.Join(atende.ErrGravacaoNaoAplicada, atende.ErrEscopoSessao)
	}
	p.Prova = prova
	if permissao, ok := identidade.Permissao(ctx); ok {
		p.Permissao = acesso.Permissao(permissao)
	}
	if p.Acao != "listar" {
		p.Contato = p.Contato.Canonico()
		p.Empresa = p.Contato.CompanyID
	}
	op, err := uuid.NewRandom()
	if err != nil {
		return respostaContatoAdmin{}, errors.Join(atende.ErrGravacaoNaoAplicada, err)
	}
	p.Operacao = op.String()
	var res respostaContatoAdmin
	escrita := p.Acao == "gravar"
	status, enviada, err := requisitarJSON(ctx, c.http, c.base+rotaContatosAdmin, c.token, p, &res, limiteMemoria, limiteMemoria, prazoMemoria)
	if err != nil {
		if escrita && enviada {
			return res, erroIncerto(op, err)
		}
		if escrita {
			return res, errors.Join(atende.ErrGravacaoNaoAplicada, err)
		}
		return res, errors.Join(ErrDadosIndisponiveis, err)
	}
	if res.Operacao != p.Operacao {
		if escrita {
			return res, erroIncerto(op, errors.New("resposta administrativa inválida"))
		}
		return res, ErrDadosIndisponiveis
	}
	if status == http.StatusForbidden && res.Resultado == "recusada" {
		return res, errors.Join(atende.ErrGravacaoNaoAplicada, atende.ErrGravacaoRecusada, atende.ErrEscopoSessao)
	}
	if escrita && status == http.StatusUnprocessableEntity && res.Resultado == "nao_aplicada" {
		return res, atende.ErrGravacaoNaoAplicada
	}
	if status != http.StatusOK || res.Resultado != "concluida" {
		if escrita {
			return res, erroIncerto(op, errors.New("gravação administrativa sem confirmação"))
		}
		return res, ErrDadosIndisponiveis
	}
	return res, nil
}

func validarMemoriaAdministrativa(c escopo.Contato, m memoria.Memoria, obrigatoria bool) error {
	c = c.Canonico()
	if c.Validar() != nil || (obrigatoria || m.CompanyID != "") && m.CompanyID != c.CompanyID || (obrigatoria || m.Telefone != "") && m.Telefone != c.Telefone {
		return atende.ErrEscopoSessao
	}
	for _, f := range m.Fatos {
		if !c.Permite(f.CondoID) {
			return atende.ErrEscopoSessao
		}
	}
	return nil
}

func validarCarteiraAdministrativa(c escopo.Contato, cart pedidos.Carteira, obrigatoria bool) error {
	c = c.Canonico()
	if c.Validar() != nil || (obrigatoria || cart.CompanyID != "") && cart.CompanyID != c.CompanyID || (obrigatoria || cart.Telefone != "") && cart.Telefone != c.Telefone {
		return atende.ErrEscopoSessao
	}
	for _, p := range cart.Pedidos {
		if p == nil || !c.Permite(p.CondoID) || (obrigatoria || p.CompanyID != "") && p.CompanyID != c.CompanyID || (obrigatoria || p.Telefone != "") && p.Telefone != c.Telefone {
			return atende.ErrEscopoSessao
		}
	}
	return nil
}

func restaurarAliasesCarteira(cart *pedidos.Carteira) {
	cart.AdministradoraID = cart.CompanyID
	for _, p := range cart.Pedidos {
		p.AdministradoraID = p.CompanyID
		if p.CondoID != nil {
			p.CondominioID = strconv.Itoa(*p.CondoID)
		}
	}
}

func adotarTrocas(cart pedidos.Carteira, trocas []TrocaProtocolo) error {
	if len(trocas) != len(cart.Pedidos) || trocas == nil {
		return errors.New("protocolos ausentes")
	}
	usados := map[string]bool{}
	for i, troca := range trocas {
		if cart.Pedidos[i] == nil || cart.Pedidos[i].Protocolo != troca.Antes || troca.Depois == "" || usados[troca.Depois] {
			return errors.New("protocolos inconsistentes")
		}
		usados[troca.Depois] = true
	}
	for i, troca := range trocas {
		cart.Pedidos[i].Protocolo = troca.Depois
	}
	return nil
}
