package conciergeapi

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"go-concierge/internal/core/administracao/regras"
	"go-concierge/internal/core/banco/bancotest"
	"go-concierge/internal/core/catalogo"
	okf "go-concierge/internal/core/catalogo/arquivos"
	"go-concierge/internal/core/dados"
	"go-concierge/internal/entrada"
	"go-concierge/internal/escopo"
	harness "go-concierge/internal/harness/agentes"
	atende "go-concierge/internal/harness/atendimento"
	"go-concierge/internal/harness/canais"
	catalogoruntime "go-concierge/internal/harness/catalogo"
	admin "go-concierge/internal/harness/configuracao"
	"go-concierge/internal/harness/pedidos"
)

func ambienteCatalogoPostgres(t *testing.T) (ambienteRotas, *catalogo.Postgres) {
	t.Helper()
	b := bancotest.Novo(t)
	cat, err := abrirCatalogo(filepath.Join("..", "..", "..", "..", "okf"), b)
	if err != nil {
		t.Fatal(err)
	}
	pg := catalogo.NovoPostgres(b.Ent)
	repo := admin.NovaEmMemoria()
	fonte := func(c *okf.Catalogo) func() []okf.Recurso { return fonteNexusDoRetrato(c, "cond-001", nil) }
	cat.Recursos = fonte(cat)
	store, err := okf.NewCatalogoNexusStorePostgres("cond-001", pg)
	if err != nil {
		t.Fatal(err)
	}
	s := &servidor{cat: cat, catalogoStudio: &repositorioCatalogo{banco: pg, recursos: fonte, prepararRecursos: func(c *okf.Catalogo) func() func() []okf.Recurso {
		return prepararRecursosDoRetrato(c, "cond-001", nil, nil)
	}}, account: "cond-001", nexus: "instantâneo", catalogoNexusStore: store, banco: b, regras: regras.NovoPostgres(b.Ent), admin: repo, atende: &atende.Servico{Agente: relConcierge}, log: log.New(io.Discard, "", 0), registro: slog.New(slog.NewJSONHandler(io.Discard, nil)), autenticador: autenticadorDev{}, authMode: authModeDev, acessos: acessosFake{atribuicoes: usuariosTeste(), mapeamento: repo}, chaveDeIntegracao: []byte(chaveDeTeste)}
	return ambienteRotas{s: s, repo: repo, handler: s.rotear(nil)}, pg
}

func TestStudioZipEPersistenciaDosRecursosNexus(t *testing.T) {
	amb, pg := ambienteCatalogoPostgres(t)
	zipOriginal := requisicaoCatalogo(amb, http.MethodGet, "/api/agentes/exportar?agente="+relMudanca, nil)
	if zipOriginal.Code != http.StatusOK {
		t.Fatalf("exportar: %d %s", zipOriginal.Code, zipOriginal.Body.String())
	}
	zr, err := zip.NewReader(bytes.NewReader(zipOriginal.Body.Bytes()), int64(zipOriginal.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	var novo bytes.Buffer
	zw := zip.NewWriter(&novo)
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		w, err := zw.Create(f.Name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(w, rc); err != nil {
			t.Fatal(err)
		}
		rc.Close()
	}
	w, err := zw.Create("knowledge/importado.md")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("# conteúdo importado")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	res := requisicaoCatalogo(amb, http.MethodPost, "/api/agentes/importar?caminho="+relMudanca, novo.Bytes())
	if res.Code != http.StatusOK {
		t.Fatalf("importar: %d %s", res.Code, res.Body.String())
	}
	res = requisicaoCatalogo(amb, http.MethodGet, "/api/arquivo?agente="+relMudanca+"&caminho=knowledge/importado.md", nil)
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), "conteúdo importado") {
		t.Fatalf("arquivo importado: %d %s", res.Code, res.Body.String())
	}

	recurso := okf.Recurso{Name: "itens.listar", Kind: okf.KindAPI, Visibility: "private", Params: []okf.ParamMeta{}}
	corpo, err := json.Marshal(map[string]any{"original": "", "recurso": recurso})
	if err != nil {
		t.Fatal(err)
	}
	res = requisicaoCatalogo(amb, http.MethodPut, "/api/catalogo-nexus/recurso", corpo)
	if res.Code != http.StatusOK {
		t.Fatalf("catálogo Nexus: %d %s", res.Code, res.Body.String())
	}
	outro, err := okf.NewCatalogoNexusStorePostgres("outra-conta", pg)
	if err != nil {
		t.Fatal(err)
	}
	if lista, err := outro.Listar(); err != nil || len(lista) != 0 {
		t.Fatalf("isolamento: %+v %v", lista, err)
	}
	reaberto, err := okf.NewCatalogoNexusStorePostgres("cond-001", pg)
	if err != nil {
		t.Fatal(err)
	}
	if lista, err := reaberto.Listar(); err != nil || len(lista) != 1 || lista[0].Name != recurso.Name {
		t.Fatalf("reabertura: %+v %v", lista, err)
	}
	res = requisicaoCatalogo(amb, http.MethodPut, "/api/nexus/recurso", corpo)
	if res.Code != http.StatusOK {
		t.Fatalf("instantâneo: %d %s", res.Code, res.Body.String())
	}
	res = requisicaoCatalogo(amb, http.MethodGet, "/api/nexus/recursos", nil)
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), recurso.Name) {
		t.Fatalf("instantâneo relido: %d %s", res.Code, res.Body.String())
	}
	amb.s.nexus = "http://nexus"
	res = requisicaoCatalogo(amb, http.MethodPut, "/api/nexus/recurso", corpo)
	if res.Code != http.StatusConflict {
		t.Fatalf("Nexus ao vivo: %d %s", res.Code, res.Body.String())
	}
}

