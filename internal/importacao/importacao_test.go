package importacao

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"go-concierge/internal/admin"
	"go-concierge/internal/banco/bancotest"
	"go-concierge/internal/escopo"
	"go-concierge/internal/memoria"
	"go-concierge/internal/pedidos"
)

func TestMain(m *testing.M) {
	codigo := m.Run()
	bancotest.Encerrar()
	os.Exit(codigo)
}

func destinosFake(*testing.T) Destinos {
	return Destinos{Mapeamento: admin.NovaEmMemoria(), Memorias: memoria.NovoFake(), Pedidos: pedidos.NovoFake()}
}

func destinosPostgres(t *testing.T) Destinos {
	db := bancotest.Novo(t)
	return Destinos{Mapeamento: admin.NovoPostgres(db.Ent), Memorias: memoria.NovoPostgres(db), Pedidos: pedidos.NovoPostgres(db)}
}

// emCadaDestino roda o cenário contra o Fake e contra o Postgres: o import tem de valer nos dois.
func emCadaDestino(t *testing.T, cenario func(t *testing.T, novo func(*testing.T) Destinos)) {
	t.Run("fake", func(t *testing.T) { cenario(t, destinosFake) })
	t.Run("postgres", func(t *testing.T) { cenario(t, destinosPostgres) })
}

// pastaComCopiaDoTestdata devolve uma pasta de trabalho para o teste alterar à vontade.
func pastaComCopiaDoTestdata(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS("testdata")); err != nil {
		t.Fatalf("copiar testdata: %v", err)
	}
	return dir
}

func escrever(t *testing.T, caminho, conteudo string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(caminho), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(caminho, []byte(conteudo), 0o644); err != nil {
		t.Fatal(err)
	}
}

func importar(t *testing.T, dir string, d Destinos) Relatorio {
	t.Helper()
	rel, err := Importar(t.Context(), dir, d)
	if err != nil {
		t.Fatalf("Importar: %v", err)
	}
	return rel
}

func TestImportarTestdata(t *testing.T) {
	emCadaDestino(t, func(t *testing.T, novo func(*testing.T) Destinos) {
		rel := importar(t, "testdata", novo(t))

		quer := Relatorio{
			AreaMapeamento: {Lidos: 6, Gravados: 6}, // 1 administradora, 1 condomínio, 2 habilitações, 1 orientação, 1 payload
			AreaMemorias:   {Lidos: 2, Gravados: 2},
			AreaPedidos:    {Lidos: 2, Gravados: 2},
		}
		if !reflect.DeepEqual(rel, quer) {
			t.Fatalf("relatório = %v, quer %v", rel, quer)
		}
		if !rel.Completo() {
			t.Error("Completo() = false com Lidos == Gravados")
		}
	})
}

func TestImportarTraduzAccountETelefone(t *testing.T) {
	emCadaDestino(t, func(t *testing.T, novo func(*testing.T) Destinos) {
		d := novo(t)
		importar(t, "testdata", d)

		ana, err := escopo.NovoContato("adm-a", "cond-001", "11999990001")
		if err != nil {
			t.Fatal(err)
		}
		m, err := d.Memorias.Ler(t.Context(), ana)
		if err != nil {
			t.Fatal(err)
		}
		if m.Nome != "Ana" || len(m.Fatos) != 2 {
			t.Errorf("memória da Ana = nome %q, %d fatos; quer Ana e 2 fatos", m.Nome, len(m.Fatos))
		}
		for _, f := range m.Fatos {
			if f.CondominioID != "cond-001" {
				t.Errorf("fato %s com condomínio %q, quer cond-001", f.Chave, f.CondominioID)
			}
		}
		cart, err := d.Pedidos.Ler(t.Context(), ana)
		if err != nil {
			t.Fatal(err)
		}
		if len(cart.Pedidos) != 2 || cart.Pedidos[0].Protocolo != "P-AAAAAAAA" {
			t.Fatalf("carteira da Ana = %d pedidos, quer 2 com o protocolo original", len(cart.Pedidos))
		}
		if cart.Pedidos[0].CondominioID != "cond-001" {
			t.Errorf("pedido com condomínio %q, quer cond-001", cart.Pedidos[0].CondominioID)
		}
	})
}

type fotografia struct {
	Mapeamento admin.Mapeamento
	Memorias   []memoria.Memoria
	Carteiras  []pedidos.Carteira
}

