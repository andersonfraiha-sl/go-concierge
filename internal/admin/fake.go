package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// EmMemoria é o Repositorio dos testes: guarda um Mapeamento. Tudo que entra e sai passa por
// JSON, como no banco: ninguém compartilha mapa com o guardado e os números dos payloads
// voltam como float64.
type EmMemoria struct {
	mu sync.Mutex
	m  Mapeamento
}

func NovaEmMemoria() *EmMemoria {
	var m Mapeamento
	m.normalizar()
	return &EmMemoria{m: m}
}

func (e *EmMemoria) Ler(context.Context) (Mapeamento, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return clonar(e.m)
}

func (e *EmMemoria) alterar(r regra) (Mapeamento, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	novo, err := r(e.m)
	if err != nil {
		return Mapeamento{}, err
	}
	guardado, err := clonar(novo)
	if err != nil {
		return Mapeamento{}, err
	}
	e.m = guardado
	return clonar(guardado)
}

func clonar(m Mapeamento) (Mapeamento, error) {
	raw, err := json.Marshal(m)
	if err != nil {
		return Mapeamento{}, fmt.Errorf("codificar mapeamento: %w", err)
	}
	var copia Mapeamento
	if err := json.Unmarshal(raw, &copia); err != nil {
		return Mapeamento{}, fmt.Errorf("decodificar mapeamento: %w", err)
	}
	copia.normalizar()
	return copia, nil
}

func (e *EmMemoria) SalvarAdministradora(_ context.Context, a Administradora) (Mapeamento, error) {
	return e.alterar(func(m Mapeamento) (Mapeamento, error) { return m.salvarAdministradora(a) })
}

func (e *EmMemoria) RemoverAdministradora(_ context.Context, id string) (Mapeamento, error) {
	return e.alterar(func(m Mapeamento) (Mapeamento, error) { return m.removerAdministradora(id) })
}

func (e *EmMemoria) SalvarCondominio(_ context.Context, c Condominio) (Mapeamento, error) {
	return e.alterar(func(m Mapeamento) (Mapeamento, error) { return m.salvarCondominio(c) })
}

func (e *EmMemoria) RemoverCondominio(_ context.Context, id string) (Mapeamento, error) {
	return e.alterar(func(m Mapeamento) (Mapeamento, error) { return m.removerCondominio(id) })
}

func (e *EmMemoria) SalvarHabilitacao(_ context.Context, condoID, caso string, h Habilitacao, limpar bool) (Mapeamento, error) {
	return e.alterar(func(m Mapeamento) (Mapeamento, error) { return m.salvarHabilitacao(condoID, caso, h, limpar) })
}

func (e *EmMemoria) SalvarOrientacao(_ context.Context, condoID string, o Orientacao) (Mapeamento, error) {
	return e.alterar(func(m Mapeamento) (Mapeamento, error) { return m.salvarOrientacao(condoID, o) })
}

func (e *EmMemoria) SalvarAjustes(_ context.Context, condoID string, ajustes map[string]map[string]any) (Mapeamento, error) {
	return e.alterar(func(m Mapeamento) (Mapeamento, error) { return m.salvarAjustes(condoID, ajustes) })
}

func (e *EmMemoria) SalvarPayload(_ context.Context, condoID, recurso string, payload map[string]any) (Mapeamento, error) {
	return e.alterar(func(m Mapeamento) (Mapeamento, error) { return m.salvarPayload(condoID, recurso, payload) })
}

func (e *EmMemoria) CriarAdministradora(_ context.Context, a Administradora) (Mapeamento, error) {
	return e.alterar(func(m Mapeamento) (Mapeamento, error) { return m.criarAdministradora(a) })
}

func (e *EmMemoria) EditarAdministradora(_ context.Context, a Administradora) (Mapeamento, error) {
	return e.alterar(func(m Mapeamento) (Mapeamento, error) { return m.editarAdministradora(a) })
}

func (e *EmMemoria) CriarCondominio(_ context.Context, c Condominio) (Mapeamento, error) {
	return e.alterar(func(m Mapeamento) (Mapeamento, error) { return m.criarCondominio(c) })
}

func (e *EmMemoria) EditarCondominio(_ context.Context, c Condominio, donaAtual string) (Mapeamento, error) {
	return e.alterar(func(m Mapeamento) (Mapeamento, error) { return m.editarCondominio(c, donaAtual) })
}
