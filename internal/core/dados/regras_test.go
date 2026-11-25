package dados

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"

	"go-concierge/internal/core/administracao/regras"
	atende "go-concierge/internal/harness/atendimento"
	"go-concierge/internal/harness/canais"
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

type fonteEscopoTeste struct{ regras []regras.RegraGravada }

func (f *fonteEscopoTeste) Ativas(context.Context) ([]regras.RegraGravada, error) {
	return f.regras, nil
}
func (f *fonteEscopoTeste) ObservarAlteracoes(func([]regras.RegraGravada)) {}

func TestRegrasHTTPAutorizaEFiltraEscopo(t *testing.T) {
	condo := 42
	repo := &repositorioCanalTeste{canal: canais.Canal{ID: uuid.New(), CompanyID: "empresa-a", CondoID: &condo, Habilitado: true}}
	fonte := &fonteEscopoTeste{regras: []regras.RegraGravada{
		{ID: "global"}, {ID: "empresa", AdministradoraID: "empresa-a"},
		{ID: "condominio", AdministradoraID: "empresa-a", Condominios: []string{"42"}},
		{ID: "outro-condominio", AdministradoraID: "empresa-a", Condominios: []string{"43"}},
		{ID: "outra-empresa", AdministradoraID: "empresa-b"},
	}}
	servidor, cliente, _, err := IniciarConfigurado(Config{Turno: gravadorFunc(nil), Regras: fonte, Canais: repo})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = servidor.Fechar(context.Background()) })
	ctx := atende.ComEscopoSessao(t.Context(), "empresa-a", repo.canal.ID)
	got, err := cliente.AtivasDoEscopo(ctx, "empresa-a", &condo)
	if err != nil || len(got) != 3 || got[0].ID != "global" || got[1].ID != "empresa" || got[2].ID != "condominio" {
		t.Fatalf("regras=%+v erro=%v", got, err)
	}
	if _, err := cliente.AtivasDoEscopo(ctx, "empresa-b", &condo); !errors.Is(err, atende.ErrEscopoSessao) {
		t.Fatalf("empresa divergente: %v", err)
	}
	outro := 43
	if _, err := cliente.AtivasDoEscopo(ctx, "empresa-a", &outro); !errors.Is(err, atende.ErrEscopoSessao) {
		t.Fatalf("condomínio divergente: %v", err)
	}
}
