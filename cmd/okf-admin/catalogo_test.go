package main

import (
	"bytes"
	"context"
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

	"go-concierge/internal/banco"
	"go-concierge/internal/banco/bancotest"
	"go-concierge/internal/catalogo"
	"go-concierge/internal/okf"
)

const (
	relConcierge = "agentes/concierge"
	relMudanca   = "agentes/concierge/agentes/agendar-mudanca"
)

func gravarArquivo(t *testing.T, raiz, rel, conteudo string) {
	t.Helper()
	p := filepath.Join(raiz, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(conteudo), 0o644); err != nil {
		t.Fatal(err)
	}
}

// sementeDoRepositorio copia agentes/ e nexus/ do catálogo real para uma pasta temporária.
func sementeDoRepositorio(t *testing.T) string {
	t.Helper()
	semente := t.TempDir()
	for _, pasta := range []string{okf.DirAgentes, "nexus"} {
		if err := os.CopyFS(filepath.Join(semente, pasta), os.DirFS(filepath.Join("..", "..", "okf", pasta))); err != nil {
			t.Fatal(err)
		}
	}
	return semente
}

func caminhosDoBanco(t *testing.T, pg *catalogo.Postgres) []string {
	t.Helper()
	lidos, err := pg.Listar(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	return caminhosDe(lidos)
}

func caminhosDe(arquivos []catalogo.Arquivo) []string {
	out := make([]string, len(arquivos))
	for i, a := range arquivos {
		out[i] = a.Caminho
	}
	return out
}

// bancoIgualAoDisco confere caminho e bytes de todos os arquivos, nos dois lados.
func bancoIgualAoDisco(t *testing.T, pg *catalogo.Postgres, copia string) {
	t.Helper()
	noBanco, err := pg.Listar(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	noDisco, err := catalogo.LerDisco(copia)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.EqualFunc(noBanco, noDisco, func(a, b catalogo.Arquivo) bool {
		return a.Caminho == b.Caminho && bytes.Equal(a.Conteudo, b.Conteudo)
	}) {
		t.Fatalf("banco e disco divergem:\nbanco: %v\ndisco: %v", caminhosDe(noBanco), caminhosDe(noDisco))
	}
}

func abrirCatalogoDeTeste(t *testing.T, semente string, db banco.Banco) *okf.Catalogo {
	t.Helper()
	cat, err := abrirCatalogo(semente, db)
	if err != nil {
		t.Fatal(err)
	}
	return cat
}

func TestPrepararCopiaSemeiaOBancoVazioEMaterializa(t *testing.T) {
	db := bancotest.Novo(t)
	pg := catalogo.NovoPostgres(db.Ent)
	semente, copia := t.TempDir(), filepath.Join(t.TempDir(), "copia")
	gravarArquivo(t, semente, "agentes/a/agent.yaml", "nome: a\n")
	gravarArquivo(t, semente, "nexus/c.yaml", "recursos: []\n")

	if err := prepararCopia(t.Context(), pg, semente, copia); err != nil {
		t.Fatal(err)
	}

	quer := []string{"agentes/a/agent.yaml", "nexus/c.yaml"}
	if got := caminhosDoBanco(t, pg); !slices.Equal(got, quer) {
		t.Fatalf("banco = %v, quer %v", got, quer)
	}
	bancoIgualAoDisco(t, pg, copia)
}

func TestPrepararCopiaComBancoPreenchidoMantemOBancoEAvisa(t *testing.T) {
	var logs bytes.Buffer
	anterior := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(anterior) })
	db := bancotest.Novo(t)
	pg := catalogo.NovoPostgres(db.Ent)
	if _, err := pg.Substituir(t.Context(), []catalogo.Arquivo{{Caminho: "agentes/editada/agent.yaml", Conteudo: []byte("nome: editada\n")}}); err != nil {
		t.Fatal(err)
	}
	semente, copia := t.TempDir(), filepath.Join(t.TempDir(), "copia")
	gravarArquivo(t, semente, "agentes/da-pasta/agent.yaml", "nome: da-pasta\n")

	if err := prepararCopia(t.Context(), pg, semente, copia); err != nil {
		t.Fatal(err)
	}

	if got := caminhosDoBanco(t, pg); !slices.Equal(got, []string{"agentes/editada/agent.yaml"}) {
		t.Fatalf("a semente sobrescreveu o banco: %v", got)
	}
	bancoIgualAoDisco(t, pg, copia)
	if !strings.Contains(logs.String(), "importar-catalogo") {
		t.Fatalf("faltou o aviso dizendo como levar a pasta ao banco:\n%s", logs.String())
	}
}

func TestPrepararCopiaComBancoIgualASementeNaoAvisa(t *testing.T) {
	var logs bytes.Buffer
	anterior := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(anterior) })
	db := bancotest.Novo(t)
	semente := t.TempDir()
	gravarArquivo(t, semente, "agentes/a/agent.yaml", "nome: a\n")
	pg := catalogo.NovoPostgres(db.Ent)
	copia := filepath.Join(t.TempDir(), "copia")
	if err := prepararCopia(t.Context(), pg, semente, copia); err != nil {
		t.Fatal(err)
	}

	if err := prepararCopia(t.Context(), pg, semente, copia); err != nil {
		t.Fatal(err)
	}

	if strings.Contains(logs.String(), "WARN") {
		t.Fatalf("avisou sem diferença:\n%s", logs.String())
	}
}

