package pedidos

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"go-concierge/internal/banco"
	"go-concierge/internal/banco/ent"
	"go-concierge/internal/banco/ent/pedido"
	"go-concierge/internal/banco/sqlc"
	"go-concierge/internal/escopo"
)

// timeoutConsulta limita cada leitura ou gravação dos pedidos.
const timeoutConsulta = 3 * time.Second

// Postgres guarda cada pedido numa linha de pedidos (o pedido inteiro em dados) e o
// histórico em pedido_eventos, só acrescentado. Ler e Gravar são queries do sqlc (cabem nos
// lotes do turno); Listar vai pelo ent.
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
	var cart Carteira
	lote := banco.NovoLote()
	p.LerNoLote(ctx, lote, c, func(lida Carteira) { cart = lida })
	if err := lote.Enviar(ctx, p.b.Pool); err != nil {
		return Carteira{}, fmt.Errorf("ler pedidos de %s: %w", c.Mascarado(), err)
	}
	return cart, nil
}

// LerNoLote põe no lote a leitura da carteira do contato (LerPedidos); guardar recebe a
// carteira quando o lote roda. Um erro volta no Enviar do lote.
func (p *Postgres) LerNoLote(ctx context.Context, lote *banco.Lote, c escopo.Contato, guardar func(Carteira)) {
	res := lote.Queries().LerPedidos(ctx, []sqlc.LerPedidosParams{{AdministradoraID: c.AdministradoraID, Telefone: c.Telefone}})
	lote.Depois(func() error {
		var errLer error
		res.Query(func(_ int, linhas []sqlc.LerPedidosRow, err error) {
			if err != nil {
				errLer = err
				return
			}
			dados := make([]dadosDoPedido, 0, len(linhas))
			for _, l := range linhas {
				dados = append(dados, dadosDoPedido{protocolo: l.Protocolo, json: l.Dados})
			}
			var cart Carteira
			if cart, errLer = carteiraDe(c, dados); errLer == nil {
				guardar(cart)
			}
		})
		return errLer
	})
}

// Gravar troca os pedidos do contato pelos da carteira (os que saíram são apagados) e
// acrescenta ao histórico os eventos novos, numa ida ao banco. A administradora e o telefone
// são os do contato. Se um pedido novo sorteou o protocolo de outro contato, nada foi gravado:
// o protocolo é redesenhado e a gravação refeita (raro; uma ida a mais por tentativa).
func (p *Postgres) Gravar(ctx context.Context, c escopo.Contato, cart Carteira) error {
	if err := validarContato(c); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, timeoutConsulta)
	defer cancel()
	pedidos, err := p.gravarRedesenhando(ctx, c, copiasDoContato(c, cart.Pedidos))
	if err != nil {
		return fmt.Errorf("gravar pedidos de %s: %w", c.Mascarado(), err)
	}
	adotarProtocolos(cart.Pedidos, pedidos)
	return nil
}

func (p *Postgres) gravarRedesenhando(ctx context.Context, c escopo.Contato, pedidos []Pedido) ([]Pedido, error) {
	for tentativa := 0; ; tentativa++ {
		var alheios []string
		lote := banco.NovoLote()
		err := p.enfileirarGravacao(ctx, lote, c, pedidos, func(a []string) error { alheios = a; return nil })
		if err == nil {
			err = lote.Enviar(ctx, p.b.Pool)
		}
		if err != nil || len(alheios) == 0 {
			return pedidos, err
		}
		if tentativa == tentativasProtocolo {
			return nil, ErrProtocoloEmUso
		}
		// a gravação já conferiu os donos: um sorteio fora de alheios é checado na próxima
		if pedidos, err = redesenharColisoes(pedidos, conhecidos(alheios), p.sortear); err != nil {
			return nil, err
		}
	}
}

// conhecidos responde deOutros com os protocolos que a gravação devolveu como alheios.
func conhecidos(alheios []string) deOutros {
	return func(protocolos []string) (map[string]bool, error) {
		out := map[string]bool{}
		for _, p := range protocolos {
			if slices.Contains(alheios, p) {
				out[p] = true
			}
		}
		return out, nil
	}
}

// GravarNoLote põe no lote a gravação da carteira, sem redesenhar protocolo: se algum for de
// outro contato, nada dos pedidos é gravado e o Enviar do lote devolve ErrProtocoloEmUso;
// quem chamou grava de novo por Gravar.
func (p *Postgres) GravarNoLote(ctx context.Context, lote *banco.Lote, c escopo.Contato, cart Carteira) error {
	if err := validarContato(c); err != nil {
		return err
	}
	return p.enfileirarGravacao(ctx, lote, c, copiasDoContato(c, cart.Pedidos), func(alheios []string) error {
		if len(alheios) > 0 {
			return ErrProtocoloEmUso
		}
		return nil
	})
}

// enfileirarGravacao põe no lote o GravarPedidos; aoGravar recebe os protocolos que não
// foram gravados por serem de outro contato (vazio: a carteira inteira foi gravada).
func (p *Postgres) enfileirarGravacao(ctx context.Context, lote *banco.Lote, c escopo.Contato, pedidos []Pedido, aoGravar func(alheios []string) error) error {
	params, err := paramsDeGravacao(c, pedidos)
	if err != nil {
		return err
	}
	res := lote.Queries().GravarPedidos(ctx, []sqlc.GravarPedidosParams{params})
	lote.Depois(func() error {
		var errGravar error
		res.QueryRow(func(_ int, r sqlc.GravarPedidosRow, err error) {
			if err != nil {
				errGravar = err
				return
			}
			// faltando só vem com alheios vazio: outro contato gravou o protocolo no meio
			errGravar = aoGravar(append(r.Alheios, r.Faltando...))
		})
		return errGravar
	})
	return nil
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
		dados := make([]dadosDoPedido, 0, len(doTelefone))
		for _, l := range doTelefone {
			dados = append(dados, dadosDoPedido{protocolo: l.ID, json: l.Dados})
		}
		cart, err := carteiraDe(c, dados)
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

// dadosDoPedido é a coluna dados de uma linha de pedidos.
type dadosDoPedido struct {
	protocolo string
	json      []byte
}

func carteiraDe(c escopo.Contato, linhas []dadosDoPedido) (Carteira, error) {
	pedidos := make([]*Pedido, 0, len(linhas))
	for _, l := range linhas {
		var ped Pedido
		if err := json.Unmarshal(l.json, &ped); err != nil {
			return Carteira{}, fmt.Errorf("ler pedido %s de %s: %w", l.protocolo, c.Mascarado(), err)
		}
		pedidos = append(pedidos, &ped)
	}
	return montar(c, pedidos), nil
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
