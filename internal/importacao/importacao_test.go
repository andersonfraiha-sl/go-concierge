package importacao

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"go-concierge/internal/admin"
	"go-concierge/internal/banco/bancotest"
	"go-concierge/internal/catalogo"
	"go-concierge/internal/escopo"
	"go-concierge/internal/memoria"
	"go-concierge/internal/pedidos"
)

func TestMain(m *testing.M) { codigo := m.Run(); bancotest.Encerrar(); os.Exit(codigo) }

func TestImportarRecusaLegadoAntesDeAcessarArquivosOuDestinos(t *testing.T) {
	quebrado := filepath.Join(t.TempDir(), "mapeamento.json")
	if err := os.WriteFile(quebrado, []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"testdata", t.TempDir(), filepath.Join(t.TempDir(), "inexistente"), filepath.Join(t.TempDir(), "5511999990001"), quebrado} {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			rel, err := Importar(t.Context(), dir, Destinos{Aviso: func(string) { t.Fatal("aviso emitido em importação recusada") }})
			if err == nil || !strings.Contains(err.Error(), "banco v2 inicia vazio") {
				t.Fatalf("recusa esperada antes do I/O: %v", err)
			}
			if len(rel) != 0 {
				t.Fatalf("relatório de importação recusada: %+v", rel)
			}
			if strings.Contains(err.Error(), dir) {
				t.Fatalf("caminho exposto na recusa: %v", err)
			}
		})
	}
}

func TestImportarRecusadoPreservaDadosExistentes(t *testing.T) {
	db := bancotest.Novo(t)
	if _, err := catalogo.NovoPostgres(db.Ent).Substituir(t.Context(), []catalogo.Arquivo{{Caminho: "agentes/atual/caso.yaml", Conteudo: []byte("id: caso-atual\nsteps:\n  - collect: [nome]\n")}}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Ent.ExecContext(t.Context(), `INSERT INTO company_use_cases(id,company_id,use_case_id,enabled,default_available,updated_at) VALUES('00000000-0000-4000-8000-000000000001','empresa-a','caso-atual',true,false,now())`); err != nil {
		t.Fatal(err)
	}
	destinos := []Destinos{
		{Mapeamento: admin.NovaEmMemoria(), Memorias: memoria.NovoFake(), Pedidos: pedidos.NovoFake()},
		{Mapeamento: admin.NovoPostgres(db.Ent), Memorias: memoria.NovoPostgres(db), Pedidos: pedidos.NovoPostgres(db)},
	}
	for i, d := range destinos {
		t.Run([]string{"fake", "postgres"}[i], func(t *testing.T) {
			contato, err := escopo.NovoContato("empresa-a", "101", "11999990001")
			if err != nil {
				t.Fatal(err)
			}
			alvo := admin.Alvo{Condominio: admin.Padrao}
			if _, err := d.Mapeamento.SalvarPayload(t.Context(), alvo, "recurso", map[string]any{"preservar": true}); err != nil {
				t.Fatal(err)
			}
			if err := d.Memorias.Gravar(t.Context(), contato, memoria.Memoria{Nome: "Atual", Fatos: []memoria.Fato{{Chave: "pets", Texto: "dois cães", CondoID: contato.CondoID, Quando: instante("2026-10-09")}}}); err != nil {
				t.Fatal(err)
			}
			if err := d.Pedidos.Gravar(t.Context(), contato, pedidos.Carteira{Pedidos: []*pedidos.Pedido{{Protocolo: "P-AAAAAAAA", Caso: "caso-atual", Estado: pedidos.Coletando, CondoID: contato.CondoID, CriadoEm: instante("2026-10-09"), AtualizadoEm: instante("2026-10-09")}}}); err != nil {
				t.Fatal(err)
			}
			antesM, err := d.Memorias.Ler(t.Context(), contato)
			if err != nil {
				t.Fatal(err)
			}
			antesP, err := d.Pedidos.Ler(t.Context(), contato)
			if err != nil {
				t.Fatal(err)
			}
			antesA, err := d.Mapeamento.Ler(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			for range 2 {
				if rel, err := Importar(t.Context(), "testdata", d); err == nil || len(rel) != 0 {
					t.Fatalf("importação aceita: %+v %v", rel, err)
				}
			}
			depoisM, err := d.Memorias.Ler(t.Context(), contato)
			if err != nil {
				t.Fatal(err)
			}
			depoisP, err := d.Pedidos.Ler(t.Context(), contato)
			if err != nil {
				t.Fatal(err)
			}
			depoisA, err := d.Mapeamento.Ler(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(antesM, depoisM) || !reflect.DeepEqual(antesP, depoisP) || !reflect.DeepEqual(antesA, depoisA) {
				t.Fatal("recusa alterou dados existentes")
			}
		})
	}
}
