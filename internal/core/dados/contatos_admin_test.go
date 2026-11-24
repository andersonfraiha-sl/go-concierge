package dados

import (
	"context"
	"errors"
	"testing"
	"time"

	"go-concierge/internal/acesso"
	"go-concierge/internal/core/banco/bancotest"
	memoriapg "go-concierge/internal/core/dados/postgres/memoria"
	pedidospg "go-concierge/internal/core/dados/postgres/pedidos"
	"go-concierge/internal/escopo"
	atende "go-concierge/internal/harness/atendimento"
	"go-concierge/internal/harness/canais"
	"go-concierge/internal/harness/memoria"
	"go-concierge/internal/identidade"
)

func TestContatosAdminHTTPPermissoesIntegracaoERevogacao(t *testing.T) {
	b := bancotest.Novo(t)
	r, contato, _ := registroValido(t)
	autoridade, err := identidade.NovaAutoridade()
	if err != nil {
		t.Fatal(err)
	}
	acessos := &acessoSessoesAdminTeste{}
	imp := escopo.Implantacao{AdministradoraFixa: contato.CompanyID}
	admin := NovosAcessosPrivados(autoridade.Verificador(), acessos, imp).ComContatos(memoriapg.NovoPostgres(b), pedidospg.NovoPostgres(b))
	repo := &repositorioCanalTeste{canal: canais.Canal{ID: r.ChannelID, CompanyID: r.CompanyID}}
	servidor, cliente, _, err := IniciarCompleto(gravadorFunc(nil), repo, atende.NovaSessaoEmMemoria(), memoriaIndisponivel{}, pedidosIndisponiveis{}, auditoriaTeste{}, admin)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = servidor.Fechar(context.Background()) })
	prova, err := autoridade.Emitir("operador")
	if err != nil {
		t.Fatal(err)
	}
	ctx := identidade.ComProva(identidade.ComUsuario(t.Context(), "operador"), prova)
	ler := identidade.ComPermissao(ctx, string(acesso.MemoriaVer))
	gravar := identidade.ComPermissao(ctx, string(acesso.MemoriaEsquecer))
	memorias := NovasMemorias(cliente)
	m, err := memorias.Ler(ler, contato)
	if err != nil || len(m.Fatos) != 0 || m.CompanyID != contato.CompanyID {
		t.Fatalf("memória inicial: %+v erro=%v", m, err)
	}
	m.Fatos = []memoria.Fato{{Chave: "preferencia", Texto: "teste", CondoID: contato.CondoID, Quando: time.Now(), Confianca: 1}}
	if err := memorias.Gravar(gravar, contato, m); err != nil {
		t.Fatal(err)
	}
	if lida, err := memorias.Ler(ler, contato); err != nil || len(lida.Fatos) != 1 || lida.AdministradoraID != contato.CompanyID {
		t.Fatalf("memória após escrita: %+v erro=%v", lida, err)
	}
	outro := contato
	condo := 43
	outro.CondoID = &condo
	if _, err := memorias.Ler(ler, outro); !errors.Is(err, atende.ErrGravacaoRecusada) {
		t.Fatalf("outro condomínio: %v", err)
	}
	invalida := m
	invalida.Fatos = []memoria.Fato{{Chave: "invasao", Texto: "fora", CondoID: &condo}}
	if err := memorias.Gravar(gravar, contato, invalida); !errors.Is(err, atende.ErrGravacaoRecusada) || errors.Is(err, atende.ErrGravacaoIncerta) {
		t.Fatalf("fato fora do alvo: %v", err)
	}
	if _, err := memorias.Listar(ler, contato.CompanyID); !errors.Is(err, atende.ErrGravacaoRecusada) {
		t.Fatalf("listagem ampliou atribuição: %v", err)
	}
	acessos.revogado.Store(true)
	if _, err := memorias.Ler(ler, contato); !errors.Is(err, atende.ErrGravacaoRecusada) {
		t.Fatalf("acesso revogado: %v", err)
	}
	acessos.revogado.Store(false)
	provaIntegracao, err := autoridade.EmitirIntegracao()
	if err != nil {
		t.Fatal(err)
	}
	servico := identidade.ComProva(identidade.ComUsuario(t.Context(), "integracao"), provaIntegracao)
	lista, err := memorias.Listar(servico, contato.CompanyID)
	if err != nil || len(lista) != 1 {
		t.Fatalf("integração lista memória: %d erro=%v", len(lista), err)
	}
	if _, err := memorias.Listar(servico, "outra-empresa"); !errors.Is(err, atende.ErrGravacaoRecusada) {
		t.Fatalf("integração fora da implantação: %v", err)
	}
	carteiras := NovosPedidos(cliente)
	cart, err := carteiras.Ler(servico, contato)
	if err != nil || len(cart.Pedidos) != 0 || cart.CompanyID != contato.CompanyID {
		t.Fatalf("integração lê pedidos: %+v erro=%v", cart, err)
	}
	if err := carteiras.Gravar(servico, contato, cart); !errors.Is(err, atende.ErrGravacaoRecusada) {
		t.Fatalf("integração ganhou escrita de pedidos: %v", err)
	}
	if err := carteiras.Gravar(identidade.ComPermissao(ctx, string(acesso.PedidosEditar)), contato, cart); err != nil {
		t.Fatalf("usuário edita carteira: %v", err)
	}
	if _, err := cliente.Carregar(servico, "integracao", imp); !errors.Is(err, ErrDadosIndisponiveis) {
		t.Fatalf("prova de integração virou usuário: %v", err)
	}
	if err := memorias.Gravar(servico, contato, m); err != nil {
		t.Fatalf("integração preserva exclusão: %v", err)
	}
}