type canalParaRetrato struct{ canal canais.Canal }

func (c canalParaRetrato) Resolver(context.Context, canais.Identidade) (canais.Canal, error) {
	return c.canal, nil
}
func (c canalParaRetrato) PorID(_ context.Context, empresa string, id uuid.UUID) (canais.Canal, error) {
	if empresa != c.canal.CompanyID || id != c.canal.ID {
		return canais.Canal{}, canais.ErrDesconhecido
	}
	return c.canal, nil
}
func (c canalParaRetrato) Casos(context.Context, canais.Canal) ([]canais.Caso, error) {
	return nil, nil
}

func TestEdicaoStudioNaoAlteraRetratoDeExecucaoJaCarregado(t *testing.T) {
	amb, pg := ambienteCatalogoPostgres(t)
	canal := canais.Canal{ID: uuid.New(), CompanyID: "empresa", Agente: relConcierge, Habilitado: true}
	servidor, cliente, _, err := dados.IniciarConfigurado(dados.Config{Turno: dados.NovoGravadorPostgres(amb.s.banco), Canais: canalParaRetrato{canal}, Catalogo: pg, ContaPadrao: "cond-001"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = servidor.Fechar(context.Background()) })
	amb.s.catalogoStudio.banco = servidor.Catalogo()
	ctx := atende.ComEscopoSessao(t.Context(), "empresa", canal.ID)
	fonte := &catalogoruntime.Fonte{Dados: cliente}
	anterior, err := fonte.Carregar(ctx, "empresa", nil)
	if err != nil {
		t.Fatal(err)
	}
	antes, err := anterior.LerArquivo(relMudanca, "caso.yaml")
	if err != nil {
		t.Fatal(err)
	}
	res := requisicaoCatalogo(amb, http.MethodGet, "/api/skills/agendar-mudanca", nil)
	if res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	var lido textosLidos
	if err := json.Unmarshal(res.Body.Bytes(), &lido); err != nil {
		t.Fatal(err)
	}
	lido.Textos.Resumo = "Nova versão para execução"
	corpo, err := json.Marshal(textosParaGravar{Versao: lido.Versao, Textos: lido.Textos})
	if err != nil {
		t.Fatal(err)
	}
	res = requisicaoCatalogo(amb, http.MethodPut, "/api/skills/agendar-mudanca/textos", corpo)
	if res.Code != 200 {
		t.Fatalf("editar: %d %s", res.Code, res.Body.String())
	}
	antigo, err := anterior.LerArquivo(relMudanca, "caso.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(antes, antigo) {
		t.Fatal("retrato anterior mudou")
	}
	proximo, err := fonte.Carregar(ctx, "empresa", nil)
	if err != nil {
		t.Fatal(err)
	}
	atual, err := proximo.LerArquivo(relMudanca, "caso.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(antes, atual) || !bytes.Contains(atual, []byte(lido.Textos.Resumo)) {
		t.Fatal("próxima execução não recebeu edição")
	}
}

func requisicaoCatalogo(amb ambienteRotas, metodo, rota string, corpo []byte) *httptest.ResponseRecorder {
	req := httptest.NewRequest(metodo, rota, bytes.NewReader(corpo))
	req.Header.Set("X-Dev-Usuario", "admin")
	req.Header.Set("Authorization", "Bearer "+chaveDeTeste)
	w := httptest.NewRecorder()
	amb.handler.ServeHTTP(w, req)
	return w
}

