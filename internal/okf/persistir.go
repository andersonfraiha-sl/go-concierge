package okf

import (
	"errors"
	"fmt"
)

// ErrPersistir marca o erro do gancho Persistir: a escrita foi para o disco, mas o catálogo
// não chegou ao banco. Quem responde ao cliente trata como falha do servidor.
var ErrPersistir = errors.New("persistir catálogo")

// Os métodos públicos de escrita ficam aqui: cada um segura c.mu, faz a escrita no disco e,
// se deu certo, chama Persistir ainda sob a trava, para duas escritas não se misturarem na
// leitura que o gancho faz do disco. Quem já segura c.mu (skills.go) chama os privados e
// persiste por conta própria; os privados não travam nem persistem.

// persistir roda o gancho, se houver. O erro volta para quem chamou a escrita.
func (c *Catalogo) persistir() error {
	if c.Persistir == nil {
		return nil
	}
	if err := c.Persistir(); err != nil {
		return fmt.Errorf("%w: %w", ErrPersistir, err)
	}
	return nil
}

// Escrever roda fn, uma escrita no catálogo feita fora do Catalogo (o instantâneo de
// recursos em nexus/), sob a trava, e persiste se ela deu certo. fn não pode chamar métodos
// do Catalogo que travam.
func (c *Catalogo) Escrever(fn func() error) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := fn(); err != nil {
		return err
	}
	return c.persistir()
}

// CriarAgente monta a pasta com os arquivos-modelo e registra a rota no pai.
func (c *Catalogo) CriarAgente(n NovoAgente) (string, []string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	rel, mudancas, err := c.criarAgente(n)
	if err != nil {
		return "", nil, err
	}
	if err := c.persistir(); err != nil {
		return "", nil, err
	}
	return rel, mudancas, nil
}

// SalvarArquivo grava (cria ou substitui). agent.yaml e caso.yaml precisam ser válidos.
func (c *Catalogo) SalvarArquivo(agenteRel, rel string, conteudo []byte) error {
	return c.Escrever(func() error { return c.salvarArquivo(agenteRel, rel, conteudo) })
}

// ExcluirArquivo remove um arquivo (nunca o agent.yaml).
func (c *Catalogo) ExcluirArquivo(agenteRel, rel string) error {
	return c.Escrever(func() error { return c.excluirArquivo(agenteRel, rel) })
}

// SalvarConfig atualiza os campos principais do agent.yaml preservando o resto.
func (c *Catalogo) SalvarConfig(agenteRel string, campos map[string]any) error {
	return c.Escrever(func() error { return c.salvarConfig(agenteRel, campos) })
}

// VincularRecurso liga ou desliga um recurso do Nexus em agent.yaml → recursos.
func (c *Catalogo) VincularRecurso(agenteRel, nome string, ligar bool) ([]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	mudancas, err := c.vincularRecurso(agenteRel, nome, ligar)
	if err != nil {
		return nil, err
	}
	if err := c.persistir(); err != nil {
		return nil, err
	}
	return mudancas, nil
}

// ImportarZip instala (ou substitui) um agente a partir do zip da pasta.
func (c *Catalogo) ImportarZip(agenteRel string, dados []byte) ([]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	arquivos, err := c.importarZip(agenteRel, dados)
	if err != nil {
		return nil, err
	}
	if err := c.persistir(); err != nil {
		return nil, err
	}
	return arquivos, nil
}
