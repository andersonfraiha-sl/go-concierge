package conciergeapi

import (
	"errors"
	"testing"

	"go-concierge/internal/core/catalogo"
	"go-concierge/internal/harness/persistencia"
)

func TestPersistirCatalogoNaoRepeteAposResultadoIncerto(t *testing.T) {
	b := &bancoCatalogoIncerto{arquivos: []catalogo.Arquivo{{Caminho: "agentes/a/agent.yaml", Conteudo: []byte("nome: a\n")}}}
	repo := &repositorioCatalogo{banco: b}
	for range 2 {
		cat, err := repo.retrato(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if err := cat.SalvarArquivo("agentes/a", "persona.md", []byte("novo")); !errors.Is(err, persistencia.ErrGravacaoIncerta) {
			t.Fatalf("incerteza perdida: %v", err)
		}
	}
	if b.chamadas != 1 {
		t.Fatalf("repetiu escrita incerta: %d", b.chamadas)
	}
}
