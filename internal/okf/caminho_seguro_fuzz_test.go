package okf

import (
	"path/filepath"
	"strings"
	"testing"
)

// FuzzCaminhoSeguro: caminhoSeguro é a guarda contra path traversal ao salvar/ler/excluir
// arquivo de agente. A invariante é simples e vale para qualquer entrada: se ela aceita um
// caminho, esse caminho junto à raiz do agente nunca escapa da raiz nem aponta para agentes/.
func FuzzCaminhoSeguro(f *testing.F) {
	for _, s := range []string{
		"agent.yaml", "persona.md", "caso.yaml", "rules/x.md", "../../../etc/passwd",
		"agentes/x/agent.yaml", "rules/../../../x.md", "rules/.hidden.md", "rules/x.exe",
		"/rules/x.md", "rules//x.md", "rules/x/../../y.md", "..", ".", "",
		"rules/x\x00.md", "rules/🙂.md", "RULES/X.MD",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, rel string) {
		aceito, err := caminhoSeguro(rel)
		if err != nil {
			return
		}
		raiz := t.TempDir()
		alvo := filepath.Join(raiz, filepath.FromSlash(aceito))
		absRaiz, _ := filepath.Abs(raiz)
		absAlvo, _ := filepath.Abs(alvo)
		if !strings.HasPrefix(absAlvo, absRaiz+string(filepath.Separator)) {
			t.Fatalf("caminhoSeguro(%q) = %q escapa da raiz: %q", rel, aceito, absAlvo)
		}
		primeiro, _, _ := strings.Cut(aceito, "/")
		if primeiro == DirAgentes {
			t.Fatalf("caminhoSeguro(%q) = %q deveria ter recusado agentes/", rel, aceito)
		}
		for _, p := range strings.Split(aceito, "/") {
			if p == ".." {
				t.Fatalf("caminhoSeguro(%q) = %q contém ..", rel, aceito)
			}
		}
	})
}
