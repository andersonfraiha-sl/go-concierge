package memoria

import (
	"context"
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

func TestTextoListaUmFatoPorChave(t *testing.T) {
	m := Memoria{}
	m, _ = m.Upsert(Fato{Chave: "Unidades", Texto: "tem só uma unidade"})
	m, _ = m.Upsert(Fato{Chave: "pets", Texto: "tem um cachorro"})
	m, _ = m.Upsert(Fato{Chave: "unidades", Texto: "tem duas unidades"})
	if got, want := m.Texto(), "- unidades: tem duas unidades\n- pets: tem um cachorro"; got != want {
		t.Fatalf("Texto() = %q, want %q", got, want)
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
