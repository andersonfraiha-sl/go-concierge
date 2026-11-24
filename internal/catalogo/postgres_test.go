package catalogo

import (
	"errors"
	"os"
	"slices"
	"testing"
	"time"

	"go-concierge/internal/banco/bancotest"
	entcatalogo "go-concierge/internal/banco/ent/catalogoarquivo"
)

func TestMain(m *testing.M) {
	codigo := m.Run()
	bancotest.Encerrar()
	os.Exit(codigo)
}

func novoStore(t *testing.T) *Postgres {
	t.Helper()
	return NovoPostgres(bancotest.Novo(t).Ent)
}

func substituir(t *testing.T, p *Postgres, arquivos ...Arquivo) Mudanca {
	t.Helper()
	m, err := p.Substituir(t.Context(), arquivos)
	if err != nil {
		t.Fatalf("Substituir: %v", err)
	}
	return m
}

func TestSubstituirGravaListaEPreservaOsBytes(t *testing.T) {
	p := novoStore(t)
	binario := []byte{0x89, 'P', 'N', 'G', 0x00, 0xff}
	m := substituir(t, p, Arquivo{"nexus/c.yaml", []byte("recursos: []")}, Arquivo{"agentes/a/img.png", binario})

	if m != (Mudanca{Gravados: 2}) {
		t.Fatalf("mudança = %+v", m)
	}
	lidos, err := p.Listar(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if got := caminhos(lidos); !slices.Equal(got, []string{"agentes/a/img.png", "nexus/c.yaml"}) {
		t.Fatalf("caminhos = %v", got)
	}
	if string(lidos[0].Conteudo) != string(binario) {
		t.Fatalf("bytes = %v, quer %v", lidos[0].Conteudo, binario)
	}
}

func TestSubstituirRepetidoNaoReescreveNada(t *testing.T) {
	p := novoStore(t)
	disco := []Arquivo{{"agentes/a.md", []byte("a")}, {"agentes/b.md", []byte("b")}}
	substituir(t, p, disco...)
	antes := atualizadoEm(t, p, "agentes/a.md")
	time.Sleep(5 * time.Millisecond)

	m := substituir(t, p, disco...)

	if m != (Mudanca{}) {
		t.Fatalf("segunda mudança = %+v, quer nenhuma", m)
	}
	if depois := atualizadoEm(t, p, "agentes/a.md"); !depois.Equal(antes) {
		t.Fatalf("atualizado_em mudou de %v para %v sem o conteúdo mudar", antes, depois)
	}
}

func TestSubstituirAplicaSoADiferenca(t *testing.T) {
	p := novoStore(t)
	substituir(t, p, Arquivo{"agentes/fica.md", []byte("fica")}, Arquivo{"agentes/muda.md", []byte("velho")}, Arquivo{"agentes/some.md", []byte("some")})
	antesFica := atualizadoEm(t, p, "agentes/fica.md")
	time.Sleep(5 * time.Millisecond)

	m, err := p.Substituir(t.Context(), []Arquivo{{"agentes/fica.md", []byte("fica")}, {"agentes/muda.md", []byte("novo")}, {"agentes/chega.md", []byte("chega")}}, "agentes/some.md")
	if err != nil {
		t.Fatal(err)
	}

	if m != (Mudanca{Gravados: 2, Removidos: 1}) {
		t.Fatalf("mudança = %+v", m)
	}
	lidos, err := p.Listar(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if got := caminhos(lidos); !slices.Equal(got, []string{"agentes/chega.md", "agentes/fica.md", "agentes/muda.md"}) {
		t.Fatalf("caminhos = %v", got)
	}
	if string(lidos[2].Conteudo) != "novo" {
		t.Fatalf("muda.md = %q", lidos[2].Conteudo)
	}
	if depois := atualizadoEm(t, p, "agentes/fica.md"); !depois.Equal(antesFica) {
		t.Fatal("o arquivo que não mudou foi regravado")
	}
}

func TestSubstituirComDiscoVazioPreservaOBanco(t *testing.T) {
	p := novoStore(t)
	substituir(t, p, Arquivo{"agentes/a.md", []byte("a")})

	substituir(t, p)

	vazio, err := p.Vazio(t.Context())
	if err != nil || vazio {
		t.Fatalf("Vazio = %v, %v", vazio, err)
	}
}

func TestSubstituirRecusaCaminhoInvalidoSemGravarNada(t *testing.T) {
	p := novoStore(t)

	_, err := p.Substituir(t.Context(), []Arquivo{{"agentes/bom.md", []byte("b")}, {"agentes/../fora.md", []byte("x")}})

	if !errors.Is(err, ErrCaminhoInvalido) {
		t.Fatalf("erro = %v, quer ErrCaminhoInvalido", err)
	}
	if vazio, _ := p.Vazio(t.Context()); !vazio {
		t.Fatal("gravou parte do lote")
	}
}

func TestVazioEResumos(t *testing.T) {
	p := novoStore(t)
	if vazio, err := p.Vazio(t.Context()); err != nil || !vazio {
		t.Fatalf("Vazio no começo = %v, %v", vazio, err)
	}

	substituir(t, p, Arquivo{"agentes/a.md", []byte("a")})

	if vazio, err := p.Vazio(t.Context()); err != nil || vazio {
		t.Fatalf("Vazio depois de gravar = %v, %v", vazio, err)
	}
	resumos, err := p.Resumos(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if resumos["agentes/a.md"] != SHA256([]byte("a")) || len(resumos) != 1 {
		t.Fatalf("resumos = %v", resumos)
	}
}

func TestSubstituirGravaMaisArquivosQueUmLote(t *testing.T) {
	p := novoStore(t)
	var disco []Arquivo
	for i := range arquivosPorLote*2 + 1 {
		disco = append(disco, Arquivo{"agentes/a" + string(rune('a'+i%26)) + "/" + string(rune('a'+i/26)) + ".md", []byte{byte(i)}})
	}

	m := substituir(t, p, disco...)

	if m.Gravados != len(disco) {
		t.Fatalf("gravados = %d, quer %d", m.Gravados, len(disco))
	}
}

func atualizadoEm(t *testing.T, p *Postgres, caminho string) time.Time {
	t.Helper()
	linha, err := p.cli.CatalogoArquivo.Query().Where(entcatalogo.ID(caminho)).Only(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	return linha.AtualizadoEm
}

func TestCatalogoV2ProjetaCasoComIdentidadeGlobalNaMesmaTransacao(t *testing.T) {
	p := novoStore(t)
	ctx := t.Context()
	a := Arquivo{"agentes/a/caso.yaml", []byte("id: global-a\nname: Caso A\nsummary: Resumo\nsteps:\n  - collect: [nome]\n")}
	substituir(t, p, a)
	rows, err := p.cli.QueryContext(ctx, `SELECT id,file_path,agent_path,name,visibility,source_sha256 FROM use_cases`)
	if err != nil {
		t.Fatal(err)
	}
	if !rows.Next() {
		t.Fatal("caso não projetado")
	}
	var id, file, agent, name, visibility, sha string
	if err := rows.Scan(&id, &file, &agent, &name, &visibility, &sha); err != nil {
		t.Fatal(err)
	}
	rows.Close()
	if id != "global-a" || file != a.Caminho || agent != "agentes/a" || name != "Caso A" || visibility != "private" || sha != SHA256(a.Conteudo) {
		t.Fatalf("projeção incorreta: %s %s %s %s %s %s", id, file, agent, name, visibility, sha)
	}
	duplicate := Arquivo{"agentes/b/caso.yaml", a.Conteudo}
	if _, err := p.Substituir(ctx, []Arquivo{a, duplicate}); err == nil {
		t.Fatal("ID global duplicado aceito")
	}
	files, err := p.Listar(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Caminho != a.Caminho {
		t.Fatal("catálogo parcialmente alterado")
	}
	invalid := Arquivo{a.Caminho, []byte("id: global-b\nsteps: []\n")}
	if _, err := p.Substituir(ctx, []Arquivo{invalid}); err == nil {
		t.Fatal("caso inválido aceito")
	}
	files, err = p.Listar(ctx)
	if err != nil || len(files) != 1 || string(files[0].Conteudo) != string(a.Conteudo) {
		t.Fatal("conteúdo canônico alterado na falha")
	}
}

func TestSubstituirParcialPreservaCasosForaDosAlvosERemoveSomenteCasoExplicitoSemUso(t *testing.T) {
	p := novoStore(t)
	a := Arquivo{"agentes/a/caso.yaml", []byte("id: caso-a\nsteps:\n  - collect: [nome]\n")}
	b := Arquivo{"agentes/b/caso.yaml", []byte("id: caso-b\nsteps:\n  - collect: [nome]\n")}
	substituir(t, p, a, b)
	if _, err := p.Substituir(t.Context(), []Arquivo{a}); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Substituir(t.Context(), nil); err != nil {
		t.Fatal(err)
	}
	arquivos, err := p.Listar(t.Context())
	if err != nil || len(arquivos) != 2 {
		t.Fatalf("snapshot parcial removeu outro caso: %v %v", arquivos, err)
	}
	m, err := p.Substituir(t.Context(), nil, a.Caminho)
	if err != nil || m.Removidos != 1 {
		t.Fatalf("remoção explícita sem uso: %+v %v", m, err)
	}
	arquivos, err = p.Listar(t.Context())
	if err != nil || len(arquivos) != 1 || arquivos[0].Caminho != b.Caminho {
		t.Fatalf("alvo externo removido: %v %v", arquivos, err)
	}
	rows, err := p.cli.QueryContext(t.Context(), `SELECT id FROM use_cases ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var id string
	if !rows.Next() {
		t.Fatal("caso b ausente")
	}
	if err := rows.Scan(&id); err != nil {
		t.Fatal(err)
	}
	if id != "caso-b" || rows.Next() {
		t.Fatalf("projeções incorretas: %s", id)
	}
}

func TestSubstituirRecusaCasoEmUsoEDesfazTodoOLote(t *testing.T) {
	p := novoStore(t)
	a := Arquivo{"agentes/a/caso.yaml", []byte("id: caso-a\nsteps:\n  - collect: [nome]\n")}
	b := Arquivo{"agentes/b/caso.yaml", []byte("id: caso-b\nsteps:\n  - collect: [nome]\n")}
	substituir(t, p, a, b)
	if _, err := p.cli.ExecContext(t.Context(), `INSERT INTO company_use_cases(id,company_id,use_case_id,enabled,default_available,updated_at) VALUES('00000000-0000-4000-8000-000000000001','outra-empresa','caso-b',true,false,now())`); err != nil {
		t.Fatal(err)
	}
	_, err := p.Substituir(t.Context(), []Arquivo{{"agentes/novo.md", []byte("novo")}}, a.Caminho, b.Caminho)
	if err == nil {
		t.Fatal("remoção de caso configurado aceita")
	}
	arquivos, err := p.Listar(t.Context())
	if err != nil || len(arquivos) != 2 {
		t.Fatalf("lote parcialmente aplicado: %v %v", arquivos, err)
	}
	rows, err := p.cli.QueryContext(t.Context(), `SELECT count(*) FROM use_cases`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var total int
	if !rows.Next() {
		t.Fatal("contagem ausente")
	}
	if err := rows.Scan(&total); err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Fatalf("rollback perdeu projeção: %d", total)
	}
}