func TestBootCatalogoSemeiaEReabreSemSementeNemCopia(t *testing.T) {
	db := bancotest.Novo(t)
	semente := filepath.Join("..", "..", "..", "..", "okf")
	primeiro, err := abrirCatalogo(semente, db)
	if err != nil {
		t.Fatal(err)
	}
	if arvore, err := primeiro.Tree(); err != nil || len(arvore) == 0 {
		t.Fatalf("semente: %d, %v", len(arvore), err)
	}
	isolado := t.TempDir()
	ausente := filepath.Join(isolado, "sem-semente")
	reaberto, err := abrirCatalogo(ausente, db)
	if err != nil {
		t.Fatal(err)
	}
	if arvore, err := reaberto.Tree(); err != nil || len(arvore) == 0 {
		t.Fatalf("reabertura: %d, %v", len(arvore), err)
	}
	if _, err := os.Stat(ausente); !os.IsNotExist(err) {
		t.Fatalf("criou raiz da semente: %v", err)
	}
	if _, err := os.Stat(filepath.Join(isolado, "var", "catalogo")); !os.IsNotExist(err) {
		t.Fatalf("criou cópia: %v", err)
	}
}

func TestStudioPostgresSalvaReabreExportaEReinicia(t *testing.T) {
	amb, pg := ambienteCatalogoPostgres(t)
	rota := "/api/skills/agendar-mudanca"
	w := requisicaoCatalogo(amb, http.MethodGet, rota, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("leitura %d: %s", w.Code, w.Body.String())
	}
	var lido textosLidos
	if err := json.Unmarshal(w.Body.Bytes(), &lido); err != nil {
		t.Fatal(err)
	}
	lido.Textos.Resumo = "Resumo salvo no PostgreSQL"
	corpo, err := json.Marshal(textosParaGravar{Versao: lido.Versao, Textos: lido.Textos})
	if err != nil {
		t.Fatal(err)
	}
	w = requisicaoCatalogo(amb, http.MethodPut, rota+"/textos", corpo)
	if w.Code != http.StatusOK {
		t.Fatalf("salvar %d: %s", w.Code, w.Body.String())
	}
	w = requisicaoCatalogo(amb, http.MethodGet, rota, nil)
	if w.Code != 200 || !bytes.Contains(w.Body.Bytes(), []byte(lido.Textos.Resumo)) {
		t.Fatalf("reabrir %d: %s", w.Code, w.Body.String())
	}
	w = requisicaoCatalogo(amb, http.MethodGet, "/api/agentes/exportar?agente="+relMudanca, nil)
	if w.Code != 200 {
		t.Fatalf("exportar %d: %s", w.Code, w.Body.String())
	}
	zr, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	achou := false
	for _, f := range zr.File {
		if f.Name == "caso.yaml" {
			rc, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			conteudo, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				t.Fatal(err)
			}
			achou = bytes.Contains(conteudo, []byte(lido.Textos.Resumo))
		}
	}
	if !achou {
		t.Fatal("ZIP não contém edição")
	}
	cat, err := abrirCatalogo(filepath.Join(t.TempDir(), "ausente"), amb.s.banco)
	if err != nil {
		t.Fatal(err)
	}
	reiniciado := &repositorioCatalogo{banco: pg}
	cat, err = reiniciado.retrato(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	textos, _, err := cat.LerTextos(relMudanca)
	if err != nil || textos.Resumo != lido.Textos.Resumo {
		t.Fatalf("após reinício: %+v, %v", textos, err)
	}
}

