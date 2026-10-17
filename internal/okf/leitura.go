package okf

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Leitura de arquivos pelo harness: o índice (Indice, Arquivos, Herdados) diz o que
// existe; aqui se resolve ONDE o arquivo mora (própria pasta ou ancestral dono) e
// se confere a fronteira antes de ler.

// Ler devolve o conteúdo de um Arquivo próprio ou herdado de um ancestral.
func (c *Catalogo) Ler(a *Agent, f Arquivo) ([]byte, error) {
	base := a.Path
	if f.HerdadoDe != "" {
		base = ancestralPorNome(a.Path, f.HerdadoDe)
		if base == "" {
			return nil, fmt.Errorf("ancestral %q não encontrado para %s", f.HerdadoDe, f.Caminho)
		}
	}
	p := filepath.Join(base, filepath.FromSlash(f.Caminho))
	if !dentro(base, p) || !Permitido(a.Path, p) {
		return nil, fmt.Errorf("fora da fronteira do agente: %s", f.Caminho)
	}
	return os.ReadFile(p)
}

// LerDado localiza um dado do agente pelo caminho e o lê. Aceita tudo do índice
// completo (também o que a progressive disclosure esconde do prompt). Num .md, os
// links relativos voltam reescritos para o caminho canônico (ReescreverLinks).
func (c *Catalogo) LerDado(a *Agent, caminho string) ([]byte, error) {
	caminho = strings.TrimPrefix(path.Clean(caminho), "./")
	for _, d := range c.IndiceCompleto(a) {
		if d.Caminho == caminho {
			raw, err := c.Ler(a, d)
			if err == nil && strings.HasSuffix(strings.ToLower(caminho), ".md") {
				raw = []byte(ReescreverLinks(caminho, string(raw)))
			}
			return raw, err
		}
	}
	return nil, fmt.Errorf("não está no índice do agente: %s", caminho)
}