func TestPrepararCopiaRecusaSementeQueNaoExiste(t *testing.T) {
	pg := catalogo.NovoPostgres(bancotest.Novo(t).Ent)

	err := prepararCopia(t.Context(), pg, filepath.Join(t.TempDir(), "nao-existe"), t.TempDir())

	if err == nil || !strings.Contains(err.Error(), "-root") {
		t.Fatalf("erro = %v, quer um erro que diga para usar -root", err)
	}
}

// TestEscritasDoCatalogoDeixamBancoIgualAoDisco: criar, editar, duplicar e excluir skill, mais o
// instantâneo de recursos, e depois o "reinício" (abrir de novo materializa o banco).
func TestEscritasDoCatalogoDeixamBancoIgualAoDisco(t *testing.T) {
	db := bancotest.Novo(t)
	pg := catalogo.NovoPostgres(db.Ent)
	semente := sementeDoRepositorio(t)
	cat := abrirCatalogoDeTeste(t, semente, db)
	bancoIgualAoDisco(t, pg, cat.Root)

	passos := []struct {
		nome string
		roda func() error
	}{
		{"cria skill", func() error { _, err := cat.CriarSkill(relConcierge, "skill-nova", "Skill nova"); return err }},
		{"edita textos", func() error {
			textos, versao, err := cat.LerTextos(relMudanca)
			if err != nil {
				return err
			}
			textos.QuandoUsar = "Quando o morador quer marcar a mudança."
			_, err = cat.SalvarTextos(relMudanca, versao, textos)
			return err
		}},
		{"duplica skill", func() error { _, err := cat.DuplicarSkill(relMudanca, "mudanca-copia", "Mudança cópia"); return err }},
		{"salva arquivo", func() error {
			return cat.SalvarArquivo(relConcierge, okf.PersonaMD, []byte("# Persona editada\n"))
		}},
		{"exclui skill", func() error { return cat.ExcluirSkill(relConcierge + "/agentes/skill-nova") }},
		{"salva recurso do instantâneo", func() error {
			p := filepath.Join(cat.Root, "nexus", "cond-001.yaml")
			return cat.Escrever(func() error {
				return okf.SalvarRecursoFixture(p, "", okf.Recurso{Name: "teste.novo", Kind: okf.KindKnowledge, Visibility: "public"})
			})
		}},
	}
	for _, p := range passos {
		if err := p.roda(); err != nil {
			t.Fatalf("%s: %v", p.nome, err)
		}
		bancoIgualAoDisco(t, pg, cat.Root)
	}

	reaberto := abrirCatalogoDeTeste(t, semente, db)

	bancoIgualAoDisco(t, pg, reaberto.Root)
	arvore, err := reaberto.Tree()
	if err != nil {
		t.Fatal(err)
	}
	if okf.Find(arvore, relConcierge+"/agentes/mudanca-copia") == nil {
		t.Error("a skill duplicada sumiu no reinício")
	}
	if okf.Find(arvore, relConcierge+"/agentes/skill-nova") != nil {
		t.Error("a skill excluída voltou no reinício")
	}
	persona, err := os.ReadFile(filepath.Join(reaberto.Root, "agentes", "concierge", okf.PersonaMD))
	if err != nil || string(persona) != "# Persona editada\n" {
		t.Errorf("persona editada perdida no reinício: %q (%v)", persona, err)
	}
	recursos, err := okf.LerRecursosFixture(filepath.Join(reaberto.Root, "nexus", "cond-001.yaml"))
	if err != nil || !slices.ContainsFunc(recursos, func(r okf.Recurso) bool { return r.Name == "teste.novo" }) {
		t.Errorf("recurso do instantâneo perdido no reinício: %v", err)
	}
}

func TestEscritaDevolveErroQuandoOBancoFalhaEACorrecaoVemNaProxima(t *testing.T) {
	db := bancotest.Novo(t)
	pg := catalogo.NovoPostgres(db.Ent)
	cat := abrirCatalogoDeTeste(t, sementeDoRepositorio(t), db)
	var falhaBanco bool
	gancho := persistirNoBanco(pg, cat.Root)
	cat.Persistir = func() error {
		if falhaBanco {
			return context.DeadlineExceeded
		}
		return gancho()
	}

	falhaBanco = true
	if _, err := cat.CriarSkill(relConcierge, "primeira", "Primeira"); err == nil {
		t.Fatal("erro do banco deveria voltar para quem escreveu")
	}
	falhaBanco = false
	if _, err := cat.CriarSkill(relConcierge, "segunda", "Segunda"); err != nil {
		t.Fatal(err)
	}

	bancoIgualAoDisco(t, pg, cat.Root)
	if !slices.Contains(caminhosDoBanco(t, pg), relConcierge+"/agentes/primeira/agent.yaml") {
		t.Error("a diferença da escrita que falhou não foi levada pela próxima")
	}
}

func TestImportarCatalogoDeixaOBancoIgualAPasta(t *testing.T) {
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
	if m != (catalogo.Mudanca{Gravados: 1, Removidos: 1}) {
		t.Fatalf("mudança = %+v", m)
	}
	if got := caminhosDoBanco(t, pg); !slices.Equal(got, []string{"agentes/a/agent.yaml", "agentes/c/agent.yaml"}) {
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