func TestLotesPostgresValidamContraErrosPreexistentes(t *testing.T) {
	amb, pg := ambienteCatalogoPostgres(t)
	_, err := pg.Substituir(t.Context(), []catalogo.Arquivo{{Caminho: "agentes/outro/agent.yaml", Conteudo: []byte("nome: [")}})
	if err != nil {
		t.Fatal(err)
	}
	valido := loteJSON(t, []arquivoDoPacote{arquivoTexto(arquivoPersona, "persona salva"), arquivoTexto(arquivoNovo, "# novo")}, nil)
	w := requisicaoCatalogo(amb, http.MethodPut, "/integracao/v1/arquivos", []byte(valido))
	if w.Code != 200 {
		t.Fatalf("lote válido %d: %s", w.Code, w.Body.String())
	}
	invalido := loteJSON(t, []arquivoDoPacote{arquivoTexto(arquivoPersona, "não salvar"), arquivoTexto("agentes/concierge/agent.yaml", "nome: [")}, nil)
	w = requisicaoCatalogo(amb, http.MethodPut, "/integracao/v1/arquivos", []byte(invalido))
	if w.Code != 422 {
		t.Fatalf("lote inválido %d: %s", w.Code, w.Body.String())
	}
	cat, err := amb.s.catalogoStudio.retrato(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	persona, err := cat.LerArquivo(relConcierge, "persona.md")
	if err != nil || string(persona) != "persona salva" {
		t.Fatalf("persona=%q, %v", persona, err)
	}
	arquivo, err := cat.LerArquivo(relConcierge, "knowledge/novo.md")
	if err != nil || string(arquivo) != "# novo" {
		t.Fatalf("arquivo=%q, %v", arquivo, err)
	}
	w = requisicaoCatalogo(amb, http.MethodGet, "/integracao/v1/pacote", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "persona salva") {
		t.Fatalf("pacote %d: %s", w.Code, w.Body.String())
	}
}

func TestPermissoesEAlcanceDoCatalogoPostgres(t *testing.T) {
	amb, _ := ambienteCatalogoPostgres(t)
	antes := amb.pedir(t, "admin", http.MethodGet, "/api/skills/agendar-mudanca", "")
	if antes.Code != http.StatusOK {
		t.Fatalf("leitura do admin: %d %s", antes.Code, antes.Body.String())
	}
	leitor := amb.pedir(t, "gestor-a", http.MethodGet, "/api/skills/agendar-mudanca", "")
	if leitor.Code != http.StatusOK {
		t.Fatalf("leitura do gestor: %d %s", leitor.Code, leitor.Body.String())
	}
	for _, rota := range []struct{ metodo, caminho, corpo string }{
		{http.MethodPost, "/api/skills", `{"nome":"nao-criar","titulo":"Não criar"}`},
		{http.MethodPut, "/api/skills/agendar-mudanca/textos", `{}`},
		{http.MethodDelete, "/api/skills/agendar-mudanca", ""},
	} {
		res := amb.pedir(t, "gestor-a", rota.metodo, rota.caminho, rota.corpo)
		if res.Code != http.StatusForbidden {
			t.Fatalf("%s %s: %d %s", rota.metodo, rota.caminho, res.Code, res.Body.String())
		}
	}
	depois := amb.pedir(t, "admin", http.MethodGet, "/api/skills/agendar-mudanca", "")
	if depois.Code != http.StatusOK || !bytes.Equal(antes.Body.Bytes(), depois.Body.Bytes()) {
		t.Fatal("pedido sem permissão alterou catálogo")
	}
}

func TestMigracaoDaCopiaNexusLegadaNaoSobrescreveOBanco(t *testing.T) {
	db := bancotest.Novo(t)
	pg := catalogo.NovoPostgres(db.Ent)
	raiz := t.TempDir()
	pasta := filepath.Join(raiz, "catalogo-nexus")
	if err := os.MkdirAll(pasta, 0700); err != nil {
		t.Fatal(err)
	}
	caminho := filepath.Join(pasta, "cond-001.yaml")
	original := []byte("recursos:\n  - name: legado\n    kind: api\n    visibility: private\n")
	if err := os.WriteFile(caminho, original, 0600); err != nil {
		t.Fatal(err)
	}
	if err := migrarCatalogoNexusLegado(raiz, "cond-001", pg); err != nil {
		t.Fatal(err)
	}
	store, err := okf.NewCatalogoNexusStorePostgres("cond-001", pg)
	if err != nil {
		t.Fatal(err)
	}
	if lista, err := store.Listar(); err != nil || len(lista) != 1 || lista[0].Name != "legado" {
		t.Fatalf("legado: %+v %v", lista, err)
	}
	if err := store.Salvar("legado", okf.Recurso{Name: "atualizado", Kind: okf.KindAPI, Visibility: "private", Params: []okf.ParamMeta{}}); err != nil {
		t.Fatal(err)
	}
	if err := migrarCatalogoNexusLegado(raiz, "cond-001", pg); err != nil {
		t.Fatal(err)
	}
	if lista, err := store.Listar(); err != nil || len(lista) != 1 || lista[0].Name != "atualizado" {
		t.Fatalf("banco sobrescrito: %+v %v", lista, err)
	}
	if raw, err := os.ReadFile(caminho); err != nil || !bytes.Equal(raw, original) {
		t.Fatalf("legado alterado: %v", err)
	}
}

func TestOperacoesDeSkillEFluxoPelaAPIComPostgres(t *testing.T) {
	amb, _ := ambienteCatalogoPostgres(t)
	res := requisicaoCatalogo(amb, http.MethodPost, "/api/skills", []byte(`{"nome":"nova-skill","titulo":"Nova skill"}`))
	if res.Code != http.StatusCreated {
		t.Fatalf("criar: %d %s", res.Code, res.Body.String())
	}
	res = requisicaoCatalogo(amb, http.MethodPost, "/api/skills/agendar-mudanca/duplicar", []byte(`{"nome":"mudanca-copia","titulo":"Mudança cópia"}`))
	if res.Code != http.StatusCreated {
		t.Fatalf("duplicar: %d %s", res.Code, res.Body.String())
	}
	res = requisicaoCatalogo(amb, http.MethodGet, "/api/skills/agendar-mudanca/fluxo", nil)
	if res.Code != 200 {
		t.Fatalf("ler fluxo: %d %s", res.Code, res.Body.String())
	}
	var lido okf.FluxoLido
	if err := json.Unmarshal(res.Body.Bytes(), &lido); err != nil {
		t.Fatal(err)
	}
	alterou := false
	for i := range lido.Fluxo.Blocos {
		if lido.Fluxo.Blocos[i].Tipo == okf.BlocoConfirmar {
			lido.Fluxo.Blocos[i].Texto = "Confirma a alteração?"
			alterou = true
			break
		}
	}
	if !alterou {
		t.Fatal("fluxo sem confirmação")
	}
	corpo, err := json.Marshal(fluxoParaGravar{Versao: lido.Versao, Fluxo: lido.Fluxo})
	if err != nil {
		t.Fatal(err)
	}
	res = requisicaoCatalogo(amb, http.MethodPut, "/api/skills/agendar-mudanca/fluxo", corpo)
	if res.Code != 200 {
		t.Fatalf("salvar fluxo: %d %s", res.Code, res.Body.String())
	}
	res = requisicaoCatalogo(amb, http.MethodGet, "/api/skills/agendar-mudanca/fluxo", nil)
	if res.Code != 200 || !strings.Contains(res.Body.String(), "Confirma a alteração?") {
		t.Fatalf("reler fluxo: %d %s", res.Code, res.Body.String())
	}
	for _, nome := range []string{"nova-skill", "mudanca-copia"} {
		res = requisicaoCatalogo(amb, http.MethodDelete, "/api/skills/"+nome, nil)
		if res.Code != http.StatusNoContent {
			t.Fatalf("excluir %s: %d %s", nome, res.Code, res.Body.String())
		}
	}
}

func TestEdicoesConcorrentesPelaAPIPreservamArquivosIndependentes(t *testing.T) {
	amb, pg := ambienteCatalogoPostgres(t)
	servidor, _, _, err := dados.IniciarConfigurado(dados.Config{Turno: dados.NovoGravadorPostgres(amb.s.banco), Catalogo: pg})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = servidor.Fechar(context.Background()) })
	amb.s.catalogoStudio.banco = servidor.Catalogo()
	resultados := make(chan *httptest.ResponseRecorder, 2)
	for _, nome := range []string{"concorrente-a", "concorrente-b"} {
		nome := nome
		go func() {
			corpo, _ := json.Marshal(map[string]string{"agente": relConcierge, "caminho": "knowledge/" + nome + ".md", "conteudo": nome})
			resultados <- requisicaoCatalogo(amb, http.MethodPut, "/api/arquivo", corpo)
		}()
	}
	for range 2 {
		res := <-resultados
		if res.Code != 200 {
			t.Fatalf("gravação: %d %s", res.Code, res.Body.String())
		}
	}
	for _, nome := range []string{"concorrente-a", "concorrente-b"} {
		res := requisicaoCatalogo(amb, http.MethodGet, "/api/arquivo?agente="+relConcierge+"&caminho=knowledge/"+nome+".md", nil)
		if res.Code != 200 || !strings.Contains(res.Body.String(), nome) {
			t.Fatalf("leitura: %d %s", res.Code, res.Body.String())
		}
	}
}

