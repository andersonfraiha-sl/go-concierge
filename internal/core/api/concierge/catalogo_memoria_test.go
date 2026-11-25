package conciergeapi

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"

	"go-concierge/internal/core/banco/bancotest"
	"go-concierge/internal/core/catalogo"
	okf "go-concierge/internal/core/catalogo/arquivos"
	"go-concierge/internal/harness/persistencia"
)

func TestCatalogoEmMemoriaPreservaEscritasIndependentes(t *testing.T) {
	db := bancotest.Novo(t)
	pg := catalogo.NovoPostgres(db.Ent)
	semente, err := catalogo.LerDisco(filepath.Join("..", "..", "..", "..", "okf"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pg.Substituir(t.Context(), semente); err != nil {
		t.Fatal(err)
	}
	repo := &repositorioCatalogo{banco: pg}
	primeiro, err := repo.retrato(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	segundo, err := repo.retrato(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		errs <- primeiro.SalvarArquivo(relConcierge, "knowledge/concorrente-a.md", []byte("A"))
	}()
	go func() {
		defer wg.Done()
		errs <- segundo.SalvarArquivo(relConcierge, "knowledge/concorrente-b.md", []byte("B"))
	}()
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	reaberto, err := repo.retrato(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	for nome, esperado := range map[string]string{"knowledge/concorrente-a.md": "A", "knowledge/concorrente-b.md": "B"} {
		lido, err := reaberto.LerArquivo(relConcierge, nome)
		if err != nil || string(lido) != esperado {
			t.Fatalf("%s: %q, %v", nome, lido, err)
		}
	}
}

type bancoCatalogoIncerto struct {
	arquivos []catalogo.Arquivo
	chamadas int
}

func (b *bancoCatalogoIncerto) Listar(context.Context) ([]catalogo.Arquivo, error) {
	return b.arquivos, nil
}
func (b *bancoCatalogoIncerto) Substituir(_ context.Context, gravar []catalogo.Arquivo, remover ...string) (catalogo.Mudanca, error) {
	b.chamadas++
	porCaminho := make(map[string]catalogo.Arquivo, len(b.arquivos))
	for _, a := range b.arquivos {
		porCaminho[a.Caminho] = a
	}
	for _, a := range gravar {
		porCaminho[a.Caminho] = a
	}
	for _, caminho := range remover {
		delete(porCaminho, caminho)
	}
	b.arquivos = b.arquivos[:0]
	for _, a := range porCaminho {
		b.arquivos = append(b.arquivos, a)
	}
	return catalogo.Mudanca{}, persistencia.ErrGravacaoIncerta
}

func TestCatalogoEmMemoriaNaoRepeteCommitIncerto(t *testing.T) {
	b := &bancoCatalogoIncerto{arquivos: []catalogo.Arquivo{{Caminho: "agentes/a/agent.yaml", Conteudo: []byte("nome: a\n")}}}
	repo := &repositorioCatalogo{banco: b}
	for range 2 {
		cat, err := repo.retrato(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if err := cat.SalvarArquivo("agentes/a", "persona.md", []byte("x")); !errors.Is(err, persistencia.ErrGravacaoIncerta) {
			t.Fatalf("erro: %v", err)
		}
	}
	if b.chamadas != 1 {
		t.Fatalf("commits: %d", b.chamadas)
	}
}

func TestLoteNaoRepersisteDepoisDeCommitIncerto(t *testing.T) {
	b := &bancoCatalogoIncerto{arquivos: []catalogo.Arquivo{{Caminho: "agentes/a/agent.yaml", Conteudo: []byte("nome: a\n")}}}
	repo := &repositorioCatalogo{banco: b}
	cat, err := repo.retrato(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	_, err = cat.AplicarArquivos([]okf.ArquivoBruto{{Caminho: "agentes/a/persona.md", Conteudo: []byte("nova")}}, nil)
	if !errors.Is(err, persistencia.ErrGravacaoIncerta) || b.chamadas != 1 {
		t.Fatalf("erro=%v tentativas=%d", err, b.chamadas)
	}
	cat, err = repo.retrato(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	raw, err := cat.LerArquivo("agentes/a", "persona.md")
	if err != nil || string(raw) != "nova" {
		t.Fatalf("resultado já confirmado foi alterado: %q, %v", raw, err)
	}
}

func TestFonteNexusReconheceInstantaneoMesmoSeInvalido(t *testing.T) {
	cat, err := okf.NovoEmMemoria([]catalogo.Arquivo{{Caminho: "nexus/conta.yaml", Conteudo: []byte("recursos: [")}})
	if err != nil {
		t.Fatal(err)
	}
	fonte := fonteNexusDoRetrato(cat, "conta", nil)
	if fonte == nil {
		t.Fatal("instantâneo existente não foi reconhecido")
	}
	if recursos := fonte(); recursos != nil {
		t.Fatalf("instantâneo inválido retornou recursos: %+v", recursos)
	}
}

func TestFonteNexusPreservaUltimaRespostaQuandoAoVivoFalha(t *testing.T) {
	cat, err := okf.NovoEmMemoria(nil)
	if err != nil {
		t.Fatal(err)
	}
	chamadas := 0
	fonte := fonteNexusDoRetrato(cat, "conta", func(context.Context) ([]okf.Recurso, error) {
		chamadas++
		if chamadas == 1 {
			return []okf.Recurso{{Name: "anterior"}}, nil
		}
		return nil, errors.New("indisponível")
	})
	if got := fonte(); len(got) != 1 || got[0].Name != "anterior" {
		t.Fatalf("primeira resposta: %+v", got)
	}
	if got := fonte(); len(got) != 1 || got[0].Name != "anterior" {
		t.Fatalf("cache após falha: %+v", got)
	}
}
