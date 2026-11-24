package dados

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"go-concierge/internal/core/administracao/regras"
	"go-concierge/internal/harness/ganchos"
)

type fonteRegrasTeste struct {
	mu       sync.Mutex
	observar func([]regras.RegraGravada)
	falha    atomic.Bool
	chamadas atomic.Int32
}

func (f *fonteRegrasTeste) Ativas(context.Context) ([]regras.RegraGravada, error) {
	f.chamadas.Add(1)
	if f.falha.Load() {
		return nil, errors.New("banco indisponível")
	}
	return []regras.RegraGravada{{ID: "regra", Versao: 1, Ativa: true}}, nil
}

func (f *fonteRegrasTeste) ObservarAlteracoes(observar func([]regras.RegraGravada)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.observar = observar
}

func (f *fonteRegrasTeste) confirmar(versao int, ativa bool) {
	f.mu.Lock()
	observar := f.observar
	f.mu.Unlock()
	observar([]regras.RegraGravada{{ID: "regra", Versao: versao, Ativa: ativa}})
}

func TestRegrasHTTPCarregamEAtualizamOUnicoIndice(t *testing.T) {
	fonte := &fonteRegrasTeste{}
	servidor, cliente, _, err := IniciarConfigurado(Config{Turno: gravadorFunc(nil), Regras: fonte})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = servidor.Fechar(context.Background()) })
	var versao, quantidade atomic.Int32
	cache := regras.NovoCache(cliente, func(rs []regras.RegraGravada) (*ganchos.Indice, []regras.ErroDeCarga) {
		quantidade.Store(int32(len(rs)))
		if len(rs) > 0 {
			versao.Store(int32(rs[0].Versao))
		}
		return &ganchos.Indice{}, nil
	}, nil)
	if err := cache.Aquecer(t.Context()); err != nil {
		t.Fatal(err)
	}
	if versao.Load() != 1 || fonte.chamadas.Load() != 1 || cache.Indice() == nil {
		t.Fatal("aquecimento não percorreu HTTP")
	}
	fonte.confirmar(2, true)
	if versao.Load() != 2 || quantidade.Load() != 1 {
		t.Fatal("escrita confirmada não atualizou índice")
	}
	fonte.confirmar(1, true)
	if versao.Load() != 2 {
		t.Fatal("aviso antigo substituiu versão nova")
	}
	fonte.falha.Store(true)
	if _, err := cliente.Ativas(t.Context()); !errors.Is(err, ErrDadosIndisponiveis) {
		t.Fatalf("falha virou lista vazia: %v", err)
	}
	if versao.Load() != 2 || quantidade.Load() != 1 {
		t.Fatal("falha apagou índice vigente")
	}
	fonte.confirmar(3, false)
	if quantidade.Load() != 0 {
		t.Fatal("desativação não atualizou índice")
	}
}
