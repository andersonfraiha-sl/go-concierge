package memoria

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go-concierge/internal/escopo"
)

func contato(t *testing.T, adm, tel string) escopo.Contato {
	t.Helper()
	c, err := escopo.NovoContato(adm, "cond-1", tel)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestStoreUpsertEsquecer(t *testing.T) {
	dir := t.TempDir()
	st := NewArquivos(dir)
	ctx := context.Background()
	c := contato(t, "adm-a", "+55 (19) 98162-1916")
	m, err := st.Ler(ctx, c)
	if err != nil || len(m.Fatos) != 0 || m.Telefone != "5519981621916" || m.AdministradoraID != "adm-a" {
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
	if err := st.Gravar(ctx, c, m); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "adm-a", "5519981621916.json")); err != nil {
		t.Fatalf("arquivo por administradora: %v", err)
	}
	m2, _ := st.Ler(ctx, contato(t, "adm-a", "19981621916"))
	if len(m2.Fatos) != 2 || !strings.Contains(m2.Texto(), "- unidades: tem duas unidades") {
		t.Fatalf("persistência: %+v %q", m2.Fatos, m2.Texto())
	}
	if !m2.Esquecer("pets") || len(m2.Fatos) != 1 {
		t.Fatal("esquecer")
	}
	l, err := st.Listar(ctx, "adm-a")
	if err != nil || len(l) != 1 {
		t.Fatalf("listar: %d %v", len(l), err)
	}
	outra, err := st.Ler(ctx, contato(t, "adm-b", "5519981621916"))
	if err != nil || len(outra.Fatos) != 0 {
		t.Fatalf("outra administradora não vê a memória: %+v %v", outra, err)
	}
	if l, err := st.Listar(ctx, "adm-b"); err != nil || len(l) != 0 {
		t.Fatalf("listar outra administradora: %d %v", len(l), err)
	}
	if _, err := st.Ler(ctx, escopo.Contato{}); err == nil {
		t.Fatal("contato vazio deveria falhar")
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

func TestErrosNaoExpoemOTelefone(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	st := NewArquivos(dir)
	c := contato(t, "adm-a", "5519981621916")
	if err := os.WriteFile(filepath.Join(dir, "adm-b"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "adm-a"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "adm-a", c.Telefone+".json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, errLer := st.Ler(ctx, c)
	errGravar := st.Gravar(ctx, contato(t, "adm-b", "5519981621916"), Memoria{})
	for _, err := range []error{errLer, errGravar} {
		if err == nil || strings.Contains(err.Error(), "981621916") {
			t.Fatalf("erro deveria existir e mascarar o telefone: %v", err)
		}
	}
}
