package pedidos

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"go-concierge/internal/banco"
	"go-concierge/internal/banco/ent"
	"go-concierge/internal/banco/ent/pedido"
	"go-concierge/internal/banco/ent/predicate"
	"go-concierge/internal/banco/sqlc"
	"go-concierge/internal/escopo"
)

// timeoutConsulta limita cada leitura ou gravação dos pedidos.
const timeoutConsulta = 3 * time.Second

// Postgres guarda cada pedido numa linha de pedidos (o pedido inteiro em dados) e o
// histórico em pedido_eventos, só acrescentado. Leituras vão pelo ent; a gravação, pela função
// gravar_pedidos via sqlc.
type Postgres struct {
	b       banco.Banco
	sortear func() string // protocolo para o pedido novo que colide
}

func NovoPostgres(b banco.Banco) *Postgres { return &Postgres{b: b, sortear: NovoProtocolo} }

func (p *Postgres) Ler(ctx context.Context, c escopo.Contato) (Carteira, error) {
	if err := validarContato(c); err != nil {
		return Carteira{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, timeoutConsulta)
	defer cancel()
	linhas, err := p.b.Ent.Pedido.Query().
		Where(doContato(c)).
		All(ctx)
	if err != nil {
		return Carteira{}, fmt.Errorf("ler pedidos de %s: %w", c.Mascarado(), err)
	}
	return carteiraDe(c, linhas)
}

// Gravar troca os pedidos do contato pelos da carteira (os que saíram são apagados) e
// acrescenta ao histórico os eventos novos, tudo numa ida ao banco. A administradora e o
// telefone são os do contato.
func (p *Postgres) Gravar(ctx context.Context, c escopo.Contato, cart Carteira) error {
	if err := validarContato(c); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, timeoutConsulta)
	defer cancel()
	pedidos := copiasDoContato(c, cart.Pedidos)
	err := p.gravar(ctx, c, pedidos)
	if errors.Is(err, ErrProtocoloEmUso) {
		// raro: um pedido novo sorteou o protocolo de outro contato e a gravação foi desfeita
		pedidos, err = redesenharColisoes(pedidos, p.deOutros(ctx, c), p.sortear)
		if err == nil {
			err = p.gravar(ctx, c, pedidos)
		}
	}
	if err != nil {
		return fmt.Errorf("gravar pedidos de %s: %w", c.Mascarado(), err)
	}
	adotarProtocolos(cart.Pedidos, pedidos)
	return nil
}

// sqlstateProtocoloAlheio é o erro que gravar_pedidos levanta quando um protocolo já é de
// outro contato (migration 20261103180000_gravar_pedidos.sql).
const sqlstateProtocoloAlheio = "CC001"

func (p *Postgres) gravar(ctx context.Context, c escopo.Contato, pedidos []Pedido) error {
	params, err := paramsDeGravacao(c, pedidos)
	if err != nil {
		return err
	}
	err = p.b.SQL.GravarPedidos(ctx, params)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == sqlstateProtocoloAlheio {
		return ErrProtocoloEmUso
	}
	return err
}

// deOutros consulta, dentre os protocolos, os que já são de outro contato. Só roda depois
// de uma colisão.
func (p *Postgres) deOutros(ctx context.Context, c escopo.Contato) deOutros {
	return func(protocolos []string) (map[string]bool, error) {
		ids, err := p.b.Ent.Pedido.Query().Where(pedido.IDIn(protocolos...), pedido.Not(doContato(c))).IDs(ctx)
		if err != nil {
			return nil, fmt.Errorf("procurar protocolos de outros contatos: %w", err)
		}
		out := make(map[string]bool, len(ids))
		for _, id := range ids {
			out[id] = true
		}
		return out, nil
	}
}

