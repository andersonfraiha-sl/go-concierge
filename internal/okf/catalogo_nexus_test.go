package okf

import (
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"
)

func TestCatalogoNexusPersistenciaIsolamentoEdicao(t *testing.T) {
	root := t.TempDir()
	store, err := NewCatalogoNexusStore(root, "account-a")
	if err != nil {
		t.Fatal(err)
	}
	outro, err := NewCatalogoNexusStore(root, "account-b")
	if err != nil {
		t.Fatal(err)
	}
	recurso := Recurso{Name: "condominios.listar", Titulo: "Listar condomínios", Kind: KindAPI, Visibility: "private", Description: "Lista", Params: []ParamMeta{}, Definicoes: []Definicao{{Name: "tenant", Label: "Tenant"}}}
	if err := store.Salvar("", recurso); err != nil {
		t.Fatal(err)
	}
	outroLista, err := outro.Listar()
	if err != nil {
		t.Fatal(err)
	} else if len(outroLista) != 0 {
		t.Fatal("as accounts compartilharam recursos")
	}
	editado := recurso
	editado.Description = "Descrição alterada"
	editado.Definicoes = nil
	if err := store.Salvar(recurso.Name, editado); err != nil {
		t.Fatal(err)
	}
	lista, err := store.Listar()
	if err != nil || len(lista) != 1 || lista[0].Description != "Descrição alterada" || len(lista[0].Definicoes) != 1 {
		t.Fatalf("edição não preservou definições: %#v, %v", lista, err)
	}
	if err := store.Remover(recurso.Name); err != nil {
		t.Fatal(err)
	}
	if lista, err = store.Listar(); err != nil || len(lista) != 0 {
		t.Fatalf("remoção falhou: %#v, %v", lista, err)
	}
	if _, err := os.Stat(store.Path()); err != nil {
		t.Fatal(err)
	}
}

func TestCatalogoNexusBloqueiaDuplicataConcorrente(t *testing.T) {
	root := t.TempDir()
	store, err := NewCatalogoNexusStore(root, "account")
	if err != nil {
		t.Fatal(err)
	}
	otherStore, err := NewCatalogoNexusStore(root, "account")
	if err != nil {
		t.Fatal(err)
	}
	const workers = 24
	var wg sync.WaitGroup
	var saved atomic.Int32
	var duplicated atomic.Int32
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			target := store
			if i%2 == 1 {
				target = otherStore
			}
			err := target.Salvar("", Recurso{Name: "servico.listar", Kind: KindAPI, Visibility: "private", Params: []ParamMeta{}})
			switch {
			case err == nil:
				saved.Add(1)
			case errors.Is(err, ErrRecursoCatalogoDuplicado):
				duplicated.Add(1)
			default:
				t.Errorf("erro ao salvar concorrentemente: %v", err)
			}
		}(i)
	}
	wg.Wait()
	if saved.Load() != 1 || duplicated.Load() != workers-1 {
		t.Fatalf("sucesso=%d duplicadas=%d", saved.Load(), duplicated.Load())
	}
}

func TestCatalogoNexusValidaAccountERetorna409SemAlterarArquivo(t *testing.T) {
	if _, err := NewCatalogoNexusStore(t.TempDir(), "../fora"); err == nil {
		t.Fatal("account inválida aceita")
	}
	if _, err := NewCatalogoNexusStore(t.TempDir(), "empresa.001"); err != nil {
		t.Fatalf("account com ponto deveria ser aceita: %v", err)
	}
	store, err := NewCatalogoNexusStore(t.TempDir(), "account")
	if err != nil {
		t.Fatal(err)
	}
	recurso := Recurso{Name: "servico.listar", Kind: KindAPI, Visibility: "private", Params: []ParamMeta{}}
	if err := store.Salvar("", recurso); err != nil {
		t.Fatal(err)
	}
	if err := store.Salvar("", recurso); !errors.Is(err, ErrRecursoCatalogoDuplicado) {
		t.Fatalf("erro de duplicata inesperado: %v", err)
	}
}
