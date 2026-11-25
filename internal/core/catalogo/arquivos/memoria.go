package arquivos

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing/fstest"

	"go-concierge/internal/core/catalogo"
)

func NovoEmMemoria(arquivos []catalogo.Arquivo) (*Catalogo, error) {
	c := &Catalogo{Root: "/catalogo"}
	if err := c.AtualizarMemoria(arquivos); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Catalogo) AtualizarMemoria(arquivos []catalogo.Arquivo) error {
	novos := make(fstest.MapFS, len(arquivos)+1)
	novos["agentes"] = &fstest.MapFile{Mode: fs.ModeDir | 0755}
	for _, arquivo := range arquivos {
		if err := catalogo.ValidarCaminho(arquivo.Caminho); err != nil {
			return err
		}
		novos[arquivo.Caminho] = &fstest.MapFile{Data: append([]byte(nil), arquivo.Conteudo...)}
	}
	c.arquivos = novos
	return nil
}

func (c *Catalogo) ArquivosEmMemoria() []catalogo.Arquivo {
	var arquivos []catalogo.Arquivo
	for nome, arq := range c.arquivos {
		if arq.Mode.IsDir() || slices.ContainsFunc(strings.Split(nome, "/"), func(p string) bool { return strings.HasPrefix(p, ".") }) {
			continue
		}
		arquivos = append(arquivos, catalogo.Arquivo{Caminho: nome, Conteudo: append([]byte(nil), arq.Data...)})
	}
	slices.SortFunc(arquivos, func(a, b catalogo.Arquivo) int { return strings.Compare(a.Caminho, b.Caminho) })
	return arquivos
}

func (c *Catalogo) relativo(p string) (string, error) {
	rel, err := filepath.Rel(c.Root, p)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("caminho fora do catálogo")
	}
	return filepath.ToSlash(rel), nil
}

func (c *Catalogo) readFile(p string) ([]byte, error) {
	if c.arquivos == nil {
		return os.ReadFile(p)
	}
	rel, err := c.relativo(p)
	if err != nil {
		return nil, err
	}
	return fs.ReadFile(c.arquivos, rel)
}

func (c *Catalogo) stat(p string) (fs.FileInfo, error) {
	if c.arquivos == nil {
		return os.Stat(p)
	}
	rel, err := c.relativo(p)
	if err != nil {
		return nil, err
	}
	return fs.Stat(c.arquivos, rel)
}

func (c *Catalogo) readDir(p string) ([]fs.DirEntry, error) {
	if c.arquivos == nil {
		return os.ReadDir(p)
	}
	rel, err := c.relativo(p)
	if err != nil {
		return nil, err
	}
	return fs.ReadDir(c.arquivos, rel)
}

func (c *Catalogo) walkDir(p string, fn fs.WalkDirFunc) error {
	if c.arquivos == nil {
		return filepath.WalkDir(p, fn)
	}
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

func (c *Catalogo) mkdirAll(p string, perm fs.FileMode) error {
	if c.arquivos == nil {
		return os.MkdirAll(p, perm)
	}
	rel, err := c.relativo(p)
	if err != nil || rel == "." {
		return err
	}
	partes := strings.Split(rel, "/")
	for i := range partes {
		atual := strings.Join(partes[:i+1], "/")
		info, err := fs.Stat(c.arquivos, atual)
		if err == nil {
			if !info.IsDir() {
				return &fs.PathError{Op: "mkdir", Path: p, Err: syscall.ENOTDIR}
			}
			continue
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		c.arquivos[atual] = &fstest.MapFile{Mode: fs.ModeDir | perm}
	}
	return nil
}

func (c *Catalogo) writeFile(p string, dados []byte, perm fs.FileMode) error {
	if c.arquivos == nil {
		return os.WriteFile(p, dados, perm)
	}
	rel, err := c.relativo(p)
	if err != nil {
		return err
	}
	pai, err := c.stat(filepath.Dir(p))
	if err != nil {
		return err
	}
	if !pai.IsDir() {
		return &fs.PathError{Op: "write", Path: p, Err: syscall.ENOTDIR}
	}
	if info, err := c.stat(p); err == nil && info.IsDir() {
		return &fs.PathError{Op: "write", Path: p, Err: syscall.EISDIR}
	}
	c.arquivos[rel] = &fstest.MapFile{Data: append([]byte(nil), dados...), Mode: perm}
	return nil
}

func (c *Catalogo) escreverArquivoAtomico(p string, dados []byte, perm fs.FileMode) error {
	if c.arquivos == nil {
		return escreverArquivoAtomico(p, dados, perm)
	}
	return c.writeFile(p, dados, perm)
}

func (c *Catalogo) lstat(p string) (fs.FileInfo, error) {
	if c.arquivos == nil {
		return os.Lstat(p)
	}
	return c.stat(p)
}

func (c *Catalogo) remove(p string) error {
	if c.arquivos == nil {
		return os.Remove(p)
	}
	rel, err := c.relativo(p)
	if err != nil {
		return err
	}
	info, err := c.stat(p)
	if err != nil {
		return err
	}
	if info.IsDir() {
		entradas, err := c.readDir(p)
		if err != nil {
			return err
		}
		if len(entradas) > 0 {
			return &fs.PathError{Op: "remove", Path: p, Err: syscall.ENOTEMPTY}
		}
	}
	delete(c.arquivos, rel)
	return nil
}

func (c *Catalogo) removeAll(p string) error {
	if c.arquivos == nil {
		return os.RemoveAll(p)
	}
	rel, err := c.relativo(p)
	if err != nil || rel == "." {
		return err
	}
	for nome := range c.arquivos {
		if nome == rel || strings.HasPrefix(nome, rel+"/") {
			delete(c.arquivos, nome)
		}
	}
	return nil
}

func (c *Catalogo) rename(origem, destino string) error {
	if c.arquivos == nil {
		return os.Rename(origem, destino)
	}
	de, err := c.relativo(origem)
	if err != nil {
		return err
	}
	para, err := c.relativo(destino)
	if err != nil {
		return err
	}
	if _, err := c.stat(origem); err != nil {
		return err
	}
	if _, err := c.stat(filepath.Dir(destino)); err != nil {
		return err
	}
	if err := c.removeAll(destino); err != nil {
		return err
	}
	novos := make(fstest.MapFS)
	for nome, arquivo := range c.arquivos {
		if nome == de || strings.HasPrefix(nome, de+"/") {
			novos[para+strings.TrimPrefix(nome, de)] = arquivo
			delete(c.arquivos, nome)
		}
	}
	for nome, arquivo := range novos {
		c.arquivos[nome] = arquivo
	}
	return nil
}

func (c *Catalogo) Arquivos() ([]catalogo.Arquivo, error) {
	if c.arquivos == nil {
		return catalogo.LerDisco(c.Root)
	}
	return c.ArquivosEmMemoria(), nil
}
