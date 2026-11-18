package memoria

import (
	"testing"
	"time"

	"go-concierge/internal/banco/bancotest"
	"go-concierge/internal/escopo"
)

func TestV2MemoriaPreservaRecortes(t *testing.T) {
	for _, implementation := range []string{"fake", "postgres", "cache"} {
		t.Run(implementation, func(t *testing.T) {
			var store Store = NovoFake()
			if implementation == "postgres" {
				store = NovoPostgres(bancotest.Novo(t))
			}
			if implementation == "cache" {
				store = NovoCache(NovoPostgres(bancotest.Novo(t)))
			}
			alpha, beta := 101, 202
			contact := func(company, phone string, condo *int) escopo.Contato {
				c, err := escopo.NovoContato(company, condo, phone)
				if err != nil {
					t.Fatal(err)
				}
				return c
			}
			amplo := contact("Nexus Company", "11999998888", nil)
			restrito := contact(amplo.CompanyID, amplo.Telefone, &alpha)
			terceiro := contact(amplo.CompanyID, "11888887777", nil)
			outra := contact("Outra empresa", amplo.Telefone, nil)
			fact := func(condo *int, text string) Fato {
				return Fato{CondoID: condo, Chave: "pets", Texto: text, Quando: time.Now().UTC().Truncate(time.Microsecond)}
			}
			if err := store.Gravar(t.Context(), amplo, Memoria{Fatos: []Fato{fact(nil, "sem condo"), fact(&alpha, "Alpha"), fact(&beta, "Beta")}}); err != nil {
				t.Fatal(err)
			}
			for _, c := range []escopo.Contato{terceiro, outra} {
				if err := store.Gravar(t.Context(), c, Memoria{Fatos: []Fato{fact(nil, "intacto")}}); err != nil {
					t.Fatal(err)
				}
			}
			assert := func(c escopo.Contato, want ...string) {
				t.Helper()
				m, err := store.Ler(t.Context(), c)
				if err != nil {
					t.Fatal(err)
				}
				if len(m.Fatos) != len(want) {
					t.Fatalf("recorte %v: fatos=%+v, want %v", c.CondoID, m.Fatos, want)
				}
				for _, text := range want {
					found := false
					for _, f := range m.Fatos {
						found = found || f.Texto == text
					}
					if !found {
						t.Fatalf("texto %q ausente em %+v", text, m.Fatos)
					}
				}
			}
			assert(restrito, "Alpha")
			assert(amplo, "sem condo", "Alpha", "Beta")
			if err := store.Gravar(t.Context(), restrito, Memoria{Fatos: []Fato{fact(&alpha, "Alpha alterado")}}); err != nil {
				t.Fatal(err)
			}
			assert(amplo, "sem condo", "Alpha alterado", "Beta")
			for _, forbidden := range []*int{nil, &beta} {
				if err := store.Gravar(t.Context(), restrito, Memoria{Fatos: []Fato{fact(forbidden, "inválido")}}); err == nil {
					t.Fatal("gravação fora do alvo aceita")
				}
			}
			if err := store.Gravar(t.Context(), restrito, Memoria{}); err != nil {
				t.Fatal(err)
			}
			assert(restrito)
			assert(amplo, "sem condo", "Beta")
			if err := store.Gravar(t.Context(), amplo, Memoria{}); err != nil {
				t.Fatal(err)
			}
			assert(amplo, "Beta")
			assert(terceiro, "intacto")
			assert(outra, "intacto")
			if err := store.Gravar(t.Context(), restrito, Memoria{Fatos: []Fato{fact(&alpha, "continuidade")}}); err != nil {
				t.Fatal(err)
			}
			assert(amplo, "Beta", "continuidade")
		})
	}
}

func TestV2FatosHomonimosCoexistemNoModelo(t *testing.T) {
	alpha, beta := 101, 202
	m, _ := Memoria{}.Upsert(Fato{CondoID: &alpha, Chave: "pets", Texto: "Alpha"})
	m, _ = m.Upsert(Fato{CondoID: &beta, Chave: "pets", Texto: "Beta"})
	m, _ = m.Upsert(Fato{Chave: "pets", Texto: "NULL"})
	if len(m.Fatos) != 3 {
		t.Fatalf("homônimos colidiram: %+v", m.Fatos)
	}
	m, mudou := m.Esquecer("pets", &alpha)
	if !mudou || len(m.Fatos) != 2 {
		t.Fatalf("Esquecer Alpha=%+v", m.Fatos)
	}
	for _, f := range m.Fatos {
		if f.CondoID != nil && *f.CondoID == alpha {
			t.Fatal("Alpha sobreviveu")
		}
	}
}
