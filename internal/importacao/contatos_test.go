package importacao

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go-concierge/internal/escopo"
	"go-concierge/internal/memoria"
)

// mapeamentoComDuasAccounts: a account cond-002 é de outro condomínio da mesma administradora.
const mapeamentoComDuasAccounts = `{
  "administradoras": [{"id": "adm-a", "nome": "Administradora A"}],
  "condominios": [
    {"id": "cond-001", "nome": "Condomínio Um", "administradora_id": "adm-a", "account": "cond-001"},
    {"id": "cond-002", "nome": "Condomínio Dois", "administradora_id": "adm-a", "account": "cond-002"}
  ]
}`

func pedidoJSON(protocolo, estado, atualizado string) string {
	return `{"protocolo": "` + protocolo + `", "caso": "x", "estado": "` + estado + `", "valores": {}, "origem": {},
	  "criado_em": "2026-10-01T10:00:00Z", "atualizado_em": "` + atualizado + `", "historico": []}`
}

func carteiraJSON(pedidos ...string) string {
	return `{"pedidos": [` + strings.Join(pedidos, ",") + `]}`
}

// pastaComContatosColidindo: o contato 5511999990001 de adm-a aparece em três arquivos.
func pastaComContatosColidindo(t *testing.T) string {
	t.Helper()
	dir := pastaComCopiaDoTestdata(t)
	escrever(t, filepath.Join(dir, "admin", "mapeamento.json"), mapeamentoComDuasAccounts)
	// mesmo telefone com DDI na mesma account
	escrever(t, filepath.Join(dir, "memorias", "cond-001", "5511999990001.json"), `{
	  "nome": "Ana Maria", "atualizada_em": "2026-10-05T10:00:00Z",
	  "fatos": [{"chave": "pets", "texto": "tem dois cachorros", "quando": "2026-10-05T10:00:00Z"},
	            {"chave": "carro", "texto": "fusca", "quando": "2026-10-05T10:00:00Z"}]}`)
	// mesmo telefone em outra account da mesma administradora
	escrever(t, filepath.Join(dir, "memorias", "cond-002", "11999990001.json"), `{
	  "atualizada_em": "2026-10-03T10:00:00Z",
	  "fatos": [{"chave": "horario", "texto": "manhã", "quando": "2026-10-03T10:00:00Z"}]}`)
	escrever(t, filepath.Join(dir, "pedidos", "cond-002", "11999990001.json"), carteiraJSON(
		pedidoJSON("P-CCCCCCCC", "coletando", "2026-10-03T10:00:00Z"),
		pedidoJSON("P-BBBBBBBB", "concluido", "2026-10-09T10:00:00Z"),
	))
	return dir
}

func TestImportarMesclaContatosQueColidem(t *testing.T) {
	emCadaDestino(t, func(t *testing.T, novo func(*testing.T) Destinos) {
		d := novo(t)

		rel := importar(t, pastaComContatosColidindo(t), d)

		quer := Relatorio{
			AreaMapeamento: {Lidos: 3, Gravados: 3},
			AreaMemorias:   {Lidos: 4, Gravados: 4},
			AreaPedidos:    {Lidos: 4, Gravados: 4}, // P-BBBBBBBB vem em dois arquivos
		}
		for area, c := range quer {
			if rel[area] != c {
				t.Errorf("%s = %+v, quer %+v", area, rel[area], c)
			}
		}
		ana, err := escopo.NovoContato("adm-a", "cond-001", "11999990001")
		if err != nil {
			t.Fatal(err)
		}
		m, err := d.Memorias.Ler(t.Context(), ana)
		if err != nil {
			t.Fatal(err)
		}
		textos := map[string]string{}
		for _, f := range m.Fatos {
			textos[f.Chave] = f.Texto
		}
		querFatos := map[string]string{"pets": "tem dois cachorros", "unidades": "apto 12", "carro": "fusca", "horario": "manhã"}
		for chave, texto := range querFatos {
			if textos[chave] != texto {
				t.Errorf("fato %s = %q, quer %q (nada pode se perder na mescla)", chave, textos[chave], texto)
			}
		}
		if m.Nome != "Ana Maria" {
			t.Errorf("nome = %q, quer o do arquivo mais recente (Ana Maria)", m.Nome)
		}
		cart, err := d.Pedidos.Ler(t.Context(), ana)
		if err != nil {
			t.Fatal(err)
		}
		estados := map[string]string{}
		for _, p := range cart.Pedidos {
			estados[p.Protocolo] = p.Estado
		}
		querPedidos := map[string]string{"P-AAAAAAAA": "concluido", "P-BBBBBBBB": "concluido", "P-CCCCCCCC": "coletando"}
		if len(estados) != len(querPedidos) {
			t.Fatalf("pedidos = %v, quer %v", estados, querPedidos)
		}
		for protocolo, estado := range querPedidos {
			if estados[protocolo] != estado {
				t.Errorf("pedido %s = %q, quer %q (o atualizado por último vence)", protocolo, estados[protocolo], estado)
			}
		}
	})
}

