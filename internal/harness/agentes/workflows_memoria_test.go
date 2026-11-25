package agentes

import (
	"context"
	"testing"

	"go-concierge/internal/harness/catalogo"
)

func TestAbrirWorkflowDoCatalogoEmMemoria(t *testing.T) {
	arquivos := map[string][]byte{
		"agentes/portaria/agent.yaml":                         []byte("nome: portaria\ntipo: operador\n"),
		"agentes/portaria/workflows/principal.md":             []byte("# Principal\nAtenda."),
		"agentes/portaria/workflows/visita.md":                []byte("Procedimento de visita."),
		"agentes/portaria/workflows/encomenda/10_validar.md":  []byte("Valide a encomenda."),
		"agentes/portaria/workflows/encomenda/20_entregar.md": []byte("Entregue a encomenda."),
	}
	cat, err := catalogo.Novo(arquivos, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	dir, err := cat.ResolveAgente("agentes/portaria")
	if err != nil {
		t.Fatal(err)
	}
	def, err := cat.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	ag, err := Build(context.Background(), cat, def, Config{Mock: true, Env: semEnv})
	if err != nil {
		t.Fatal(err)
	}
	for _, caso := range []struct {
		nome                     string
		args                     map[string]any
		conteudo, etapa, proxima string
	}{
		{"simples", map[string]any{"nome": "visita"}, "Procedimento de visita.", "", ""},
		{"primeira etapa", map[string]any{"nome": "encomenda"}, "Valide a encomenda.", "10_validar", "20_entregar"},
		{"etapa nomeada", map[string]any{"nome": "encomenda", "etapa": "20_entregar.md"}, "Entregue a encomenda.", "20_entregar", ""},
	} {
		t.Run(caso.nome, func(t *testing.T) {
			out := chama(t, ag, "abrir_workflow", caso.args)
			if out["conteudo"] != caso.conteudo || caso.etapa != "" && out["etapa"] != caso.etapa || caso.proxima != "" && out["proxima"] != caso.proxima {
				t.Fatalf("resultado inesperado: %+v", out)
			}
		})
	}
}