func TestEdicaoNaoAplicadaNaoReapareceNaOperacaoSeguinte(t *testing.T) {
	amb, pg := ambienteCatalogoPostgres(t)
	chamadas := 0
	amb.s.catalogoStudio.banco = bancoComFalhaCatalogo{armazenamentoCatalogo: pg, substituir: func(ctx context.Context, gravar []catalogo.Arquivo, remover ...string) (catalogo.Mudanca, error) {
		chamadas++
		if chamadas == 1 {
			return catalogo.Mudanca{}, errors.New("gravação recusada")
		}
		return pg.Substituir(ctx, gravar, remover...)
	}}
	cat, err := amb.s.catalogoStudio.retrato(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cat.CriarSkill(relConcierge, "primeira", "Primeira"); err == nil {
		t.Fatal("primeira gravação deveria falhar")
	}
	if _, err := cat.CriarSkill(relConcierge, "segunda", "Segunda"); err != nil {
		t.Fatal(err)
	}
	arquivos, err := pg.Listar(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	var primeira, segunda bool
	for _, a := range arquivos {
		primeira = primeira || strings.HasPrefix(a.Caminho, relConcierge+"/agentes/primeira/")
		segunda = segunda || strings.HasPrefix(a.Caminho, relConcierge+"/agentes/segunda/")
	}
	if primeira || !segunda {
		t.Fatalf("primeira=%v segunda=%v", primeira, segunda)
	}
}

func TestCriacaoDeInstantaneoNexusAusentePelaAPI(t *testing.T) {
	amb, pg := ambienteCatalogoPostgres(t)
	if _, err := pg.Substituir(t.Context(), nil, "nexus/cond-001.yaml"); err != nil {
		t.Fatal(err)
	}
	recurso := okf.Recurso{Name: "novo.recurso", Kind: okf.KindAPI, Visibility: "private", Params: []okf.ParamMeta{}}
	corpo, err := json.Marshal(map[string]any{"original": "", "recurso": recurso})
	if err != nil {
		t.Fatal(err)
	}
	res := requisicaoCatalogo(amb, http.MethodPut, "/api/nexus/recurso", corpo)
	if res.Code != 200 || !strings.Contains(res.Body.String(), "novo.recurso") || !strings.Contains(res.Body.String(), `"conferido":true`) {
		t.Fatalf("resposta de criação: %d %s", res.Code, res.Body.String())
	}
	res = requisicaoCatalogo(amb, http.MethodGet, "/api/nexus/recursos", nil)
	if res.Code != 200 || !strings.Contains(res.Body.String(), "novo.recurso") {
		t.Fatalf("leitura posterior: %d %s", res.Code, res.Body.String())
	}
}

func TestSkillAbreESalvaComCatalogoMaiorQueLimiteDoTransporte(t *testing.T) {
	amb, pg := ambienteCatalogoPostgres(t)
	grande := bytes.Repeat([]byte("x"), 1<<20)
	arquivos := make([]catalogo.Arquivo, 50)
	for i := range arquivos {
		arquivos[i] = catalogo.Arquivo{Caminho: fmt.Sprintf("agentes/outro/knowledge/grande-%02d.txt", i), Conteudo: grande}
	}
	arquivos[0].Conteudo = bytes.Repeat([]byte("y"), 5<<20)
	if _, err := pg.Substituir(t.Context(), arquivos); err != nil {
		t.Fatal(err)
	}
	servidor, _, _, err := dados.IniciarConfigurado(dados.Config{Turno: dados.NovoGravadorPostgres(amb.s.banco), Catalogo: pg})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = servidor.Fechar(context.Background()) })
	amb.s.catalogoStudio.banco = servidor.Catalogo()
	rota := "/api/skills/agendar-mudanca"
	res := requisicaoCatalogo(amb, http.MethodGet, rota, nil)
	if res.Code != http.StatusOK {
		t.Fatalf("abrir: %d %s", res.Code, res.Body.String())
	}
	var lido textosLidos
	if err := json.Unmarshal(res.Body.Bytes(), &lido); err != nil {
		t.Fatal(err)
	}
	lido.Textos.Resumo = "Edição com catálogo acima de 64 MiB codificados"
	corpo, err := json.Marshal(textosParaGravar{Versao: lido.Versao, Textos: lido.Textos})
	if err != nil {
		t.Fatal(err)
	}
	res = requisicaoCatalogo(amb, http.MethodPut, rota+"/textos", corpo)
	if res.Code != http.StatusOK {
		t.Fatalf("salvar: %d %s", res.Code, res.Body.String())
	}
	res = requisicaoCatalogo(amb, http.MethodGet, rota, nil)
	if res.Code != 200 || !strings.Contains(res.Body.String(), lido.Textos.Resumo) {
		t.Fatalf("reabrir: %d %s", res.Code, res.Body.String())
	}
	var tamanho int
	if err := amb.s.banco.Pool.QueryRow(t.Context(), `SELECT octet_length(content) FROM catalog_files WHERE path=$1`, arquivos[0].Caminho).Scan(&tamanho); err != nil || tamanho != 5<<20 {
		t.Fatalf("arquivo grande alterado: tamanho=%d erro=%v", tamanho, err)
	}
}

