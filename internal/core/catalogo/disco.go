package catalogo

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"go-concierge/internal/falha"
)

// LerDisco lê agentes/ e nexus/ de raiz, nessa ordem e em ordem alfabética dentro de cada
// uma. Pula pastas e arquivos ocultos e o que não é arquivo comum. Pasta que não existe
// não é erro: o catálogo pode não ter nexus/.
func LerDisco(raiz string) ([]Arquivo, error) {
	out := []Arquivo{}
	for _, pasta := range pastasDoCatalogo {
		lidos, err := lerPasta(raiz, pasta)
		if err != nil {
			return nil, falha.ComPilha(fmt.Errorf("ler catálogo do disco: %w", err))
		}
		out = append(out, lidos...)
	}
	return out, nil
}

func lerPasta(raiz, pasta string) ([]Arquivo, error) {
	base := filepath.Join(raiz, pasta)
	if _, err := os.Stat(base); errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	var out []Arquivo
	err := filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p != base && strings.HasPrefix(d.Name(), ".") {
			return pularOculto(d)
		}
		if !d.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(raiz, p)
		if err != nil {
			return err
		}
		conteudo, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		out = append(out, Arquivo{Caminho: filepath.ToSlash(rel), Conteudo: conteudo})
		return nil
	})
	return out, err
}

func pularOculto(d fs.DirEntry) error {
	if d.IsDir() {
		return filepath.SkipDir
	}
	return nil
}

// Materializar refaz agentes/ e nexus/ de raiz só com arquivos: apaga as duas pastas e
// escreve cada arquivo. Todos os caminhos são validados antes do primeiro apagar, então um
// caminho vindo do banco que escape da raiz recusa a operação inteira sem tocar no disco.
func Materializar(raiz string, arquivos []Arquivo) error {
	for _, a := range arquivos {
		if err := ValidarCaminho(a.Caminho); err != nil {
			return fmt.Errorf("materializar catálogo: %w", err)
		}
	}
	for _, pasta := range pastasDoCatalogo {
		if err := os.RemoveAll(filepath.Join(raiz, pasta)); err != nil {
			return falha.ComPilha(fmt.Errorf("materializar catálogo: limpar %s: %w", pasta, err))
		}
	}
	for _, a := range arquivos {
		if err := escrever(raiz, a); err != nil {
			return falha.ComPilha(fmt.Errorf("materializar catálogo: %w", err))
		}
	}
	return nil
}

func escrever(raiz string, a Arquivo) error {
	destino := filepath.Join(raiz, filepath.FromSlash(a.Caminho))
	if err := os.MkdirAll(filepath.Dir(destino), 0o755); err != nil {
		return err
	}
	return os.WriteFile(destino, a.Conteudo, 0o644)
}
