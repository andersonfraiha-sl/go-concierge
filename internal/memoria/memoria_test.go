package memoria

import (
	"context"
	"strings"
	"testing"
)

func TestStoreUpsertEsquecer(t *testing.T) {
	st := NewArquivos(t.TempDir())
	ctx := context.Background()
	m, err := st.Ler(ctx, "cond-001", "+55 (19) 98162-1916")
	if err != nil || len(m.Fatos) != 0 || m.Telefone != "5519981621916" {
		t.Fatalf("%v %+v", err, m)
	}
	if !m.Upsert(Fato{Chave: "Unidades", Texto: "tem só uma unidade"}) || !m.Upsert(Fato{Chave: "pets", Texto: "tem um cachorro"}) {
		t.Fatal("upsert deveria mudar")
	}
	if m.Upsert(Fato{Chave: "unidades", Texto: "tem só uma unidade"}) {
		t.Fatal("mesmo texto não muda")
	}
	if !m.Upsert(Fato{Chave: "unidades", Texto: "tem duas unidades"}) || len(m.Fatos) != 2 || m.Fatos[0].Texto != "tem duas unidades" {
		t.Fatalf("substituir pela chave: %+v", m.Fatos)
	}
	if err := st.Gravar(ctx, m); err != nil {
		t.Fatal(err)
	}
	m2, _ := st.Ler(ctx, "cond-001", "5519981621916")
	if len(m2.Fatos) != 2 || !strings.Contains(m2.Texto(), "- unidades: tem duas unidades") {
		t.Fatalf("persistência: %+v %q", m2.Fatos, m2.Texto())
	}
	if !m2.Esquecer("pets") || len(m2.Fatos) != 1 {
		t.Fatal("esquecer")
	}
	l, _ := st.Listar(ctx, "cond-001")
	if len(l) != 1 {
		t.Fatalf("listar: %d", len(l))
	}
	if _, err := st.Ler(ctx, "cond-001", ""); err == nil {
		t.Fatal("telefone vazio deveria falhar")
	}
}

func TestRegras(t *testing.T) {
	fatos, _ := Regras{}.Extrair(context.Background(), "na verdade só tenho um apartamento, e tenho um cachorro. pode me chamar de Van", "", Memoria{})
	chaves := map[string]string{}
	for _, f := range fatos {
		chaves[f.Chave] = f.Texto
	}
	if chaves["unidades"] == "" || chaves["pets"] == "" || chaves["nome_preferido"] != "prefere ser chamado de Van" {
		t.Fatalf("%+v", chaves)
	}
	if f, _ := (Regras{}).Extrair(context.Background(), "até que horas a piscina abre?", "", Memoria{}); len(f) != 0 {
		t.Fatalf("pergunta não gera fato: %+v", f)
	}
}
