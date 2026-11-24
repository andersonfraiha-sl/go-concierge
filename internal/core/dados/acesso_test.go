package dados

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"

	"go-concierge/internal/acesso"
	acessopg "go-concierge/internal/core/dados/postgres/acesso"
	"go-concierge/internal/escopo"
	atende "go-concierge/internal/harness/atendimento"
	"go-concierge/internal/harness/canais"
	"go-concierge/internal/harness/ganchos/auditoria"
	"go-concierge/internal/identidade"
)

type acessoTeste struct {
	chamadas     atomic.Int32
	revogado     atomic.Bool
	indisponivel atomic.Bool
}

func (a *acessoTeste) Carregar(_ context.Context, usuario string, imp escopo.Implantacao) (acesso.Acesso, error) {
	a.chamadas.Add(1)
	if a.indisponivel.Load() {
		return acesso.Acesso{}, errors.New("indisponível")
	}
	if usuario == "desconhecido" {
		return acesso.Acesso{}, acessopg.ErrUsuarioDesconhecido
	}
	resultado := acesso.Acesso{UsuarioID: usuario, Implantacao: imp, Papeis: map[string]acesso.Papel{"gestor": {ID: "gestor", Permissoes: []acesso.Permissao{acesso.PedidosVer}}}}
	if !a.revogado.Load() {
		resultado.Atribuicoes = []acesso.Atribuicao{{PapelID: "gestor", Escopo: acesso.Escopo{Tipo: acesso.TipoAdministradora, ID: "empresa"}}}
	}
	return resultado, nil
}

type auditoriaTeste struct{}

func (auditoriaTeste) Gravar(context.Context, string, uuid.UUID, []auditoria.Linha) error {
	return nil
}

func TestAcessoHTTPVerificaPrincipalImplantacaoERevogacao(t *testing.T) {
	r, _, _ := registroValido(t)
	repo := &repositorioCanalTeste{canal: canais.Canal{ID: r.ChannelID, CompanyID: r.CompanyID}}
	autoridade, err := identidade.NovaAutoridade()
	if err != nil {
		t.Fatal(err)
	}
	store := &acessoTeste{}
	imp := escopo.Implantacao{AdministradoraFixa: "empresa"}
	servidor, cliente, _, err := IniciarCompleto(gravadorFunc(nil), repo, atende.NovaSessaoEmMemoria(), memoriaIndisponivel{}, pedidosIndisponiveis{}, auditoriaTeste{}, NovosAcessosPrivados(autoridade.Verificador(), store, imp))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = servidor.Fechar(context.Background()) })
	prova, err := autoridade.Emitir("gestor")
	if err != nil {
		t.Fatal(err)
	}
	ctx := identidade.ComProva(t.Context(), prova)
	alvo := acesso.Escopo{Tipo: acesso.TipoAdministradora, ID: "empresa"}
	a, err := cliente.Carregar(ctx, "gestor", imp)
	if err != nil || !acesso.Permitido(a, acesso.PedidosVer, alvo) {
		t.Fatalf("acesso inicial: %+v erro=%v", a, err)
	}
	for nome, tc := range map[string]struct {
		ctx     context.Context
		usuario string
		imp     escopo.Implantacao
	}{
		"usuário diferente":     {ctx, "outro", imp},
		"assinatura adulterada": {identidade.ComProva(t.Context(), prova+"x"), "gestor", imp},
		"implantação diferente": {ctx, "gestor", escopo.Implantacao{}},
		"sem prova":             {t.Context(), "gestor", imp},
	} {
		t.Run(nome, func(t *testing.T) {
			if _, err := cliente.Carregar(tc.ctx, tc.usuario, tc.imp); !errors.Is(err, ErrDadosIndisponiveis) {
				t.Fatalf("esperada recusa, erro=%v", err)
			}
		})
	}
	if store.chamadas.Load() != 1 {
		t.Fatalf("consultas após recusas: %d", store.chamadas.Load())
	}
	store.revogado.Store(true)
	a, err = cliente.Carregar(ctx, "gestor", imp)
	if err != nil || acesso.Permitido(a, acesso.PedidosVer, alvo) || store.chamadas.Load() != 2 {
		t.Fatalf("revogação não refletida: %+v erro=%v chamadas=%d", a, err, store.chamadas.Load())
	}
	desconhecido, err := autoridade.Emitir("desconhecido")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cliente.Carregar(identidade.ComProva(t.Context(), desconhecido), "desconhecido", imp); !errors.Is(err, acessopg.ErrUsuarioDesconhecido) {
		t.Fatalf("usuário ausente: %v", err)
	}
	store.indisponivel.Store(true)
	if _, err := cliente.Carregar(ctx, "gestor", imp); !errors.Is(err, ErrDadosIndisponiveis) {
		t.Fatalf("falha de infraestrutura: %v", err)
	}
}
