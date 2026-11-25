package dados

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"

	"go-concierge/internal/core/banco/bancotest"
	corecatalogo "go-concierge/internal/core/catalogo"
	atende "go-concierge/internal/harness/atendimento"
	"go-concierge/internal/harness/canais"
	"go-concierge/internal/harness/catalogo"
	"go-concierge/internal/harness/provedores/mcp"
	"go-concierge/internal/harness/provedores/nexus"
)

type catalogoExecucaoTeste struct {
	arquivos  []corecatalogo.Arquivo
	listagens atomic.Int32
	recortes  atomic.Int32
}

func (c *catalogoExecucaoTeste) Listar(context.Context) ([]corecatalogo.Arquivo, error) {
	c.listagens.Add(1)
	return c.arquivos, nil
}

func (c *catalogoExecucaoTeste) ListarParaExecucao(_ context.Context, agente, conta string) ([]corecatalogo.Arquivo, error) {
	c.recortes.Add(1)
	selecao, err := corecatalogo.SelecionarExecucao(agente, conta)
	if err != nil {
		return nil, err
	}
	arquivos := []corecatalogo.Arquivo{}
	for _, arquivo := range c.arquivos {
		if selecao.Inclui(arquivo.Caminho) {
			arquivos = append(arquivos, arquivo)
		}
	}
	return arquivos, nil
}

type mcpCatalogoExecucaoTeste struct{}

func (mcpCatalogoExecucaoTeste) ListTools(context.Context) ([]mcp.Tool, error) {
	return []mcp.Tool{{Name: "comum", Description: "via MCP"}}, nil
}

func TestCatalogoExecucaoHTTPComNexusSimulado(t *testing.T) {
	for _, caso := range []struct {
		nome        string
		status      int
		json        string
		esperados   []string
		requisicoes int32
	}{
		{nome: "sucesso", status: http.StatusOK, json: `[{"name":"comum","description":"via Nexus"},{"name":"remoto"}]`, esperados: []string{"comum", "remoto"}, requisicoes: 1},
		{nome: "sucesso vazio", status: http.StatusOK, json: `[]`, esperados: []string{"comum"}, requisicoes: 1},
		{nome: "fallback", status: http.StatusServiceUnavailable, esperados: []string{"comum", "fixture"}, requisicoes: 2},
	} {
		t.Run(caso.nome, func(t *testing.T) {
			var chamadas atomic.Int32
			gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				chamadas.Add(1)
				if r.Method != http.MethodGet || r.URL.RequestURI() != "/v1/accounts/conta-padrao/resources" || r.Header.Get("Authorization") != "Bearer token-simulado" {
					t.Errorf("consulta do catálogo inesperada: %s %s", r.Method, r.URL.RequestURI())
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				w.WriteHeader(caso.status)
				_, _ = fmt.Fprint(w, caso.json)
			}))
			t.Cleanup(gateway.Close)
			condo, outroCondo := 42, 43
			repo := &repositorioCanalTeste{canal: canais.Canal{
				ID: uuid.New(), CompanyID: "empresa", CondoID: &condo, Habilitado: true,
				Agente: "agentes/raiz", Integracoes: canais.Integracoes{NexusAccountID: "outra"},
			}}
			store := &catalogoExecucaoTeste{arquivos: []corecatalogo.Arquivo{
				{Caminho: "agentes/raiz/agent.yaml", Conteudo: []byte("nome: Raiz\ntipo: operador\n")},
				{Caminho: "nexus/conta-padrao.yaml", Conteudo: []byte("recursos:\n  - name: comum\n    kind: api\n  - name: fixture\n    kind: api\n")},
				{Caminho: "nexus/outra.yaml", Conteudo: []byte("recursos:\n  - name: isolado\n    kind: api\n")},
			}}
			servidor, cliente, _, err := IniciarConfigurado(Config{
				Turno: gravadorFunc(nil), Canais: repo, Catalogo: store, ContaPadrao: "conta-padrao",
				RecursosNexus: nexus.FonteAoVivo(nexus.New(gateway.URL, "token-simulado"), "conta-padrao"),
			})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = servidor.Fechar(context.Background()) })
			ctx := atende.ComEscopoSessao(t.Context(), "empresa", repo.canal.ID)
			fonte := &catalogo.Fonte{Dados: cliente, MCP: mcpCatalogoExecucaoTeste{}}
			if _, err := fonte.Carregar(ctx, "empresa", &outroCondo); !errors.Is(err, atende.ErrEscopoSessao) || chamadas.Load() != 0 {
				t.Fatalf("escopo indevido: erro=%v chamadas=%d", err, chamadas.Load())
			}
			for range 2 {
				cat, err := fonte.Carregar(ctx, "empresa", &condo)
				if err != nil {
					t.Fatal(err)
				}
				recursos := cat.RecursosNexus()
				var nomes []string
				for _, recurso := range recursos {
					nomes = append(nomes, recurso.Name)
				}
				if !reflect.DeepEqual(nomes, caso.esperados) || recursos[0].Description != "via MCP" {
					t.Fatalf("recursos inesperados: %+v", recursos)
				}
			}
			if total := chamadas.Load(); total != caso.requisicoes {
				t.Fatalf("requisições ao Nexus: obtido %d, esperado %d", total, caso.requisicoes)
			}
		})
	}
}
func (c *catalogoExecucaoTeste) Substituir(context.Context, []corecatalogo.Arquivo, ...string) (corecatalogo.Mudanca, error) {
	return corecatalogo.Mudanca{}, nil
}

