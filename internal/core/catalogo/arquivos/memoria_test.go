package arquivos

import (
	"bytes"
	"encoding/json"
	"maps"
	"path/filepath"
	"strings"
	"testing"

	"go-concierge/internal/core/catalogo"
)

func TestLeituraEmMemoriaEquivaleAoCatalogoDaSemente(t *testing.T) {
	raiz := filepath.Join("..", "..", "..", "..", "okf")
	arquivos, err := catalogo.LerDisco(raiz)
	if err != nil {
		t.Fatal(err)
	}
	emDisco, err := NewCatalogo(raiz)
	if err != nil {
		t.Fatal(err)
	}
	emMemoria, err := NovoEmMemoria(arquivos)
	if err != nil {
		t.Fatal(err)
	}
	disco, err := emDisco.Tree()
	if err != nil {
		t.Fatal(err)
	}
	memoria, err := emMemoria.Tree()
	if err != nil {
		t.Fatal(err)
	}
	a, err := json.Marshal(disco)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(memoria)
	if err != nil {
		t.Fatal(err)
	}
	if strings.ReplaceAll(string(a), emDisco.Root, "/catalogo") != string(b) {
		t.Fatal("árvore em memória diverge da árvore lida da semente")
	}
	rel := "agentes/concierge"
	original, err := emDisco.LerArquivo(rel, PersonaMD)
	if err != nil {
		t.Fatal(err)
	}
	lido, err := emMemoria.LerArquivo(rel, PersonaMD)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(original, lido) {
		t.Fatal("conteúdo em memória diverge")
	}
}

func TestEscritasDoCatalogoEmMemoria(t *testing.T) {
	raiz := filepath.Join("..", "..", "..", "..", "okf")
	arquivos, err := catalogo.LerDisco(raiz)
	if err != nil {
		t.Fatal(err)
	}
	recursos, err := LerRecursosFixture(filepath.Join(raiz, "nexus", "cond-001.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, escrita := range escritasDoCatalogo() {
		t.Run(escrita.nome, func(t *testing.T) {
			c, err := NovoEmMemoria(arquivos)
			if err != nil {
				t.Fatal(err)
			}
			c.Recursos = func() []Recurso { return recursos }
			chamadas := 0
			c.Persistir = func() error { chamadas++; return nil }
			if err := escrita.roda(t, c); err != nil {
				t.Fatal(err)
			}
			if chamadas != 1 {
				t.Fatalf("persistência chamada %d vezes", chamadas)
			}
		})
	}
}

func TestLoteEmMemoriaValidaAntesDeAplicar(t *testing.T) {
	arquivos, err := catalogo.LerDisco(filepath.Join("..", "..", "..", "..", "okf"))
	if err != nil {
		t.Fatal(err)
	}
	c, err := NovoEmMemoria(arquivos)
	if err != nil {
		t.Fatal(err)
	}
	antes := maps.Clone(c.arquivos)
	lote := []ArquivoBruto{
		{Caminho: "agentes/concierge/persona.md", Conteudo: []byte("nova persona")},
		{Caminho: "agentes/concierge/agent.yaml", Conteudo: []byte("nome: [")},
	}
	resultado, err := c.AplicarArquivos(lote, nil)
	if err != nil || len(resultado.Problemas) == 0 {
		t.Fatalf("lote inválido: resultado=%+v erro=%v", resultado, err)
	}
	for nome, anterior := range antes {
		atual, ok := c.arquivos[nome]
		if !ok || !bytes.Equal(atual.Data, anterior.Data) {
			t.Fatalf("arquivo %s mudou após rejeição", nome)
		}
	}
	if len(c.arquivos) != len(antes) {
		t.Fatal("quantidade de arquivos mudou após rejeição")
	}
}

func TestMemoriaCopiaBytesERecusaCaminhoInvalido(t *testing.T) {
	dados := []byte("original")
	c, err := NovoEmMemoria([]catalogo.Arquivo{{Caminho: "agentes/a/agent.yaml", Conteudo: dados}})
	if err != nil {
		t.Fatal(err)
	}
	dados[0] = 'X'
	lido, err := c.readFile("/catalogo/agentes/a/agent.yaml")
	if err != nil || string(lido) != "original" {
		t.Fatalf("conteúdo %q: %v", lido, err)
	}
	if _, err := NovoEmMemoria([]catalogo.Arquivo{{Caminho: "../fora", Conteudo: dados}}); err == nil {
		t.Fatal("aceitou caminho fora do catálogo")
	}
}
