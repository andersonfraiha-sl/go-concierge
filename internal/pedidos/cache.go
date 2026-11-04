package pedidos

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"go-concierge/internal/banco"
	"go-concierge/internal/escopo"
)

// Cache guarda, por contato, a carteira que este processo leu ou gravou, por cima do Postgres
// (write-through): Ler não vai ao banco quando o contato já está quente, e Gravar de uma
// carteira igual à guardada não grava nada. Vale só com uma réplica escrevendo os pedidos
// (README, "Limite conhecido"). A carteira fica serializada: quem lê ganha uma cópia própria
// para mudar à vontade.
//
// ponytail: sem despejo; um contato ocupa uns poucos KB. LRU quando o número de contatos
// ativos por réplica crescer.
type Cache struct {
	pg *Postgres
	mu sync.Mutex
	m  map[string][]byte // contato (escopo.Contato.Chave) → []Pedido em JSON
}

func NovoCache(pg *Postgres) *Cache { return &Cache{pg: pg, m: map[string][]byte{}} }

func (c *Cache) Ler(ctx context.Context, contato escopo.Contato) (Carteira, error) {
	if cart, ok, err := c.guardada(contato); ok || err != nil {
		return cart, err
	}
	cart, err := c.pg.Ler(ctx, contato)
	if err != nil {
		return Carteira{}, err
	}
	c.guardarSeAusente(contato, cart)
	return cart, nil
}

func (c *Cache) Gravar(ctx context.Context, contato escopo.Contato, cart Carteira) error {
	if c.igual(contato, cart) {
		return nil
	}
	if err := c.pg.Gravar(ctx, contato, cart); err != nil {
		c.esquecer(contato) // o estado do banco não é mais conhecido
		return err
	}
	c.guardar(contato, cart)
	return nil
}

func (c *Cache) Listar(ctx context.Context, administradoraID string) ([]Carteira, error) {
	return c.pg.Listar(ctx, administradoraID)
}

// AquecerNoLote põe no lote a leitura da carteira, se o contato ainda não está quente.
func (c *Cache) AquecerNoLote(ctx context.Context, lote *banco.Lote, contato escopo.Contato) {
	if c.quente(contato) || validarContato(contato) != nil {
		return
	}
	c.pg.LerNoLote(ctx, lote, contato, func(cart Carteira) { c.guardarSeAusente(contato, cart) })
}

// GravarNoLote põe no lote a gravação da carteira, se ela mudou, e devolve o que fazer depois
// que o lote for confirmado (nil: nada a gravar). Lote que falhar: chame Gravar.
func (c *Cache) GravarNoLote(ctx context.Context, lote *banco.Lote, contato escopo.Contato, cart Carteira) (confirmar func(), err error) {
	if c.igual(contato, cart) {
		return nil, nil
	}
	if err := c.pg.GravarNoLote(ctx, lote, contato, cart); err != nil {
		return nil, err
	}
	return func() { c.guardar(contato, cart) }, nil
}

func (c *Cache) quente(contato escopo.Contato) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.m[contato.Chave()]
	return ok
}

func (c *Cache) guardada(contato escopo.Contato) (Carteira, bool, error) {
	c.mu.Lock()
	raw, ok := c.m[contato.Chave()]
	c.mu.Unlock()
	if !ok {
		return Carteira{}, false, nil
	}
	var pedidos []*Pedido
	if err := json.Unmarshal(raw, &pedidos); err != nil {
		return Carteira{}, false, fmt.Errorf("ler pedidos de %s do cache: %w", contato.Mascarado(), err)
	}
	return montar(contato, pedidos), true, nil
}

// serializada é a carteira como o banco a teria: pedidos com o contato de quem grava.
func serializada(contato escopo.Contato, cart Carteira) ([]byte, error) {
	return json.Marshal(copiasDoContato(contato, cart.Pedidos))
}

func (c *Cache) igual(contato escopo.Contato, cart Carteira) bool {
	raw, err := serializada(contato, cart)
	if err != nil {
		return false // Gravar devolve o erro de codificação
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	guardada, ok := c.m[contato.Chave()]
	return ok && bytes.Equal(guardada, raw)
}

func (c *Cache) guardar(contato escopo.Contato, cart Carteira) { c.por(contato, cart, true) }

// guardarSeAusente não sobrescreve uma gravação que chegou enquanto a leitura voltava.
func (c *Cache) guardarSeAusente(contato escopo.Contato, cart Carteira) { c.por(contato, cart, false) }

func (c *Cache) por(contato escopo.Contato, cart Carteira, sobrescrever bool) {
	raw, err := serializada(contato, cart)
	c.mu.Lock()
	defer c.mu.Unlock()
	chave := contato.Chave()
	if _, existe := c.m[chave]; existe && !sobrescrever {
		return
	}
	if err != nil {
		delete(c.m, chave)
		return
	}
	c.m[chave] = raw
}

func (c *Cache) esquecer(contato escopo.Contato) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.m, contato.Chave())
}
