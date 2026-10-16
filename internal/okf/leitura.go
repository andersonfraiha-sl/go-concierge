package okf

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Leitura de recursos pelo harness: o índice (Dados, Arquivos, Herdados) diz o que
// existe; aqui se resolve ONDE o arquivo mora (própria pasta, ancestral dono ou
// fonte declarada) e se confere a fronteira antes de ler.

// Ler devolve o conteúdo de um Arquivo próprio, herdado de um ancestral ou vindo de
// uma `fontes:` do agent.yaml (caminho `fonte:<pasta>/<rel>`).
func (c *Catalogo) Ler(a *Agent, f Arquivo) ([]byte, error) {
	base := a.Path
	switch f.HerdadoDe {
	case "":
	case "fonte":
		rotulo, rel, ok := strings.Cut(f.Caminho, "/")
		if !ok || !strings.HasPrefix(rotulo, "fonte:") {
			return nil, fmt.Errorf("caminho de fonte inválido: %s", f.Caminho)
		}
		nome := strings.TrimPrefix(rotulo, "fonte:")
		for _, fonte := range a.Fontes {
			dir := filepath.Clean(filepath.Join(a.Path, filepath.FromSlash(fonte)))
			if filepath.Base(dir) != nome || !dirExists(dir) || !Permitido(a.Path, dir) {
				continue
			}
			p := filepath.Join(dir, filepath.FromSlash(rel))
			if !dentro(dir, p) {
				return nil, fmt.Errorf("caminho fora da fonte: %s", f.Caminho)
			}
			return os.ReadFile(p)
		}
		return nil, fmt.Errorf("fonte desconhecida: %s", f.Caminho)
	default:
		achou := false
		for _, anc := range Ancestrais(a.Path) {
			if filepath.Base(anc) == f.HerdadoDe {
				base, achou = anc, true
				break
			}
		}
		if !achou {
			return nil, fmt.Errorf("ancestral %q não encontrado para %s", f.HerdadoDe, f.Caminho)
		}
	}
	p := filepath.Join(base, filepath.FromSlash(f.Caminho))
	if !dentro(base, p) || !Permitido(a.Path, p) {
		return nil, fmt.Errorf("fora da fronteira do agente: %s", f.Caminho)
	}
	return os.ReadFile(p)
}

// LerDado localiza um item do índice de dados (o mesmo que o prompt vê) pelo
// caminho e o lê.
func (c *Catalogo) LerDado(a *Agent, caminho string) ([]byte, error) {
	for _, d := range c.Dados(a) {
		if d.Caminho == caminho {
			return c.Ler(a, d)
		}
	}
	return nil, fmt.Errorf("dado não está no índice do agente: %s", caminho)
}