func fotografar(t *testing.T, d Destinos) fotografia {
	t.Helper()
	m, err := d.Mapeamento.Ler(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	m.AtualizadoEm = time.Time{} // o instante muda a cada gravação
	mems, err := d.Memorias.Listar(t.Context(), "adm-a")
	if err != nil {
		t.Fatal(err)
	}
	carts, err := d.Pedidos.Listar(t.Context(), "adm-a")
	if err != nil {
		t.Fatal(err)
	}
	return fotografia{Mapeamento: m, Memorias: mems, Carteiras: carts}
}

func TestImportarDuasVezesNaoDuplica(t *testing.T) {
	emCadaDestino(t, func(t *testing.T, novo func(*testing.T) Destinos) {
		d := novo(t)
		primeiro := importar(t, "testdata", d)
		antes := fotografar(t, d)

		segundo := importar(t, "testdata", d)
		depois := fotografar(t, d)

		if !reflect.DeepEqual(primeiro, segundo) {
			t.Errorf("segundo relatório = %v, quer o mesmo do primeiro %v", segundo, primeiro)
		}
		if len(depois.Memorias) != 2 || len(depois.Carteiras) != 1 || len(depois.Carteiras[0].Pedidos) != 2 {
			t.Fatalf("depois de importar 2x: %d memórias, %d carteiras; quer 2 e 1 (com 2 pedidos)", len(depois.Memorias), len(depois.Carteiras))
		}
		if !reflect.DeepEqual(antes.Mapeamento, depois.Mapeamento) {
			t.Errorf("mapeamento mudou ao reimportar:\nantes  %+v\ndepois %+v", antes.Mapeamento, depois.Mapeamento)
		}
		for i, p := range antes.Carteiras[0].Pedidos {
			if p.Protocolo != depois.Carteiras[0].Pedidos[i].Protocolo {
				t.Errorf("protocolo %d mudou ao reimportar", i)
			}
		}
	})
}

func TestImportarAccountOrfaListaENaoGravaAArea(t *testing.T) {
	emCadaDestino(t, func(t *testing.T, novo func(*testing.T) Destinos) {
		dir := pastaComCopiaDoTestdata(t)
		escrever(t, filepath.Join(dir, "memorias", "sem-dono", "11988880000.json"), `{"telefone":"11988880000","fatos":[]}`)
		escrever(t, filepath.Join(dir, "memorias", "outra-sem-dono", "11988880001.json"), `{"telefone":"11988880001","fatos":[]}`)
		d := novo(t)

		rel, err := Importar(t.Context(), dir, d)

		if err == nil {
			t.Fatal("Importar sem erro com accounts órfãs")
		}
		for _, orfa := range []string{"sem-dono", "outra-sem-dono"} {
			if !strings.Contains(err.Error(), orfa) {
				t.Errorf("erro %q sem a account órfã %q", err, orfa)
			}
		}
		if got := rel[AreaMemorias].Gravados; got != 0 {
			t.Errorf("memórias gravadas = %d, quer 0 com account órfã", got)
		}
		mems, listErr := d.Memorias.Listar(t.Context(), "adm-a")
		if listErr != nil || len(mems) != 0 {
			t.Errorf("memórias no destino = %d (%v), quer 0", len(mems), listErr)
		}
		if got := rel[AreaPedidos]; got != (Contagem{Lidos: 2, Gravados: 2}) {
			t.Errorf("pedidos = %+v: a área órfã não pode travar as outras", got)
		}
	})
}

func TestImportarErroNaoVazaTelefone(t *testing.T) {
	emCadaDestino(t, func(t *testing.T, novo func(*testing.T) Destinos) {
		dir := pastaComCopiaDoTestdata(t)
		escrever(t, filepath.Join(dir, "memorias", "cond-001", "11999990003.json"), `{quebrado`)

		rel, err := Importar(t.Context(), dir, novo(t))

		if err == nil {
			t.Fatal("Importar sem erro com JSON quebrado")
		}
		if strings.Contains(err.Error(), "999990003") {
			t.Errorf("erro com o telefone inteiro: %v", err)
		}
		if !strings.Contains(err.Error(), "5511****0003") {
			t.Errorf("erro %q sem o contato mascarado", err)
		}
		if rel[AreaMemorias].Gravados != 0 {
			t.Errorf("memórias gravadas = %d, quer 0: JSON quebrado derruba a área antes de gravar", rel[AreaMemorias].Gravados)
		}
	})
}

// ocuparProtocolo grava, em outra administradora, um pedido com o protocolo do testdata.
func ocuparProtocolo(t *testing.T, d Destinos, outraAdm string) {
	t.Helper()
	if _, err := d.Mapeamento.SalvarAdministradora(t.Context(), admin.Administradora{ID: outraAdm, Nome: "Outra"}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Mapeamento.SalvarCondominio(t.Context(), admin.Condominio{ID: "cond-b", Nome: "B", Administradora: outraAdm}); err != nil {
		t.Fatal(err)
	}
	outra, err := escopo.NovoContato(outraAdm, "cond-b", "11977770000")
	if err != nil {
		t.Fatal(err)
	}
	ocupa := &pedidos.Pedido{Protocolo: "P-AAAAAAAA", Caso: "x", Estado: "coletando"}
	if err := d.Pedidos.Gravar(t.Context(), outra, pedidos.Carteira{Pedidos: []*pedidos.Pedido{ocupa}}); err != nil {
		t.Fatal(err)
	}
}

func TestImportarRelataProtocoloRedesenhado(t *testing.T) {
	emCadaDestino(t, func(t *testing.T, novo func(*testing.T) Destinos) {
		d := novo(t)
		ocuparProtocolo(t, d, "adm-b")

		rel := importar(t, "testdata", d)

		if got := rel[AvisoProtocolosRedesenhados].Lidos; got != 1 {
			t.Errorf("aviso de protocolos redesenhados = %d, quer 1", got)
		}
		if c := rel[AreaPedidos]; c.Lidos != c.Gravados {
			t.Errorf("pedidos = %+v: o redesenho não pode falhar a importação", c)
		}
	})
}

func TestImportarPastaVaziaNaoGravaNada(t *testing.T) {
	rel := importar(t, t.TempDir(), destinosFake(t))
	if !rel.Completo() || rel[AreaMapeamento] != (Contagem{}) {
		t.Errorf("relatório de pasta vazia = %v", rel)
	}
}