func TestMesclarMemoriasFatoMaisRecenteVenceENomeDoMaisNovo(t *testing.T) {
	antigo := memoria.Memoria{Nome: "Velho", Fatos: []memoria.Fato{{Chave: "k", Texto: "novo", Quando: instante("2026-10-09")}}, AtualizadaEm: instante("2026-10-01")}
	recente := memoria.Memoria{Fatos: []memoria.Fato{{Chave: "k", Texto: "velho", Quando: instante("2026-10-02")}}, AtualizadaEm: instante("2026-10-08")}

	junta := mesclarMemorias([]memoria.Memoria{recente, antigo})

	if len(junta.Fatos) != 1 || junta.Fatos[0].Texto != "novo" {
		t.Errorf("fatos = %+v, quer o de Quando mais recente, independente do arquivo", junta.Fatos)
	}
	if junta.Nome != "Velho" {
		t.Errorf("nome = %q, quer o do mais recente que tem nome", junta.Nome)
	}
}

func TestImportarAccountAmbiguaEErro(t *testing.T) {
	emCadaDestino(t, func(t *testing.T, novo func(*testing.T) Destinos) {
		dir := pastaComCopiaDoTestdata(t)
		escrever(t, filepath.Join(dir, "admin", "mapeamento.json"), `{
		  "administradoras": [{"id": "adm-a", "nome": "A"}, {"id": "adm-b", "nome": "B"}],
		  "condominios": [
		    {"id": "cond-001", "nome": "Um", "administradora_id": "adm-a", "account": "cond-001"},
		    {"id": "cond-009", "nome": "Nove", "administradora_id": "adm-b", "account": "cond-001"}]}`)

		rel, err := Importar(t.Context(), dir, novo(t))

		if err == nil {
			t.Fatal("Importar sem erro com account em duas administradoras")
		}
		for _, trecho := range []string{"cond-001", "adm-a", "adm-b", "mais de uma administradora"} {
			if !strings.Contains(err.Error(), trecho) {
				t.Errorf("erro %q sem %q", err, trecho)
			}
		}
		if rel[AreaMemorias].Gravados != 0 || rel[AreaPedidos].Gravados != 0 {
			t.Errorf("memórias %+v e pedidos %+v: account ambígua não grava nada", rel[AreaMemorias], rel[AreaPedidos])
		}
		if rel[AreaMapeamento].Gravados != 4 {
			t.Errorf("mapeamento = %+v: ele é válido e entra", rel[AreaMapeamento])
		}
	})
}

