package dados

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"go-concierge/internal/core/banco"
	auditoriapg "go-concierge/internal/core/dados/postgres/auditoria"
	pedidospg "go-concierge/internal/core/dados/postgres/pedidos"
	sessoespg "go-concierge/internal/core/dados/postgres/sessoes"
	"go-concierge/internal/escopo"
	atende "go-concierge/internal/harness/atendimento"
	"go-concierge/internal/harness/ganchos/auditoria"
	"go-concierge/internal/harness/pedidos"
)

type GravadorPostgres struct {
	banco   banco.Banco
	sessoes *sessoespg.SessaoPostgres
	pedidos *pedidospg.Postgres
}

func NovoGravadorPostgres(b banco.Banco) *GravadorPostgres {
	return &GravadorPostgres{banco: b, sessoes: sessoespg.NovoSessaoPostgres(b), pedidos: pedidospg.NovoPostgres(b)}
}

func (g *GravadorPostgres) Gravar(ctx context.Context, r atende.RegistroSessao, c escopo.Contato, cart pedidos.Carteira, linhas []auditoria.Linha) error {
	if err := validarTurno(r, c, cart, linhas); err != nil {
		return errors.Join(atende.ErrGravacaoNaoAplicada, atende.ErrGravacaoRecusada, err)
	}
	c = c.Canonico()
	ctx = atende.ComEscopoSessao(ctx, r.CompanyID, r.ChannelID)
	lote := banco.NovoLote()
	if err := g.pedidos.GravarNoLote(ctx, lote, c, cart); err != nil {
		return errors.Join(atende.ErrGravacaoNaoAplicada, err)
	}
	g.sessoes.SalvarNoLote(ctx, lote, r)
	auditoriapg.NoLote(lote, linhas)
	if err := lote.Enviar(ctx, g.banco.Pool); err != nil {
		if errors.Is(err, banco.ErrCommitIncerto) {
			return errors.Join(atende.ErrGravacaoIncerta, err)
		}
		if errors.Is(err, atende.ErrEscopoSessao) || errors.Is(err, atende.ErrRegistroSessao) {
			return errors.Join(atende.ErrGravacaoNaoAplicada, atende.ErrGravacaoRecusada, err)
		}
		return errors.Join(atende.ErrGravacaoNaoAplicada, err)
	}
	return nil
}

func validarTurno(r atende.RegistroSessao, c escopo.Contato, cart pedidos.Carteira, linhas []auditoria.Linha) error {
	if err := atende.ValidarRegistroSessao(r); err != nil {
		return err
	}
	if err := c.Validar(); err != nil {
		return err
	}
	c = c.Canonico()
	if r.CompanyID != c.CompanyID || r.UserPhone != c.Telefone || !escopo.MesmoCondominio(r.CondoID, c.CondoID) || cart.CompanyID != c.CompanyID || escopo.NormalizarTelefone(cart.Telefone) != c.Telefone {
		return errors.New("sessão, contato e carteira fora do mesmo escopo")
	}
	for _, p := range cart.Pedidos {
		if p == nil || p.CompanyID != c.CompanyID || escopo.NormalizarTelefone(p.Telefone) != c.Telefone || !c.Permite(p.CondoID) {
			return errors.New("pedido fora do escopo")
		}
	}
	condo := ""
	if c.CondoID != nil {
		condo = strconv.Itoa(*c.CondoID)
	}
	for _, linha := range linhas {
		if linha.AdministradoraID != "" && linha.AdministradoraID != c.CompanyID || linha.ChannelID != "" && linha.ChannelID != r.ChannelID.String() || linha.SessaoID != "" && linha.SessaoID != r.ID || linha.Protocolo != "" && linha.Protocolo != r.Protocolo || linha.CondominioID != "" && linha.CondominioID != condo {
			return fmt.Errorf("auditoria fora do escopo do turno")
		}
	}
	return nil
}