func TestCatalogoExecucaoHTTPAutorizaCanalEIncluiSomenteFamilia(t *testing.T) {
	condo := 42
	repo := &repositorioCanalTeste{canal: canais.Canal{ID: uuid.New(), CompanyID: "empresa", CondoID: &condo, Habilitado: true, Agente: "agentes/pai/agentes/filho", Integracoes: canais.Integracoes{NexusAccountID: "outra"}}}
	store := &catalogoExecucaoTeste{arquivos: []corecatalogo.Arquivo{
		{Caminho: "agentes/pai/agent.yaml", Conteudo: []byte("pai")},
		{Caminho: "agentes/pai/knowledge/dado.md", Conteudo: []byte("herança")},
		{Caminho: "agentes/pai/agentes/filho/agent.yaml", Conteudo: []byte("filho")},
		{Caminho: "agentes/pai/agentes/filho/agentes/neto/agent.yaml", Conteudo: []byte("neto")},
		{Caminho: "agentes/pai/agentes/irmao/agent.yaml", Conteudo: []byte("irmão")},
		{Caminho: "agentes/outra/agent.yaml", Conteudo: []byte("outro")},
		{Caminho: "nexus/conta.yaml", Conteudo: []byte("recursos: []")},
		{Caminho: "nexus/outra.yaml", Conteudo: []byte("privado")},
	}}
	servidor, cliente, _, err := IniciarConfigurado(Config{Turno: gravadorFunc(nil), Canais: repo, Catalogo: store, ContaPadrao: "conta"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = servidor.Fechar(context.Background()) })
	ctx := atende.ComEscopoSessao(t.Context(), "empresa", repo.canal.ID)
	retrato, err := cliente.CatalogoDoEscopo(ctx, "empresa", &condo)
	if err != nil {
		t.Fatal(err)
	}
	if retrato.Agente != repo.canal.Agente || retrato.Conta != "conta" || len(retrato.Arquivos) != 5 {
		t.Fatalf("retrato: agente=%q conta=%q arquivos=%d", retrato.Agente, retrato.Conta, len(retrato.Arquivos))
	}
	if _, ok := retrato.Arquivos["agentes/pai/knowledge/dado.md"]; !ok {
		t.Fatal("herança ausente")
	}
	if _, ok := retrato.Arquivos["agentes/pai/agentes/filho/agentes/neto/agent.yaml"]; !ok {
		t.Fatal("subagente ausente")
	}
	if _, ok := retrato.Arquivos["agentes/pai/agentes/irmao/agent.yaml"]; ok {
		t.Fatal("irmão fora do escopo incluído")
	}
	if _, err := cliente.CatalogoDoEscopo(ctx, "outra", &condo); !errors.Is(err, atende.ErrEscopoSessao) {
		t.Fatalf("empresa divergente: %v", err)
	}
	outro := 43
	if _, err := cliente.CatalogoDoEscopo(ctx, "empresa", &outro); !errors.Is(err, atende.ErrEscopoSessao) {
		t.Fatalf("condomínio divergente: %v", err)
	}
	if store.listagens.Load() != 0 || store.recortes.Load() != 1 {
		t.Fatalf("consultas: listagens gerais=%d recortes=%d", store.listagens.Load(), store.recortes.Load())
	}
}

func TestCatalogoExecucaoPostgresComFallbackNexusSimulado(t *testing.T) {
	banco := bancotest.Novo(t)
	store := corecatalogo.NovoPostgres(banco.Ent)
	_, err := store.Substituir(t.Context(), []corecatalogo.Arquivo{
		{Caminho: "agentes/raiz/agent.yaml", Conteudo: []byte("nome: Raiz\ntipo: operador\n")},
		{Caminho: "agentes/outro/agent.yaml", Conteudo: []byte("nome: Outro\n")},
		{Caminho: "nexus/conta-padrao.yaml", Conteudo: []byte("recursos:\n  - name: fixture\n    kind: api\n")},
	})
	if err != nil {
		t.Fatal(err)
	}
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }))
	t.Cleanup(gateway.Close)
	condo := 42
	repo := &repositorioCanalTeste{canal: canais.Canal{ID: uuid.New(), CompanyID: "empresa", CondoID: &condo, Habilitado: true, Agente: "agentes/raiz"}}
	servidor, cliente, _, err := IniciarConfigurado(Config{
		Turno: gravadorFunc(nil), Canais: repo, Catalogo: store, ContaPadrao: "conta-padrao",
		RecursosNexus: nexus.FonteAoVivo(nexus.New(gateway.URL, "token-simulado"), "conta-padrao"),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = servidor.Fechar(context.Background()) })
	ctx := atende.ComEscopoSessao(t.Context(), "empresa", repo.canal.ID)
	retrato, err := cliente.CatalogoDoEscopo(ctx, "empresa", &condo)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := retrato.Arquivos["agentes/outro/agent.yaml"]; ok {
		t.Fatal("arquivo de agente alheio foi incluído")
	}
	fonte := &catalogo.Fonte{Dados: cliente}
	cat, err := fonte.Carregar(ctx, "empresa", &condo)
	if err != nil {
		t.Fatal(err)
	}
	recursos := cat.RecursosNexus()
	if len(recursos) != 1 || recursos[0].Name != "fixture" {
		t.Fatalf("fallback inesperado: %+v", recursos)
	}
}