func TestImportarCondominioSemAdministradoraNaoEOrfa(t *testing.T) {
	dir := pastaComCopiaDoTestdata(t)
	escrever(t, filepath.Join(dir, "admin", "mapeamento.json"), `{
	  "condominios": [{"id": "cond-001", "nome": "Um", "account": "cond-001"}]}`)

	_, err := Importar(t.Context(), dir, destinosFake(t))

	if err == nil || !strings.Contains(err.Error(), "condomínio sem administradora (cond-001)") {
		t.Fatalf("erro = %v, quer apontar o condomínio sem administradora", err)
	}
	if strings.Contains(err.Error(), "sem condomínio no mapeamento") {
		t.Errorf("erro %q trata como órfã uma account que tem condomínio", err)
	}
}

func TestImportarRecusaNomeDeArquivoQueNaoETelefone(t *testing.T) {
	emCadaDestino(t, func(t *testing.T, novo func(*testing.T) Destinos) {
		dir := pastaComCopiaDoTestdata(t)
		escrever(t, filepath.Join(dir, "memorias", "cond-001", "1199999.json"), `{"fatos":[]}`)
		escrever(t, filepath.Join(dir, "memorias", "cond-001", "rascunho.json"), `{"fatos":[]}`)
		d := novo(t)

		rel, err := Importar(t.Context(), dir, d)

		if err == nil || !strings.Contains(err.Error(), "sem telefone válido") {
			t.Fatalf("erro = %v, quer recusar os nomes que não são telefone", err)
		}
		if strings.Contains(err.Error(), "1199999") || strings.Contains(err.Error(), "rascunho") {
			t.Errorf("erro %q repete o nome do arquivo", err)
		}
		if rel[AreaMemorias].Gravados != 0 {
			t.Errorf("memórias gravadas = %d, quer 0", rel[AreaMemorias].Gravados)
		}
	})
}

// semGuardar aceita a gravação e não guarda nada: a Store que perde dados em silêncio.
type semGuardar struct{ memoria.Store }

func (semGuardar) Gravar(context.Context, escopo.Contato, memoria.Memoria) error { return nil }

func TestImportarContaOQueAStoreGuardouDeVerdade(t *testing.T) {
	d := destinosFake(t)
	d.Memorias = semGuardar{d.Memorias}

	rel, err := Importar(t.Context(), "testdata", d)

	if err != nil {
		t.Fatalf("Importar: %v", err)
	}
	if got := rel[AreaMemorias]; got != (Contagem{Lidos: 2, Gravados: 0}) {
		t.Errorf("memórias = %+v, quer lidos=2 gravados=0", got)
	}
	if rel.Completo() {
		t.Error("Completo() = true com uma Store que não guardou")
	}
}

func TestRelatorioCompleto(t *testing.T) {
	tests := []struct {
		nome string
		rel  Relatorio
		quer bool
	}{
		{"vazio", Relatorio{}, true},
		{"tudo gravado", Relatorio{AreaMapeamento: {3, 3}, AreaMemorias: {2, 2}, AreaPedidos: {1, 1}}, true},
		{"memória a menos", Relatorio{AreaMapeamento: {3, 3}, AreaMemorias: {2, 1}}, false},
		{"pedido a menos", Relatorio{AreaPedidos: {4, 3}}, false},
		{"aviso não conta", Relatorio{AvisoProtocolosRedesenhados: {2, 2}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.nome, func(t *testing.T) {
			if got := tt.rel.Completo(); got != tt.quer {
				t.Errorf("Completo() = %v, quer %v", got, tt.quer)
			}
		})
	}
}

func TestImportarAvisaCadaProtocoloRedesenhado(t *testing.T) {
	d := destinosFake(t)
	var avisos []string
	d.Aviso = func(a string) { avisos = append(avisos, a) }
	ocuparProtocolo(t, d, "adm-b")

	importar(t, "testdata", d)

	if len(avisos) != 1 || !strings.Contains(avisos[0], "P-AAAAAAAA → P-") || strings.Contains(avisos[0], "11999990001") {
		t.Errorf("avisos = %q, quer um \"P-AAAAAAAA → P-NOVO\" com o contato mascarado", avisos)
	}
}

func instante(dia string) time.Time {
	t, err := time.Parse(time.DateOnly, dia)
	if err != nil {
		panic(err)
	}
	return t
}
