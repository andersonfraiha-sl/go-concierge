package memoria

import (
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
	m, _ = m.Upsert(Fato{Chave: "Tratamento", Texto: "informal"})
	m, _ = m.Upsert(Fato{Chave: "tamanho_resposta", Texto: "curta"})
	m, _ = m.Upsert(Fato{Chave: "tratamento", Texto: "formal"})
	if got, want := m.Texto(), "- tratamento: formal\n- tamanho_resposta: curta"; got != want {
		t.Fatalf("Texto() = %q, want %q", got, want)
	}
}