func TestFluxoNaoUsaClassificacaoAnteriorARecursoConcorrente(t *testing.T) {
	amb, _ := ambienteCatalogoPostgres(t)
	res := requisicaoCatalogo(amb, http.MethodGet, "/api/skills/agendar-mudanca/fluxo", nil)
	if res.Code != 200 {
		t.Fatalf("ler fluxo: %d %s", res.Code, res.Body.String())
	}
	var lido okf.FluxoLido
	if err := json.Unmarshal(res.Body.Bytes(), &lido); err != nil {
		t.Fatal(err)
	}
	var recurso okf.Recurso
	for _, r := range amb.s.cat.RecursosNexus() {
		if r.Name == "unidades.listar" {
			recurso = r
			break
		}
	}
	if recurso.Name == "" || recurso.Write {
		t.Fatal("fixture não contém recurso de leitura")
	}
	recurso.Write = true
	corpoRecurso, err := json.Marshal(map[string]any{"original": recurso.Name, "recurso": recurso})
	if err != nil {
		t.Fatal(err)
	}
	corpoFluxo, err := json.Marshal(fluxoParaGravar{Versao: lido.Versao, Fluxo: lido.Fluxo})
	if err != nil {
		t.Fatal(err)
	}
	pronto, continuar := make(chan struct{}), make(chan struct{})
	liberar := sync.OnceFunc(func() { close(continuar) })
	defer liberar()
	original := amb.s.catalogoStudio.prepararRecursos
	amb.s.catalogoStudio.prepararRecursos = func(cat *okf.Catalogo) func() func() []okf.Recurso {
		preparar := original(cat)
		return func() func() []okf.Recurso { fonte := preparar(); close(pronto); <-continuar; return fonte }
	}
	resultado := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		resultado <- requisicaoCatalogo(amb, http.MethodPut, "/api/skills/agendar-mudanca/fluxo", corpoFluxo)
	}()
	select {
	case <-pronto:
	case resposta := <-resultado:
		t.Fatalf("fluxo terminou antes da sincronização: %d %s", resposta.Code, resposta.Body.String())
	case <-time.After(15 * time.Second):
		t.Fatal("fluxo não chegou à sincronização")
	}
	res = requisicaoCatalogo(amb, http.MethodPut, "/api/nexus/recurso", corpoRecurso)
	if res.Code != 200 {
		liberar()
		<-resultado
		t.Fatalf("salvar recurso: %d %s", res.Code, res.Body.String())
	}
	liberar()
	res = <-resultado
	if res.Code != http.StatusConflict {
		t.Fatalf("fluxo com versão anterior: %d %s", res.Code, res.Body.String())
	}
	atual := requisicaoCatalogo(amb, http.MethodGet, "/api/skills/agendar-mudanca/fluxo", nil)
	if atual.Code != 200 {
		t.Fatalf("reler: %d %s", atual.Code, atual.Body.String())
	}
	var novo okf.FluxoLido
	if err := json.Unmarshal(atual.Body.Bytes(), &novo); err != nil {
		t.Fatal(err)
	}
	if novo.Versao == lido.Versao {
		t.Fatal("classificação atual não mudou a versão")
	}
}

