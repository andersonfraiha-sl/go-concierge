package okf

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

var (
	ErrRecursoCatalogoDuplicado = errors.New("já existe um recurso com esse nome")
	ErrRecursoCatalogoAusente   = errors.New("recurso não encontrado")
)

var catalogoNexusLocks sync.Map

type CatalogoNexusStore struct {
	path string
	mu   *sync.Mutex
}

func NewCatalogoNexusStore(root, account string) (*CatalogoNexusStore, error) {
	account = strings.TrimSpace(account)
	if account == "" || account == "." || account == ".." || strings.ContainsAny(account, "/\\\x00") {
		return nil, fmt.Errorf("account inválida para o catálogo Nexus")
	}
	path, err := filepath.Abs(filepath.Join(root, "catalogo-nexus", account+".yaml"))
	if err != nil {
		return nil, err
	}
	lock, _ := catalogoNexusLocks.LoadOrStore(path, &sync.Mutex{})
	return &CatalogoNexusStore{path: path, mu: lock.(*sync.Mutex)}, nil
}

func (s *CatalogoNexusStore) Path() string { return s.path }

func (s *CatalogoNexusStore) Listar() ([]Recurso, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ler()
}

func (s *CatalogoNexusStore) Salvar(original string, recurso Recurso) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := recurso.Validar(); err != nil {
		return err
	}
	lista, err := s.ler()
	if err != nil {
		return err
	}
	if original == "" {
		for _, existente := range lista {
			if existente.Name == recurso.Name {
				return ErrRecursoCatalogoDuplicado
			}
		}
		lista = append(lista, recurso)
		return s.gravar(lista)
	}
	indice := -1
	for i := range lista {
		if lista[i].Name == recurso.Name && lista[i].Name != original {
			return ErrRecursoCatalogoDuplicado
		}
		if lista[i].Name == original {
			indice = i
		}
	}
	if indice < 0 {
		return ErrRecursoCatalogoAusente
	}
	if recurso.Definicoes == nil {
		recurso.Definicoes = lista[indice].Definicoes
	}
	lista[indice] = recurso
	return s.gravar(lista)
}

func (s *CatalogoNexusStore) Remover(nome string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	lista, err := s.ler()
	if err != nil {
		return err
	}
	resto := make([]Recurso, 0, len(lista))
	removido := false
	for _, recurso := range lista {
		if recurso.Name == nome {
			removido = true
			continue
		}
		resto = append(resto, recurso)
	}
	if !removido {
		return ErrRecursoCatalogoAusente
	}
	return s.gravar(resto)
}

func (s *CatalogoNexusStore) ler() ([]Recurso, error) {
	raw, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return []Recurso{}, nil
	}
	if err != nil {
		return nil, err
	}
	var dados struct {
		Recursos []Recurso `yaml:"recursos"`
	}
	if err := yaml.Unmarshal(raw, &dados); err != nil {
		return nil, fmt.Errorf("ler catálogo Nexus: %w", err)
	}
	if dados.Recursos == nil {
		dados.Recursos = []Recurso{}
	}
	return dados.Recursos, nil
}

func (s *CatalogoNexusStore) gravar(lista []Recurso) error {
	corpo, err := yaml.Marshal(struct {
		Recursos []Recurso `yaml:"recursos"`
	}{Recursos: lista})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".catalogo-nexus-*.yaml")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(corpo); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, s.path)
}
