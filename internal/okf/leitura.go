package okf

import (
	"fmt"
	"os"
	"path/filepath"
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

// LerDado localiza um item do índice (o mesmo que o prompt vê) pelo caminho e o lê.
func (c *Catalogo) LerDado(a *Agent, caminho string) ([]byte, error) {
	for _, d := range c.Indice(a) {
		if d.Caminho == caminho {
			return c.Ler(a, d)
		}
	}
	return nil, fmt.Errorf("não está no índice do agente: %s", caminho)
}
