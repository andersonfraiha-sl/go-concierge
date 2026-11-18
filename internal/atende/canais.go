package atende

import (
	"context"
	"errors"
	"strconv"

	"github.com/google/uuid"

	"go-concierge/internal/admin"
	"go-concierge/internal/canais"
	"go-concierge/internal/entrada"
	"go-concierge/internal/escopo"
	"go-concierge/internal/gruvi"
)

type chaveCanal struct{}

func canalDoContexto(ctx context.Context) (canais.Canal, bool) {
	c, ok := ctx.Value(chaveCanal{}).(canais.Canal)
	return c, ok
}

func (s *Servico) ResolverCanal(ctx context.Context, msg entrada.Mensagem) (canais.Canal, error) {
	if s.Canais == nil {
		return canais.Canal{}, canais.ErrDesconhecido
	}
	if msg.ChannelID != uuid.Nil {
		c, err := s.Canais.PorID(ctx, msg.CompanyID, msg.ChannelID)
		if err != nil {
			return c, err
		}
		if msg.Origem.Conector != "atende-emulado" && (c.Provider != msg.Provider || c.Tipo != msg.ChannelType || c.Tenant != msg.Tenant || c.Endpoint != msg.Endpoint) {
			return canais.Canal{}, canais.ErrDesconhecido
		}
		if !c.Habilitado {
			return canais.Canal{}, canais.ErrDesabilitado
		}
		return c, nil
	}
	c, err := s.Canais.Resolver(ctx, canais.Identidade{Provider: msg.Provider, Tipo: msg.ChannelType, Tenant: msg.Tenant, Endpoint: msg.Endpoint})
	if err == nil && !c.Habilitado {
		return canais.Canal{}, canais.ErrDesabilitado
	}
	return c, err
}

func (s *Servico) autorizarMensagem(ctx context.Context, msg entrada.Mensagem) (context.Context, entrada.Mensagem, error) {
	if s.Canais == nil {
		return ctx, msg, nil
	}
	c, err := s.ResolverCanal(ctx, msg)
	if err != nil {
		return ctx, msg, err
	}
	if !s.Implantacao.Permite(c.CompanyID) {
		return ctx, msg, admin.ErrForaDaImplantacao
	}
	msg.ChannelID, msg.CompanyID = c.ID, c.CompanyID
	if msg.Contato.Phone != "" {
		msg.Contato.Phone = escopo.NormalizarTelefone(msg.Contato.Phone)
		msg.Destino.Telefone = msg.Contato.Phone
	}
	msg.Origem.ConexaoID = c.ID.String()
	msg.Destino.ChannelID = c.ID
	msg.Destino.CompanyID = c.CompanyID
	msg.Destino.ConexaoID = c.ID.String()
	ctx = context.WithValue(ctx, chaveCanal{}, c)
	return ComEscopoSessao(ctx, c.CompanyID, c.ID), msg, nil
}

func condominioDoCanal(c canais.Canal) admin.Condominio {
	id := ""
	if c.CondoID != nil {
		id = strconv.Itoa(*c.CondoID)
	}
	return admin.Condominio{ID: id, Nome: c.Nome, Administradora: c.CompanyID, CompanyID: c.Integracoes.GruviCompanyID, Account: c.Integracoes.NexusAccountID, Tenant: c.Tenant}
}

func (s *Servico) conteudoDoCanal(ctx context.Context, c canais.Canal) (admin.Mapeamento, error) {
	if repo, ok := s.Mapeamento.(interface {
		Conteudo(context.Context, string, *int) (admin.Mapeamento, error)
	}); ok {
		return repo.Conteudo(ctx, c.CompanyID, c.CondoID)
	}
	if s.Mapeamento == nil {
		return admin.Mapeamento{}, nil
	}
	return admin.Mapeamento{}, errors.New("repositório sem composição de conteúdo empresarial")
}

func (sess *Sessao) fixarCanal(c canais.Canal, phone string) error {
	contato, err := escopo.NovoContato(c.CompanyID, c.CondoID, phone)
	if err != nil {
		return err
	}
	sess.mu.Lock()
	defer sess.mu.Unlock()
	if sess.ChannelID != uuid.Nil && (sess.ChannelID != c.ID || sess.CompanyID != c.CompanyID || !mesmoCondo(sess.CondoID, c.CondoID) || sess.escopo.Telefone != contato.Telefone) {
		return errors.New("canal ou escopo da sessão divergente")
	}
	sess.ChannelID, sess.CompanyID, sess.CondoID = c.ID, c.CompanyID, c.CondoID
	sess.AdministradoraID, sess.Condominio = c.CompanyID, condominioDoCanal(c).ID
	sess.GruviCompanyID = c.Integracoes.GruviCompanyID
	sess.escopo = contato
	sess.Destino.ChannelID = c.ID
	sess.Destino.CompanyID = c.CompanyID
	return nil
}

func mesmoCondo(a, b *int) bool { return a == nil && b == nil || a != nil && b != nil && *a == *b }

func (s *Servico) contextoDaSessao(ctx context.Context, sess *Sessao) (context.Context, error) {
	if s.Canais == nil {
		return ctx, nil
	}
	c, err := s.Canais.PorID(ctx, sess.CompanyID, sess.ChannelID)
	if err != nil {
		return ctx, err
	}
	if !c.Habilitado {
		return ctx, canais.ErrDesabilitado
	}
	if err := sess.fixarCanal(c, sess.Contato.Phone); err != nil {
		return ctx, err
	}
	return context.WithValue(ComEscopoSessao(ctx, c.CompanyID, c.ID), chaveCanal{}, c), nil
}

func (s *Servico) IdentificarCanal(ctx context.Context, company string, channel uuid.UUID, phone string) (gruvi.Identidade, error) {
	c, err := s.Canais.PorID(ctx, company, channel)
	if err != nil {
		return gruvi.Identidade{}, err
	}
	if !c.Habilitado {
		return gruvi.Identidade{}, canais.ErrDesabilitado
	}
	if !s.Implantacao.Permite(c.CompanyID) {
		return gruvi.Identidade{}, admin.ErrForaDaImplantacao
	}
	ctx = context.WithValue(ctx, chaveCanal{}, c)
	id, _, err := s.identidade(ctx, escopo.NormalizarTelefone(phone), c.Integracoes.GruviCompanyID)
	return id, err
}
