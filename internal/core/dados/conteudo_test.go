package dados

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"go-concierge/internal/core/administracao/camadas"
	"go-concierge/internal/escopo"
	atende "go-concierge/internal/harness/atendimento"
	"go-concierge/internal/harness/canais"
)

type conteudoTeste struct {
	versao    atomic.Int32
	chamadas  atomic.Int32
	falha     atomic.Bool
	vazio     atomic.Bool
	vazamento atomic.Bool
}

func (c *conteudoTeste) LerRetratoDoEscopo(_ context.Context, empresa string, condo *int) (camadas.Retrato, error) {
	c.chamadas.Add(1)
	if c.falha.Load() {
		return camadas.Retrato{}, errors.New("banco indisponível")
	}
	r := camadas.Retrato{Bases: map[string]camadas.VersaoBase{}, Administradoras: map[string]map[string]camadas.Cabeca{}, Condominios: map[string]map[string]camadas.Cabeca{}}
	if !c.vazio.Load() {
		r.Bases["teste"] = camadas.VersaoBase{Cabeca: camadas.Cabeca{VersaoID: int(c.versao.Load())}}
		r.Administradoras["teste"] = map[string]camadas.Cabeca{empresa: {}}
		if condo != nil {
			r.Condominios["teste"] = map[string]camadas.Cabeca{escopo.ChaveCondominio(empresa, "42"): {}}
		}
	}
	if c.vazamento.Load() {
		r.Administradoras["teste"] = map[string]camadas.Cabeca{"outra": {}}
	}
	return r, nil
}

func TestConteudoHTTPRelêEscopoESeparaVazioDeFalha(t *testing.T) {
	r, _, _ := registroValido(t)
	repo := &repositorioCanalTeste{canal: canais.Canal{ID: r.ChannelID, CompanyID: r.CompanyID, CondoID: r.CondoID}}
	store := &conteudoTeste{}
	store.versao.Store(1)
	servidor, cliente, _, err := IniciarConfigurado(Config{Turno: gravadorFunc(nil), Canais: repo, Conteudo: store})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = servidor.Fechar(context.Background()) })
	ctx := atende.ComEscopoSessao(t.Context(), r.CompanyID, r.ChannelID)
	for _, versao := range []int32{1, 2} {
		store.versao.Store(versao)
		retrato, err := cliente.LerRetratoDoEscopo(ctx, r.CompanyID, r.CondoID)
		if err != nil || retrato.Bases["teste"].VersaoID != int(versao) {
			t.Fatalf("conteúdo atualizado: %+v erro=%v", retrato, err)
		}
	}
	if _, err := cliente.LerRetratoDoEscopo(ctx, r.CompanyID, nil); !errors.Is(err, atende.ErrEscopoSessao) {
		t.Fatalf("canal fixo ampliado: %v", err)
	}
	if _, err := cliente.LerRetratoDoEscopo(ctx, "outra", r.CondoID); !errors.Is(err, atende.ErrEscopoSessao) {
		t.Fatalf("empresa cruzada: %v", err)
	}
	if store.chamadas.Load() != 2 {
		t.Fatalf("recusas chegaram ao store: %d", store.chamadas.Load())
	}
	store.falha.Store(true)
	if _, err := cliente.LerRetratoDoEscopo(ctx, r.CompanyID, r.CondoID); !errors.Is(err, ErrDadosIndisponiveis) {
		t.Fatalf("falha virou conteúdo vazio: %v", err)
	}
	store.falha.Store(false)
	store.vazio.Store(true)
	if retrato, err := cliente.LerRetratoDoEscopo(ctx, r.CompanyID, r.CondoID); err != nil || len(retrato.Bases) != 0 {
		t.Fatalf("vazio legítimo: %+v erro=%v", retrato, err)
	}
	store.vazio.Store(false)
	store.vazamento.Store(true)
	if _, err := cliente.LerRetratoDoEscopo(ctx, r.CompanyID, r.CondoID); !errors.Is(err, ErrDadosIndisponiveis) {
		t.Fatalf("conteúdo de outra empresa aceito: %v", err)
	}
}
