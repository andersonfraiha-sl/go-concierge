package regras

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"go-concierge/internal/ganchos"
)

// prazoAquecer limita a leitura das ativas no boot.
const prazoAquecer = 5 * time.Second

// Cache guarda as regras ativas e o índice compilado delas para o turno, que o lê a cada
// mensagem sem lock. Cada escrita confirmada pelo Postgres de origem chega aqui (NovoCache
// liga isso), recompila o índice inteiro e troca o ponteiro: o turno vê o índice antigo ou o
// novo, nunca metade. Escrita de outro processo só aparece depois de reiniciar (uma réplica).
//
// Ordem: a versão de cada regra cresce no banco; um aviso com versão menor ou igual à já
// guardada daquela regra é ignorado, inclusive a de uma regra que já foi desativada.
type Cache struct {
	lerAtivas    func(context.Context) ([]RegraGravada, error)
	compilar     Compilador
	aoRecarregar func(resultado string)

	mu      sync.Mutex // uma recompilação por vez, na ordem em que os avisos são aplicados
	ativas  map[string]RegraGravada
	versoes map[string]int // última versão vista por regra, ativa ou não
	indice  atomic.Pointer[ganchos.Indice]
}

// NovoCache devolve o cache das regras de p e passa a receber as escritas de p. p só pode ter
// um cache. aoRecarregar recebe "ok" ou "parcial" a cada recompilação (pode ser nil).
func NovoCache(p *Postgres, compilar Compilador, aoRecarregar func(resultado string)) *Cache {
	if aoRecarregar == nil {
		aoRecarregar = func(string) {}
	}
	c := &Cache{
		lerAtivas:    p.Ativas,
		compilar:     compilar,
		aoRecarregar: aoRecarregar,
		ativas:       map[string]RegraGravada{},
		versoes:      map[string]int{},
	}
	p.aoAlterar = c.guardar
	return c
}

// Aquecer lê as ativas do banco e publica o primeiro índice. Erro de leitura não é guardado.
func (c *Cache) Aquecer(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, prazoAquecer)
	defer cancel()
	regras, err := c.lerAtivas(ctx)
	if err != nil {
		return fmt.Errorf("aquecer cache de regras: %w", err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, r := range regras {
		c.aplicar(r)
	}
	c.recompilar()
	return nil
}

// Indice devolve o índice publicado; nil só antes de Aquecer.
func (c *Cache) Indice() *ganchos.Indice { return c.indice.Load() }

func (c *Cache) guardar(r RegraGravada) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.aplicar(r) {
		return
	}
	c.recompilar()
}

// aplicar guarda r se ela for mais nova que a versão vista; devolve se guardou.
func (c *Cache) aplicar(r RegraGravada) bool {
	if r.Versao <= c.versoes[r.ID] {
		return false
	}
	c.versoes[r.ID] = r.Versao
	if r.Ativa {
		c.ativas[r.ID] = r
	} else {
		delete(c.ativas, r.ID)
	}
	return true
}

func (c *Cache) recompilar() {
	ordenadas := slices.SortedFunc(maps.Values(c.ativas), func(a, b RegraGravada) int { return cmp.Compare(a.ID, b.ID) })
	indice, erros := c.compilar(ordenadas)
	if indice == nil {
		indice = &ganchos.Indice{}
	}
	c.indice.Store(indice)
	for _, e := range erros {
		slog.Error("regra fora do índice", "operacao", "compilar_regras", "regra_id", e.RegraID, "erro", e.Err)
	}
	if len(erros) > 0 {
		c.aoRecarregar("parcial")
		return
	}
	c.aoRecarregar("ok")
}
