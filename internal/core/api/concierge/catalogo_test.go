package conciergeapi

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go-concierge/internal/core/banco/bancotest"
	"go-concierge/internal/core/catalogo"
	okf "go-concierge/internal/core/catalogo/arquivos"
)

const (
	relConcierge = "agentes/concierge"
	relMudanca   = "agentes/concierge/agentes/agendar-mudanca"
)

func gravarArquivo(t *testing.T, raiz, rel, conteudo string) {
	t.Helper()
	p := filepath.Join(raiz, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(conteudo), 0644); err != nil {
		t.Fatal(err)
	}
}

func caminhosDoBanco(t *testing.T, pg *catalogo.Postgres) []string {
	t.Helper()
	arquivos, err := pg.Listar(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	out := make([]string, len(arquivos))
	for i, a := range arquivos {
		out[i] = a.Caminho
	}
	return out
}

func TestImportarCatalogoPreservaArquivosAusentesDaPasta(t *testing.T) {
	pg := catalogo.NovoPostgres(bancotest.Novo(t).Ent)
	pasta := t.TempDir()
	gravarArquivo(t, pasta, "agentes/a/agent.yaml", "nome: a\n")
	gravarArquivo(t, pasta, "agentes/b/agent.yaml", "nome: b\n")
	if _, err := substituirCatalogo(t.Context(), pg, pasta); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(pasta, "agentes", "b")); err != nil {
		t.Fatal(err)
	}
	gravarArquivo(t, pasta, "agentes/c/agent.yaml", "nome: c\n")

	m, err := substituirCatalogo(t.Context(), pg, pasta)

	if err != nil {
		t.Fatal(err)
	}
	if m != (catalogo.Mudanca{Gravados: 1}) {
		t.Fatalf("mudança = %+v", m)
	}
	if got := caminhosDoBanco(t, pg); !slices.Equal(got, []string{"agentes/a/agent.yaml", "agentes/b/agent.yaml", "agentes/c/agent.yaml"}) {
		t.Fatalf("banco = %v", got)
	}
}

func TestImportarCatalogoRecusaPastaSemArquivosParaNaoEsvaziarOBanco(t *testing.T) {
	pg := catalogo.NovoPostgres(bancotest.Novo(t).Ent)
	if _, err := pg.Substituir(t.Context(), []catalogo.Arquivo{{Caminho: "agentes/a/agent.yaml", Conteudo: []byte("nome: a\n")}}); err != nil {
		t.Fatal(err)
	}

	_, err := substituirCatalogo(t.Context(), pg, t.TempDir())

	if err == nil {
		t.Fatal("pasta vazia deveria ser recusada")
	}
	if got := caminhosDoBanco(t, pg); len(got) != 1 {
		t.Fatalf("o banco foi esvaziado: %v", got)
	}
}

func TestWriteErrTrataFalhaDePersistirComoErroDoServidorSemExporOBanco(t *testing.T) {
	rec := httptest.NewRecorder()

	writeErr(rec, http.StatusBadRequest, fmt.Errorf("%w: %w", okf.ErrPersistir, errors.New("conexão com 10.0.0.5 recusada")))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, quer 500", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "10.0.0.5") {
		t.Fatalf("a resposta expôs o texto do banco: %s", rec.Body)
	}
}

func TestSementeDivergenteNaoSobrescreveBanco(t *testing.T) {
	b := bancotest.Novo(t)
	pg := catalogo.NovoPostgres(b.Ent)
	arquivo := catalogo.Arquivo{Caminho: "agentes/banco/agent.yaml", Conteudo: []byte("nome: banco\n")}
	if _, err := pg.Substituir(t.Context(), []catalogo.Arquivo{arquivo}); err != nil {
		t.Fatal(err)
	}
	semente := t.TempDir()
	gravarArquivo(t, semente, "agentes/semente/agent.yaml", "nome: semente\n")
	var logs bytes.Buffer
	anterior := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(anterior) })
	cat, err := abrirCatalogo(semente, b)
	if err != nil {
		t.Fatal(err)
	}
	arvore, err := cat.Tree()
	if err != nil || len(arvore) != 1 || arvore[0].Nome != "banco" {
		t.Fatalf("árvore=%+v erro=%v", arvore, err)
	}
	if !strings.Contains(logs.String(), "importar-catalogo") {
		t.Fatalf("aviso ausente: %s", logs.String())
	}
}

func TestBancoPopuladoNaoDependeDeSementeInacessivel(t *testing.T) {
	b := bancotest.Novo(t)
	pg := catalogo.NovoPostgres(b.Ent)
	if _, err := pg.Substituir(t.Context(), []catalogo.Arquivo{{Caminho: "agentes/banco/agent.yaml", Conteudo: []byte("nome: banco\n")}}); err != nil {
		t.Fatal(err)
	}
	semente := filepath.Join(t.TempDir(), "nao-e-pasta")
	if err := os.WriteFile(semente, []byte("arquivo"), 0600); err != nil {
		t.Fatal(err)
	}
	cat, err := abrirCatalogo(semente, b)
	if err != nil {
		t.Fatal(err)
	}
	if arvore, err := cat.Tree(); err != nil || len(arvore) != 1 || arvore[0].Nome != "banco" {
		t.Fatalf("árvore=%+v erro=%v", arvore, err)
	}
	if err := migrarCatalogoNexusLegado(semente, "conta", pg); err != nil {
		t.Fatal(err)
	}
}

func TestCopiaEditavelNexusNaoGeraAvisoDeSementeDivergente(t *testing.T) {
	b := bancotest.Novo(t)
	pg := catalogo.NovoPostgres(b.Ent)
	semente := t.TempDir()
	gravarArquivo(t, semente, "agentes/a/agent.yaml", "nome: a\n")
	if _, err := abrirCatalogo(semente, b); err != nil {
		t.Fatal(err)
	}
	if err := pg.InserirCatalogoNexusSeAusente(t.Context(), catalogo.Arquivo{Caminho: "nexus/catalogo-nexus/conta.yaml", Conteudo: []byte("recursos: []\n")}); err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	anterior := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(anterior) })
	if _, err := abrirCatalogo(semente, b); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(logs.String(), "WARN") {
		t.Fatalf("aviso indevido: %s", logs.String())
	}
}
