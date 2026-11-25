package catalogo

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"testing/fstest"

	"go-concierge/internal/harness/conteudo"
)

type Catalogo struct {
	Root     string
	arquivos fstest.MapFS
	Recursos func() []Recurso
}

func Novo(arquivos map[string][]byte, recursos []conteudo.Recurso, conferir bool) (*Catalogo, error) {
	c := &Catalogo{Root: "/catalogo", arquivos: make(fstest.MapFS, len(arquivos))}
	for nome, dados := range arquivos {
		if !fs.ValidPath(nome) || strings.ContainsAny(nome, "\\:\x00") || nome == "." || (!strings.HasPrefix(nome, "agentes/") && !strings.HasPrefix(nome, "nexus/")) {
			return nil, fmt.Errorf("caminho inválido no catálogo: %q", nome)
		}
		c.arquivos[nome] = &fstest.MapFile{Data: append([]byte(nil), dados...)}
	}
	if conferir {
		copia := append([]conteudo.Recurso(nil), recursos...)
		c.Recursos = func() []Recurso { return copia }
	}
	return c, nil
}

func (c *Catalogo) relativo(p string) (string, error) {
	rel, err := filepath.Rel(c.Root, p)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("caminho fora do catálogo")
	}
	return filepath.ToSlash(rel), nil
}

func (c *Catalogo) readFile(p string) ([]byte, error) {
	rel, err := c.relativo(p)
	if err != nil {
		return nil, err
	}
	return fs.ReadFile(c.arquivos, rel)
}

func (c *Catalogo) stat(p string) (fs.FileInfo, error) {
	rel, err := c.relativo(p)
	if err != nil {
		return nil, err
	}
	return fs.Stat(c.arquivos, rel)
}

func (c *Catalogo) readDir(p string) ([]fs.DirEntry, error) {
	rel, err := c.relativo(p)
	if err != nil {
		return nil, err
	}
	return fs.ReadDir(c.arquivos, rel)
}

func (c *Catalogo) walkDir(p string, fn fs.WalkDirFunc) error {
	rel, err := c.relativo(p)
	if err != nil {
		return err
	}
	return fs.WalkDir(c.arquivos, rel, func(nome string, d fs.DirEntry, err error) error {
		return fn(filepath.Join(c.Root, filepath.FromSlash(nome)), d, err)
	})
}

func (c *Catalogo) loadCaso(p string) (*Caso, error) {
	raw, err := c.readFile(p)
	if err != nil {
		return nil, err
	}
	return ParseCaso(raw)
}

func (c *Catalogo) LerArquivo(agente, rel string) ([]byte, error) {
	dir, err := c.ResolveAgente(agente)
	if err != nil {
		return nil, err
	}
	if strings.ContainsRune(rel, 0) {
		return nil, errors.New("caminho inválido")
	}
	rel = filepath.ToSlash(filepath.Clean("/" + rel))[1:]
	if rel == "" || rel == "." {
		return nil, errors.New("caminho vazio")
	}
	if rel != AgentYAML && rel != PersonaMD && rel != CasoYAML {
		primeiro, _, tem := strings.Cut(rel, "/")
		if _, ok := CaixinhaPorNome(primeiro); !ok || !tem {
			return nil, errors.New("caminho inválido")
		}
		for _, p := range strings.Split(rel, "/") {
			if p == "" || strings.HasPrefix(p, ".") {
				return nil, errors.New("caminho inválido")
			}
		}
		switch strings.ToLower(filepath.Ext(rel)) {
		case ".md", ".yaml", ".yml", ".json", ".txt", ".csv", ".png", ".jpg", ".jpeg", ".svg", ".gif", ".webp", ".pdf":
		default:
			return nil, errors.New("extensão não permitida")
		}
	}
	return c.readFile(filepath.Join(dir, filepath.FromSlash(rel)))
}

type Retrato struct {
	Agente        string              `json:"agente"`
	Conta         string              `json:"conta"`
	Arquivos      map[string][]byte   `json:"arquivos"`
	RecursosNexus *[]conteudo.Recurso `json:"recursos_nexus,omitempty"`
}
