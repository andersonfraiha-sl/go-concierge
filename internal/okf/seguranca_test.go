package okf

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// caminhoSeguro é o único portão entre o ?caminho= da API (/api/arquivo) e o disco: cobre
// partição de equivalência (fora de uma caixinha, caixinha inexistente, extensão fora da
// allowlist) e valor-limite (vazio, "..", ponto final, barra inicial).
func TestCaminhoSeguroRecusaTraversal(t *testing.T) {
	validos := []string{"knowledge/faq.md", "assets/logo.png", AgentYAML, PersonaMD, CasoYAML}
	for _, c := range validos {
		t.Run("aceita "+c, func(t *testing.T) {
			if _, err := caminhoSeguro(c); err != nil {
				t.Fatalf("%q deveria ser aceito: %v", c, err)
			}
		})
	}
	// "../agent.yaml" e "knowledge/../../agent.yaml" normalizam para "agent.yaml": o
	// filepath.Clean("/"+rel) prefixado trava qualquer ".." na raiz, então eles não escapam
	// (voltam ao mesmo arquivo seguro, não a um alvo fora do agente) e são aceitos.
	for _, c := range []string{"../agent.yaml", "knowledge/../../agent.yaml"} {
		t.Run("normaliza sem escapar "+c, func(t *testing.T) {
			rel, err := caminhoSeguro(c)
			if err != nil || rel != AgentYAML {
				t.Fatalf("%q: rel=%q err=%v, queria %q sem erro", c, rel, err, AgentYAML)
			}
		})
	}
	recusados := []string{
		"",
		"..",
		"../../etc/passwd",
		"knowledge/../../../etc/passwd",
		"/etc/passwd",
		"knowledge/.hidden.md",
		"knowledge/..",
		"agentes/outro/persona.md",  // agentes/ é dos subagentes, não edita por aqui
		"knowledge/script.exe",      // extensão fora da allowlist
		"knowledge/arquivo",         // sem extensão
		"caixinha-que-nao-existe/x", // nem é uma caixinha válida
		"knowledge",                 // caixinha sem arquivo dentro
	}
	for _, c := range recusados {
		t.Run("recusa "+c, func(t *testing.T) {
			if rel, err := caminhoSeguro(c); err == nil {
				t.Fatalf("%q deveria ser recusado, voltou %q", c, rel)
			}
		})
	}
	// unicode e emoji no nome não são, por si, motivo de recusa (nenhuma allowlist de
	// caracteres, só de extensão): o caminho é aceito e fica dentro da caixinha.
	t.Run("aceita unicode e emoji no nome", func(t *testing.T) {
		if _, err := caminhoSeguro("knowledge/café 🏠.md"); err != nil {
			t.Fatalf("unicode/emoji não deveriam ser recusados: %v", err)
		}
	})
}

// zipMalicioso monta um zip com as entradas dadas (nome literal, sem passar por caminhoSeguro).
func zipMalicioso(t *testing.T, entradas map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for nome, conteudo := range entradas {
		fw, err := zw.Create(nome)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(conteudo)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestImportarZipNaoEscreveForaDoAgente: um zip com "../../fora.md" ou caminho absoluto não
// pode gravar fora da pasta do agente que está sendo importado (zip-slip).
func TestImportarZipNaoEscreveForaDoAgente(t *testing.T) {
	c, _ := montaFamilia(t)
	malicioso := zipMalicioso(t, map[string]string{
		AgentYAML:                      "nome: vitima\n",
		"../../../fora-do-agente.md":   "conteudo vazado",
		"knowledge/../../escapou.yaml": "y: 1\n",
	})
	arqs, err := c.ImportarZip("agentes/vitima-zip", malicioso)
	if err != nil {
		t.Fatalf("importação deveria aceitar o agent.yaml válido: %v", err)
	}
	for _, a := range arqs {
		if strings.Contains(a, "..") {
			t.Fatalf("entrada com .. sobreviveu à importação: %s", a)
		}
	}
	if _, err := os.Stat(filepath.Join(c.Root, "fora-do-agente.md")); err == nil {
		t.Fatal("o zip escreveu fora da raiz do catálogo")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(c.Root), "fora-do-agente.md")); err == nil {
		t.Fatal("o zip escreveu um nível acima da raiz do catálogo")
	}
}
