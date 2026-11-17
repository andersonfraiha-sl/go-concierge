package catalogo

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func escreverNoDisco(t *testing.T, raiz, rel, conteudo string) {
	t.Helper()
	p := filepath.Join(raiz, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(conteudo), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLerDiscoLeAgentesENexusEPulaOcultos(t *testing.T) {
	raiz := t.TempDir()
	escreverNoDisco(t, raiz, "nexus/cond.yaml", "recursos: []")
	escreverNoDisco(t, raiz, "agentes/b/agent.yaml", "nome: b")
	escreverNoDisco(t, raiz, "agentes/a/agent.yaml", "nome: a")
	escreverNoDisco(t, raiz, "agentes/a/.rag/indice.bin", "oculto")
	escreverNoDisco(t, raiz, "agentes/a/.novo", "oculto")
	escreverNoDisco(t, raiz, "var/catalogo/agentes/x.md", "fora das pastas do catálogo")
	escreverNoDisco(t, raiz, "agentes.md", "arquivo solto na raiz")

	arquivos, err := LerDisco(raiz)
	if err != nil {
		t.Fatal(err)
	}
	quer := []string{"agentes/a/agent.yaml", "agentes/b/agent.yaml", "nexus/cond.yaml"}
	if got := caminhos(arquivos); !slices.Equal(got, quer) {
		t.Fatalf("caminhos = %v, quer %v", got, quer)
	}
	if string(arquivos[0].Conteudo) != "nome: a" {
		t.Fatalf("conteúdo = %q", arquivos[0].Conteudo)
	}
}

func TestLerDiscoSemAsPastasDevolveListaVazia(t *testing.T) {
	arquivos, err := LerDisco(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if arquivos == nil || len(arquivos) != 0 {
		t.Fatalf("arquivos = %#v, quer lista vazia", arquivos)
	}
}

func TestMaterializarRefazAsDuasPastas(t *testing.T) {
	raiz := t.TempDir()
	escreverNoDisco(t, raiz, "agentes/velho/agent.yaml", "velho")
	escreverNoDisco(t, raiz, "nexus/velho.yaml", "velho")
	escreverNoDisco(t, raiz, "outra/ficou.txt", "fora do catálogo")

	novos := []Arquivo{{"agentes/novo/agent.yaml", []byte("novo")}, {"agentes/novo/persona.md", []byte{0xff, 0x00, 0x01}}}
	if err := Materializar(raiz, novos); err != nil {
		t.Fatal(err)
	}

	lidos, err := LerDisco(raiz)
	if err != nil {
		t.Fatal(err)
	}
	if len(lidos) != 2 || lidos[0].Caminho != novos[0].Caminho || string(lidos[1].Conteudo) != string(novos[1].Conteudo) {
		t.Fatalf("lidos = %+v, quer %+v", lidos, novos)
	}
	if _, err := os.Stat(filepath.Join(raiz, "outra", "ficou.txt")); err != nil {
		t.Fatalf("Materializar mexeu fora de agentes/ e nexus/: %v", err)
	}
}

func TestMaterializarRecusaCaminhoQueEscapaDaRaizSemTocarNoDisco(t *testing.T) {
	casos := []struct {
		nome    string
		caminho string
	}{
		{"ponto-ponto", "agentes/../../fora.txt"},
		{"absoluto", "/tmp/fora.txt"},
		{"barra invertida", `agentes\..\..\fora.txt`},
		{"unidade", "C:/fora.txt"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			pai := t.TempDir()
			raiz := filepath.Join(pai, "raiz")
			escreverNoDisco(t, raiz, "agentes/existente/agent.yaml", "fica")

			err := Materializar(raiz, []Arquivo{{"agentes/bom.md", []byte("bom")}, {c.caminho, []byte("mau")}})

			if !errors.Is(err, ErrCaminhoInvalido) {
				t.Fatalf("erro = %v, quer ErrCaminhoInvalido", err)
			}
			if _, err := os.Stat(filepath.Join(raiz, "agentes", "existente", "agent.yaml")); err != nil {
				t.Fatalf("o disco foi tocado antes de recusar: %v", err)
			}
			if _, err := os.Stat(filepath.Join(pai, "fora.txt")); err == nil {
				t.Fatal("escreveu fora da raiz")
			}
		})
	}
}
