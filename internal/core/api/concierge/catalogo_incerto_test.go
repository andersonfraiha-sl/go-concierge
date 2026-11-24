package conciergeapi

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"go-concierge/internal/core/catalogo"
	"go-concierge/internal/harness/persistencia"
)

type catalogoIncertoTeste struct{ chamadas int }

func (c *catalogoIncertoTeste) Substituir(context.Context, []catalogo.Arquivo, ...string) (catalogo.Mudanca, error) {
	c.chamadas++
	return catalogo.Mudanca{}, persistencia.ErrGravacaoIncerta
}

func TestPersistirCatalogoNaoRepeteAposResultadoIncerto(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "agentes"), 0700); err != nil {
		t.Fatal(err)
	}
	store := &catalogoIncertoTeste{}
	persistir := persistirNoBanco(store, dir)
	if err := os.WriteFile(filepath.Join(dir, "agentes", "novo.md"), []byte("novo"), 0600); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := persistir(); !errors.Is(err, persistencia.ErrGravacaoIncerta) {
			t.Fatalf("incerteza perdida: %v", err)
		}
	}
	if store.chamadas != 1 {
		t.Fatalf("repetiu escrita incerta: %d", store.chamadas)
	}
}