type mapeamentoParaPedido struct{ *admin.EmMemoria }

func (m mapeamentoParaPedido) Conteudo(ctx context.Context, _ string, _ *int) (admin.Mapeamento, error) {
	return m.Ler(ctx)
}

type canalParaPedido struct{ canal canais.Canal }

func (c canalParaPedido) Resolver(context.Context, canais.Identidade) (canais.Canal, error) {
	return c.canal, nil
}
func (c canalParaPedido) PorID(_ context.Context, empresa string, id uuid.UUID) (canais.Canal, error) {
	if empresa != c.canal.CompanyID || id != c.canal.ID {
		return canais.Canal{}, canais.ErrDesconhecido
	}
	return c.canal, nil
}
func (c canalParaPedido) Casos(context.Context, canais.Canal) ([]canais.Caso, error) {
	return []canais.Caso{{ID: "reserva_area_comum", Habilitado: true, Padrao: true}}, nil
}

func TestEdicaoStudioAtualizaCarteiraEPreenchimentoNoMesmoProcesso(t *testing.T) {
	amb, pg := ambienteCatalogoPostgres(t)
	condo := 101
	canal := canais.Canal{ID: uuid.New(), CompanyID: "empresa", CondoID: &condo, Agente: relConcierge, Habilitado: true, Modo: "INHERIT"}
	canaisTeste := canalParaPedido{canal}
	servidor, cliente, _, err := dados.IniciarConfigurado(dados.Config{Turno: dados.NovoGravadorPostgres(amb.s.banco), Canais: canaisTeste, Catalogo: pg, ContaPadrao: "cond-001"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = servidor.Fechar(context.Background()) })
	amb.s.catalogoStudio.banco = servidor.Catalogo()
	ctx := atende.ComEscopoSessao(t.Context(), "empresa", canal.ID)
	telefone := "5511999990000"
	contato, err := escopo.NovoContato("empresa", &condo, telefone)
	if err != nil {
		t.Fatal(err)
	}
	origem := entrada.Origem{Conector: "atende-emulado", Meio: "whatsapp", ConexaoID: canal.ID.String()}
	agora := time.Now().UTC().Truncate(time.Microsecond)
	dadosSessao, err := json.Marshal(map[string]any{"id": "sessao-teste", "company_id": "empresa", "channel_id": canal.ID, "condo_id": condo, "protocolo": "protocolo-teste", "contato": entrada.Contato{Phone: telefone}, "origem": origem, "escopo": contato, "atualizada_em": agora, "criada_em": agora, "turnos": []any{}, "identidade": map[string]any{"authenticated": true, "role": "morador"}})
	if err != nil {
		t.Fatal(err)
	}
	storeSessoes := atende.NovaSessaoEmMemoria()
	if err := storeSessoes.Salvar(ctx, atende.RegistroSessao{ID: "sessao-teste", CompanyID: "empresa", ChannelID: canal.ID, CondoID: &condo, Connector: origem.Conector, Protocolo: "protocolo-teste", UserPhone: telefone, AtualizadaEm: agora, Dados: dadosSessao}); err != nil {
		t.Fatal(err)
	}
	pasta, err := amb.s.cat.ResolveAgente(relConcierge + "/agentes/reserva-area-comum")
	if err != nil {
		t.Fatal(err)
	}
	agente, err := amb.s.cat.Load(pasta)
	if err != nil {
		t.Fatal(err)
	}
	contrato, err := pedidos.ContratoDe(agente.Caso)
	if err != nil {
		t.Fatal(err)
	}
	carteira := pedidos.Carteira{CompanyID: "empresa", Telefone: telefone}
	pedido := carteira.Novo(contrato, "reserva-area-comum", condo, pedidos.Origem{})
	storePedidos := pedidos.NovoFake()
	if err := storePedidos.Gravar(ctx, contato, carteira); err != nil {
		t.Fatal(err)
	}
	amb.s.atende.Cat = amb.s.cat
	amb.s.atende.Catalogos = &catalogoruntime.Fonte{Dados: cliente}
	amb.s.atende.Canais = canaisTeste
	amb.s.atende.Mapeamento = mapeamentoParaPedido{amb.repo}
	amb.s.atende.Sessoes = atende.NewSessoes(storeSessoes)
	amb.s.atende.Pedidos = storePedidos
	amb.s.atende.Config = harness.Config{Mock: true, Env: func(string) string { return "" }}
	amb.s.outbox = &atende.Outbox{}
	amb.s.atende.Sender = amb.s.outbox
	caso, err := amb.s.cat.LerArquivo(relConcierge+"/agentes/reserva-area-comum", "caso.yaml")
	if err != nil {
		t.Fatal(err)
	}
	alterado := strings.Replace(string(caso), "params:\n", "params:\n  - name: observacao\n    title: Observação atualizada\n    ask: Qual observação?\n", 1)
	alterado = strings.Replace(alterado, "- collect: [data]", "- collect: [data, observacao]", 1)
	corpo, err := json.Marshal(map[string]string{"agente": relConcierge + "/agentes/reserva-area-comum", "caminho": "caso.yaml", "conteudo": alterado})
	if err != nil {
		t.Fatal(err)
	}
	res := requisicaoCatalogo(amb, http.MethodPut, "/api/arquivo", corpo)
	if res.Code != 200 {
		t.Fatalf("editar caso: %d %s", res.Code, res.Body.String())
	}
	res = requisicaoCatalogo(amb, http.MethodGet, "/api/pedidos?company_id=empresa&condo_id=101&phone="+telefone, nil)
	if res.Code != 200 || !strings.Contains(res.Body.String(), "Observação atualizada") {
		t.Fatalf("carteira: %d %s", res.Code, res.Body.String())
	}
	corpo, err = json.Marshal(map[string]any{"acao": "preencher", "sessao_id": "sessao-teste", "pedido": pedido.Protocolo, "atendente": "Ana", "valores": map[string]any{"observacao": "Ligar antes"}})
	if err != nil {
		t.Fatal(err)
	}
	res = requisicaoCatalogo(amb, http.MethodPost, "/api/pedidos/acao?company_id=empresa", corpo)
	if res.Code != 200 || !strings.Contains(res.Body.String(), "Ligar antes") {
		t.Fatalf("preencher: %d %s", res.Code, res.Body.String())
	}
}

func TestPaginaPostgresDetectaAlteracaoEntreConsultas(t *testing.T) {
	_, pg := ambienteCatalogoPostgres(t)
	primeira, err := pg.ListarPagina(t.Context(), "", 0)
	if err != nil || primeira.Fim {
		t.Fatalf("primeira página: %+v %v", primeira, err)
	}
	if _, err := pg.Substituir(t.Context(), []catalogo.Arquivo{{Caminho: arquivoPersona, Conteudo: []byte("alterada entre páginas")}}); err != nil {
		t.Fatal(err)
	}
	segunda, err := pg.ListarPagina(t.Context(), primeira.Proximo, primeira.Deslocamento)
	if err != nil {
		t.Fatal(err)
	}
	if primeira.Versao == segunda.Versao {
		t.Fatal("alteração não modificou a versão da leitura")
	}
}
