package memoria

import (
	"context"
	"reflect"
	"slices"
	"sync"
	"time"

	"go-concierge/internal/banco"
	"go-concierge/internal/escopo"
)

// Cache guarda, por contato, a memória que este processo leu ou gravou, por cima do Postgres
// (write-through): Ler não vai ao banco quando o contato já está quente, e Gravar de uma
// memória igual à guardada não grava nada. Vale só com uma réplica escrevendo a memória
// (README, "Limite conhecido"). Fato não tem ponteiro: copiar a lista basta para quem lê
// mudar a sua sem mexer na guardada.
//
// ponytail: sem despejo; uma memória ocupa poucos KB. LRU quando o número de contatos ativos
// por réplica crescer.
type Cache struct {
	pg *Postgres
	mu sync.Mutex
	m  map[string]Memoria // contato (escopo.Contato.Chave) → memória como o banco a tem
}

func NovoCache(pg *Postgres) *Cache { return &Cache{pg: pg, m: map[string]Memoria{}} }

func (c *Cache) Ler(ctx context.Context, contato escopo.Contato) (Memoria, error) {
	if m, ok := c.guardada(contato); ok {
		return m, nil
	}
	m, err := c.pg.Ler(ctx, contato)
	if err != nil {
		return Memoria{}, err
	}
	c.por(contato, m, false)
	return copia(m), nil
}

func (c *Cache) Gravar(ctx context.Context, contato escopo.Contato, m Memoria) error {
	if atual, ok := c.guardada(contato); ok && mesmoConteudo(atual, comoGuardada(contato, m, atual.AtualizadaEm)) {
		return nil
	}
	if err := c.pg.Gravar(ctx, contato, m); err != nil {
		c.mu.Lock()
		delete(c.m, contato.Chave()) // o estado do banco não é mais conhecido
		c.mu.Unlock()
		return err
	}
	// timestamptz guarda microssegundos: o cache fica como uma leitura do banco ficaria
	c.por(contato, comoGuardada(contato, m, time.Now()).truncada(), true)
	return nil
}

func (c *Cache) Listar(ctx context.Context, administradoraID string) ([]Memoria, error) {
	return c.pg.Listar(ctx, administradoraID)
}

// AquecerNoLote põe no lote a leitura da memória, se o contato ainda não está quente.
func (c *Cache) AquecerNoLote(ctx context.Context, lote *banco.Lote, contato escopo.Contato) {
	if validarContato(contato) != nil {
		return
	}
	c.mu.Lock()
	_, quente := c.m[contato.Chave()]
	c.mu.Unlock()
	if quente {
		return
	}
	c.pg.LerNoLote(ctx, lote, contato, func(m Memoria) { c.por(contato, m, false) })
}

func (c *Cache) guardada(contato escopo.Contato) (Memoria, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	m, ok := c.m[contato.Chave()]
	return copia(m), ok
}

// por guarda a memória; sem sobrescrever, não troca uma gravação que chegou enquanto a
// leitura voltava.
func (c *Cache) por(contato escopo.Contato, m Memoria, sobrescrever bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, existe := c.m[contato.Chave()]; existe && !sobrescrever {
		return
	}
	c.m[contato.Chave()] = copia(m)
}

func (m Memoria) truncada() Memoria {
	m.AtualizadaEm = m.AtualizadaEm.Truncate(time.Microsecond)
	m.Fatos = slices.Clone(m.Fatos)
	for i := range m.Fatos {
		m.Fatos[i].Quando = m.Fatos[i].Quando.Truncate(time.Microsecond)
	}
	return m
}

// mesmoConteudo compara nome e fatos (AtualizadaEm muda a cada gravação e não conta).
func mesmoConteudo(a, b Memoria) bool {
	b = b.truncada()
	return a.Nome == b.Nome && reflect.DeepEqual(a.Fatos, b.Fatos)
}
