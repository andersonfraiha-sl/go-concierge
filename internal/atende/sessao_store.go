package atende

import (
	"context"
	"slices"
	"sync"
	"time"
)

// RegistroSessao é a sessão como o store a guarda: as colunas que filtram e ordenam,
// e a Sessao inteira serializada (com o histórico do Eino) em Dados.
type RegistroSessao struct {
	ID, AdministradoraID, CondominioID, Protocolo string
	AtualizadaEm                                  time.Time
	Dados                                         []byte // Sessao serializada, com o histórico do Eino
}

// SessaoStore guarda as sessões do ATENDE fora do processo (sobrevivem a restart).
type SessaoStore interface {
	// Salvar grava o registro, substituindo o de mesmo ID.
	Salvar(ctx context.Context, r RegistroSessao) error
	Carregar(ctx context.Context, id string) (RegistroSessao, bool, error)
	// Listar devolve até limite sessões, mais recentes primeiro; administradoraID vazio
	// lista todas.
	Listar(ctx context.Context, administradoraID string, limite int) ([]RegistroSessao, error)
	// Remover apaga a sessão; apagar uma inexistente não é erro.
	Remover(ctx context.Context, id string) error
}

// SessaoEmMemoria é o SessaoStore dos testes unitários.
type SessaoEmMemoria struct {
	mu        sync.Mutex
	registros map[string]RegistroSessao
}

func NovaSessaoEmMemoria() *SessaoEmMemoria {
	return &SessaoEmMemoria{registros: map[string]RegistroSessao{}}
}

func (m *SessaoEmMemoria) Salvar(_ context.Context, r RegistroSessao) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r.Dados = slices.Clone(r.Dados)
	m.registros[r.ID] = r
	return nil
}

func (m *SessaoEmMemoria) Carregar(_ context.Context, id string) (RegistroSessao, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.registros[id]
	r.Dados = slices.Clone(r.Dados)
	return r, ok, nil
}

func (m *SessaoEmMemoria) Listar(_ context.Context, administradoraID string, limite int) ([]RegistroSessao, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var regs []RegistroSessao
	for _, r := range m.registros {
		if administradoraID == "" || r.AdministradoraID == administradoraID {
			r.Dados = slices.Clone(r.Dados)
			regs = append(regs, r)
		}
	}
	slices.SortFunc(regs, func(a, b RegistroSessao) int { return b.AtualizadaEm.Compare(a.AtualizadaEm) })
	return regs[:min(limite, len(regs))], nil
}

func (m *SessaoEmMemoria) Remover(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.registros, id)
	return nil
}