// Listar devolve as carteiras da administradora com algum pedido, a atualizada por último
// primeiro.
func (p *Postgres) Listar(ctx context.Context, administradoraID string) ([]Carteira, error) {
	if err := validarAdministradora(administradoraID); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, timeoutConsulta)
	defer cancel()
	// ponytail: lista a administradora inteira, como a interface pede; paginar quando o Console listar
	linhas, err := p.b.Ent.Pedido.Query().
		Where(pedido.AdministradoraID(administradoraID)).
		Order(ent.Asc(pedido.FieldTelefone)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("listar pedidos da administradora %s: %w", administradoraID, err)
	}
	out := []Carteira{}
	for doTelefone := range porTelefone(linhas) {
		c := escopo.Contato{AdministradoraID: administradoraID, Telefone: doTelefone[0].Telefone}
		cart, err := carteiraDe(c, doTelefone)
		if err != nil {
			return nil, err
		}
		out = append(out, cart)
	}
	slices.SortStableFunc(out, maisRecentesPrimeiro)
	return out, nil
}

// porTelefone percorre as linhas, já ordenadas por telefone, um contato por vez.
func porTelefone(linhas []*ent.Pedido) func(yield func([]*ent.Pedido) bool) {
	return func(yield func([]*ent.Pedido) bool) {
		for inicio := 0; inicio < len(linhas); {
			fim := inicio + 1
			for fim < len(linhas) && linhas[fim].Telefone == linhas[inicio].Telefone {
				fim++
			}
			if !yield(linhas[inicio:fim]) {
				return
			}
			inicio = fim
		}
	}
}

func carteiraDe(c escopo.Contato, linhas []*ent.Pedido) (Carteira, error) {
	pedidos := make([]*Pedido, 0, len(linhas))
	for _, l := range linhas {
		var ped Pedido
		if err := json.Unmarshal(l.Dados, &ped); err != nil {
			return Carteira{}, fmt.Errorf("ler pedido %s de %s: %w", l.ID, c.Mascarado(), err)
		}
		pedidos = append(pedidos, &ped)
	}
	return montar(c, pedidos), nil
}

func doContato(c escopo.Contato) predicate.Pedido {
	return pedido.And(pedido.AdministradoraID(c.AdministradoraID), pedido.Telefone(c.Telefone))
}

// paramsDeGravacao põe os pedidos e o histórico de cada um em colunas; seq é a posição no
// histórico, contando de 1. Os arrays nunca são nil: nil vira NULL e "protocolo <> ALL(NULL)"
// não apagaria nada.
func paramsDeGravacao(c escopo.Contato, pedidos []Pedido) (sqlc.GravarPedidosParams, error) {
	n := len(pedidos)
	p := sqlc.GravarPedidosParams{
		AdministradoraID: c.AdministradoraID, Telefone: c.Telefone,
		Protocolos: make([]string, 0, n), Condominios: make([]string, 0, n), Casos: make([]string, 0, n),
		Estados: make([]string, 0, n), Dados: make([]string, 0, n),
		Criados: make([]pgtype.Timestamptz, 0, n), Atualizados: make([]pgtype.Timestamptz, 0, n),
		EventoProtocolos: []string{}, EventoSeqs: []int64{}, EventoTipos: []string{},
		EventoTextos: []string{}, EventoPors: []string{}, EventoQuandos: []pgtype.Timestamptz{},
	}
	for _, ped := range pedidos {
		dados, err := json.Marshal(ped)
		if err != nil {
			return sqlc.GravarPedidosParams{}, fmt.Errorf("codificar pedido %s: %w", ped.Protocolo, err)
		}
		p.Protocolos = append(p.Protocolos, ped.Protocolo)
		p.Condominios = append(p.Condominios, ped.CondominioID)
		p.Casos = append(p.Casos, ped.Caso)
		p.Estados = append(p.Estados, ped.Estado)
		p.Dados = append(p.Dados, string(dados))
		p.Criados = append(p.Criados, quando(ped.CriadoEm))
		p.Atualizados = append(p.Atualizados, quando(ped.AtualizadoEm))
		for i, e := range ped.Historico {
			p.EventoProtocolos = append(p.EventoProtocolos, ped.Protocolo)
			p.EventoSeqs = append(p.EventoSeqs, int64(i+1))
			p.EventoTipos = append(p.EventoTipos, e.Tipo)
			p.EventoTextos = append(p.EventoTextos, e.Texto)
			p.EventoPors = append(p.EventoPors, e.Por)
			p.EventoQuandos = append(p.EventoQuandos, quando(e.Quando))
		}
	}
	return p, nil
}

func quando(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }
