package dados

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"go-concierge/internal/core/banco/bancotest"
	"go-concierge/internal/core/catalogo"
)

type catalogoPaginadoTeste struct {
	arquivos []catalogo.Arquivo
	chamadas int
	mudar    bool
}

func TestCatalogoPaginadoUsaOrdemBinariaEmColacaoLinguistica(t *testing.T) {
	b := bancotest.Novo(t)
	if _, err := b.Pool.Exec(t.Context(), `CREATE COLLATION catalogo_teste_linguistico (provider = icu, locale = 'pt-BR')`); err != nil {
		t.Fatalf("criar colação linguística: %v", err)
	}
	if _, err := b.Pool.Exec(t.Context(), `ALTER TABLE catalog_files ALTER COLUMN path TYPE text COLLATE catalogo_teste_linguistico`); err != nil {
		t.Fatalf("configurar colação da coluna: %v", err)
	}
	pg := catalogo.NovoPostgres(b.Ent)
	nomes := []string{"Z", "a", "á", "A", "b", "B", "c", "C", "d", "D", "e", "E", "z", "é", "É", "ñ", "n", "9", "2"}
	arquivos := make([]catalogo.Arquivo, 0, len(nomes))
	for _, nome := range nomes {
		conteudo := []byte("conteúdo " + nome)
		if nome == "B" {
			conteudo = bytes.Repeat([]byte("x"), catalogo.TamanhoTrecho+17)
		}
		arquivos = append(arquivos, catalogo.Arquivo{Caminho: "agentes/a/knowledge/" + nome + ".md", Conteudo: conteudo})
	}
	if _, err := pg.Substituir(t.Context(), arquivos); err != nil {
		t.Fatal(err)
	}
	var linguistica, binaria []string
	if err := b.Pool.QueryRow(t.Context(), `SELECT array_agg(path ORDER BY path), array_agg(path ORDER BY path COLLATE "C") FROM catalog_files`).Scan(&linguistica, &binaria); err != nil {
		t.Fatal(err)
	}
	if slices.Equal(linguistica, binaria) {
		t.Fatalf("o teste exige ordens distintas: %v", linguistica)
	}
	slices.SortFunc(arquivos, func(a, b catalogo.Arquivo) int { return strings.Compare(a.Caminho, b.Caminho) })
	hash := md5.New()
	for _, arquivo := range arquivos {
		fmt.Fprintf(hash, "%d:%s%s", utf8.RuneCountInString(arquivo.Caminho), arquivo.Caminho, catalogo.SHA256(arquivo.Conteudo))
	}
	versao := hex.EncodeToString(hash.Sum(nil))
	var caminho string
	var deslocamento int64
	paginas := 0
	for {
		pagina, err := pg.ListarPagina(t.Context(), caminho, deslocamento)
		if err != nil {
			t.Fatal(err)
		}
		paginas++
		if pagina.Versao != versao {
			t.Fatalf("versão não segue ordem binária: %s, esperada %s", pagina.Versao, versao)
		}
		if pagina.Fim {
			break
		}
		caminho, deslocamento = pagina.Proximo, pagina.Deslocamento
		if paginas > len(nomes)+3 {
			t.Fatal("paginação não terminou")
		}
	}
	if paginas < 2 {
		t.Fatalf("não atravessou páginas: %d", paginas)
	}
	servidor, _, _, err := IniciarConfigurado(Config{Turno: gravadorFunc(nil), Catalogo: pg})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = servidor.Fechar(context.Background()) })
	recebidos, err := servidor.Catalogo().Listar(t.Context())
	if err != nil {
		t.Fatalf("listar pelo transporte interno: %v", err)
	}
	if !slices.EqualFunc(recebidos, arquivos, func(a, b catalogo.Arquivo) bool { return a.Caminho == b.Caminho && bytes.Equal(a.Conteudo, b.Conteudo) }) {
		t.Fatalf("arquivos paginados diferem do esperado: recebidos=%d esperados=%d", len(recebidos), len(arquivos))
	}
}

func (s *catalogoPaginadoTeste) Listar(context.Context) ([]catalogo.Arquivo, error) {
	return s.arquivos, nil
}
func (s *catalogoPaginadoTeste) ListarParaExecucao(context.Context, string, string) ([]catalogo.Arquivo, error) {
	return s.arquivos, nil
}
func (s *catalogoPaginadoTeste) Substituir(context.Context, []catalogo.Arquivo, ...string) (catalogo.Mudanca, error) {
	return catalogo.Mudanca{}, nil
}
func (s *catalogoPaginadoTeste) ListarPagina(_ context.Context, caminho string, deslocamento int64) (catalogo.Pagina, error) {
	s.chamadas++
	if s.mudar && s.chamadas == 2 {
		s.arquivos[0].Conteudo = []byte("nova versão")
	}
	return catalogo.Paginar(s.arquivos, caminho, deslocamento)
}

func TestCatalogoHTTPReiniciaPaginacaoQuandoVersaoMuda(t *testing.T) {
	store := &catalogoPaginadoTeste{mudar: true}
	for i := range 10 {
		store.arquivos = append(store.arquivos, catalogo.Arquivo{Caminho: fmt.Sprintf("agentes/a/%02d", i), Conteudo: []byte("original")})
	}
	store.arquivos[1].Conteudo = bytes.Repeat([]byte("a"), catalogo.TamanhoTrecho+9)
	servidor, _, _, err := IniciarConfigurado(Config{Turno: gravadorFunc(nil), Catalogo: store})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = servidor.Fechar(context.Background()) })
	arquivos, err := servidor.Catalogo().Listar(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if store.chamadas < 4 || len(arquivos) != 10 || string(arquivos[0].Conteudo) != "nova versão" || !bytes.Equal(arquivos[1].Conteudo, store.arquivos[1].Conteudo) {
		t.Fatalf("retrato misturado: chamadas=%d arquivos=%d", store.chamadas, len(arquivos))
	}
}
