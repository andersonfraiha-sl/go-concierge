package pedidos

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"entgo.io/ent/dialect/sql"
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
// histórico em pedido_eventos, só acrescentado. Pedidos vão pelo ent; eventos pelo sqlc.
type Postgres struct {
	b banco.Banco
}

func NovoPostgres(b banco.Banco) *Postgres { return &Postgres{b: b} }

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
// acrescenta ao histórico os eventos novos. A administradora e o telefone são os do contato.
func (p *Postgres) Gravar(ctx context.Context, c escopo.Contato, cart Carteira) error {
	if err := validarContato(c); err != nil {
		return err
	}
	pedidos := copiasDoContato(c, cart.Pedidos)
	linhas, err := linhasDe(pedidos)
	if err != nil {
		return fmt.Errorf("codificar pedidos de %s: %w", c.Mascarado(), err)
	}
	ctx, cancel := context.WithTimeout(ctx, timeoutConsulta)
	defer cancel()
	if err := emTransacao(ctx, p.b.Ent, func(cli *ent.Client) error { return trocarPedidos(ctx, cli, c, linhas) }); err != nil {
		return fmt.Errorf("gravar pedidos de %s: %w", c.Mascarado(), err)
	}
	// Os eventos vão depois do commit: o ent fala database/sql e o sqlc fala pgx, e os dois
	// não dividem transação. dados já tem o histórico inteiro; se esta parte falhar, o erro
	// volta, o Servico guarda a carteira como pendente e a próxima gravação acrescenta o que
	// faltou (os já gravados são ignorados).
	if err := p.b.SQL.AcrescentarEventos(ctx, eventosDe(pedidos)); err != nil {
		return fmt.Errorf("gravar histórico dos pedidos de %s: %w", c.Mascarado(), err)
	}
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

// linha é o pedido já serializado para a coluna dados.
type linha struct {
	pedido Pedido
	dados  json.RawMessage
}

func linhasDe(pedidos []Pedido) ([]linha, error) {
	out := make([]linha, 0, len(pedidos))
	for _, p := range pedidos {
		dados, err := json.Marshal(p)
		if err != nil {
			return nil, fmt.Errorf("pedido %s: %w", p.Protocolo, err)
		}
		out = append(out, linha{pedido: p, dados: dados})
	}
	return out, nil
}

// trocarPedidos grava os pedidos do contato e apaga os que saíram da carteira. O upsert só
// sobrescreve linha do próprio contato; se algum protocolo é de outro, a contagem não fecha
// e a transação é desfeita.
func trocarPedidos(ctx context.Context, cli *ent.Client, c escopo.Contato, linhas []linha) error {
	protocolos := make([]string, 0, len(linhas))
	for _, l := range linhas {
		protocolos = append(protocolos, l.pedido.Protocolo)
	}
	if _, err := cli.Pedido.Delete().Where(doContato(c), pedido.IDNotIn(protocolos...)).Exec(ctx); err != nil {
		return fmt.Errorf("apagar pedidos removidos: %w", err)
	}
	if len(linhas) == 0 {
		return nil
	}
	if err := upsertPedidos(ctx, cli, linhas); err != nil {
		return err
	}
	meus, err := cli.Pedido.Query().Where(doContato(c), pedido.IDIn(protocolos...)).Count(ctx)
	if err != nil {
		return fmt.Errorf("conferir donos dos pedidos: %w", err)
	}
	if meus != len(protocolos) {
		return errProtocoloDeOutroContato
	}
	return nil
}

func doContato(c escopo.Contato) predicate.Pedido {
	return pedido.And(pedido.AdministradoraID(c.AdministradoraID), pedido.Telefone(c.Telefone))
}

func upsertPedidos(ctx context.Context, cli *ent.Client, linhas []linha) error {
	atual, proposto := sql.Table(pedido.Table), sql.Table("excluded")
	mesmoContato := sql.And(
		sql.ColumnsEQ(atual.C(pedido.FieldAdministradoraID), proposto.C(pedido.FieldAdministradoraID)),
		sql.ColumnsEQ(atual.C(pedido.FieldTelefone), proposto.C(pedido.FieldTelefone)),
	)
	err := cli.Pedido.MapCreateBulk(linhas, func(cr *ent.PedidoCreate, i int) {
		p := linhas[i].pedido
		cr.SetID(p.Protocolo).SetAdministradoraID(p.AdministradoraID).SetCondominioID(p.CondominioID).
			SetTelefone(p.Telefone).SetCaso(p.Caso).SetEstado(p.Estado).SetDados(linhas[i].dados).
			SetCriadoEm(p.CriadoEm).SetAtualizadoEm(p.AtualizadoEm)
	}).
		OnConflict(sql.ConflictColumns(pedido.FieldID), sql.UpdateWhere(mesmoContato)).
		UpdateNewValues().
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("gravar pedidos: %w", err)
	}
	return nil
}

// eventosDe põe o histórico de cada pedido em colunas; seq é a posição no histórico,
// contando de 1. Sem eventos, os arrays vão NULL e a unnest não devolve linha.
func eventosDe(pedidos []Pedido) sqlc.AcrescentarEventosParams {
	var p sqlc.AcrescentarEventosParams
	for _, ped := range pedidos {
		for i, e := range ped.Historico {
			p.Protocolos = append(p.Protocolos, ped.Protocolo)
			p.Seqs = append(p.Seqs, int64(i+1))
			p.Tipos = append(p.Tipos, e.Tipo)
			p.Textos = append(p.Textos, e.Texto)
			p.Pors = append(p.Pors, e.Por)
			p.Quandos = append(p.Quandos, pgtype.Timestamptz{Time: e.Quando, Valid: true})
		}
	}
	return p
}

// emTransacao roda fn numa transação e desfaz tudo se ela falhar.
func emTransacao(ctx context.Context, cli *ent.Client, fn func(*ent.Client) error) error {
	tx, err := cli.Tx(ctx)
	if err != nil {
		return fmt.Errorf("abrir transação: %w", err)
	}
	if err := fn(tx.Client()); err != nil {
		if errDesfazer := tx.Rollback(); errDesfazer != nil {
			err = errors.Join(err, fmt.Errorf("desfazer transação: %w", errDesfazer))
		}
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("confirmar transação: %w", err)
	}
	return nil
}
