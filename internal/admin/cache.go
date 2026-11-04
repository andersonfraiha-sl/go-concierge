package admin

import (
	"context"
	"sync"
)

// Cache guarda o mapeamento lido do Postgres para o caminho do turno, que o lê a cada
// mensagem. Não tem prazo: cada escrita confirmada pelo Postgres de origem substitui o valor
// guardado (NovoCache liga isso). Escrita de outro processo (outra réplica, importar-json) só
// aparece aqui depois de reiniciar: o serviço assume uma réplica (README, "Limite conhecido").
// Erro de leitura não é guardado: sem valor, cada Ler vai ao banco e falha fechado como antes.
//
// Ordem: cada escrita chega com a versão do commit dela, e uma versão menor que a guardada é
// ignorada (duas escritas não se atropelam). Uma leitura fria só guarda o que leu se nenhuma
// escrita chegou enquanto ela ia ao banco (a leitura pode ser de antes da escrita).
//
// O Mapeamento guardado é compartilhado entre os turnos e tratado como imutável: quem lê não
// pode mudar os mapas dele (PayloadsDe e OrientacaoDe devolvem cópias).
//
// O Cache embute o Postgres para as escritas (que passam pelo mesmo aoAlterar); só o turno o
// usa. Admin e RBAC continuam com o *Postgres, que lê o que acabou de escrever.
type Cache struct {
	*Postgres
	lerDoBanco func(context.Context) (Mapeamento, error)
	carregando sync.Mutex // uma leitura do banco por vez quando frio
	mu         sync.Mutex
	valor      Mapeamento
	quente     bool
	versao     uint64 // versão da última escrita guardada
	escritas   uint64 // quantas escritas chegaram: muda enquanto uma leitura fria vai ao banco
}

// NovoCache devolve o cache do mapeamento de p e passa a receber as escritas de p. p só pode
// ter um cache.
func NovoCache(p *Postgres) *Cache {
	c := &Cache{Postgres: p, lerDoBanco: p.Ler}
	p.aoAlterar = c.guardar
	return c
}

func (c *Cache) Ler(ctx context.Context) (Mapeamento, error) {
	if m, ok := c.guardado(); ok {
		return m, nil
	}
	c.carregando.Lock()
	defer c.carregando.Unlock()
	if m, ok := c.guardado(); ok {
		return m, nil
	}
	c.mu.Lock()
	antes := c.escritas
	c.mu.Unlock()
	m, err := c.lerDoBanco(ctx)
	if err != nil {
		return Mapeamento{}, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.escritas == antes {
		c.valor, c.quente = m, true
	}
	return m, nil
}

func (c *Cache) guardado() (Mapeamento, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.valor, c.quente
}

func (c *Cache) guardar(m Mapeamento, versao uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.escritas++
	if versao < c.versao {
		return
	}
	c.valor, c.quente, c.versao = m, true, versao
}
