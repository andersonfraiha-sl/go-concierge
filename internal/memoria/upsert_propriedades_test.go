package memoria

import (
	"testing"
	"time"

	"pgregory.net/rapid"
)

func gerarFato(t *rapid.T) Fato {
	chave := rapid.SampledFrom(ChavesDePreferencia()).Draw(t, "chave")
	return Fato{Chave: chave, Texto: rapid.SampledFrom(valoresValidos(chave)).Draw(t, "texto")}
}

// valoresValidos são valores aceitos por NormalizarPreferencia para a chave da lista.
func valoresValidos(chave string) []string {
	switch chave {
	case "tamanho_resposta":
		return []string{"curta", "detalhada"}
	case "tratamento":
		return []string{"informal", "formal"}
	case "formato_boleto":
		return []string{"linha_digitavel", "pix", "link"}
	}
	return []string{"Van", "Ana Paula", "Zé"}
}

// TestUpsertNuncaDuplicaChaveENuncaMutaOReceptor: para qualquer sequência de fatos
// (mesma chave reaparecendo ou não), Upsert nunca deixa duas linhas com a mesma chave e
// nunca altera a Memoria em que foi chamado — ela sempre devolve uma cópia.
func TestUpsertNuncaDuplicaChaveENuncaMutaOReceptor(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		m := Memoria{AdministradoraID: "adm-a", Telefone: "5519981621916"}
		antes := func() []Fato { c := make([]Fato, len(m.Fatos)); copy(c, m.Fatos); return c }

		n := rapid.IntRange(1, 12).Draw(t, "n")
		for i := 0; i < n; i++ {
			f := gerarFato(t)
			f.Quando = time.Now()
			snapshotAntes := antes()

			nova, _ := m.Upsert(f)

			if got := antes(); !slicesIguais(got, snapshotAntes) {
				t.Fatalf("Upsert alterou o receptor: %v -> %v", snapshotAntes, got)
			}
			vistos := map[string]bool{}
			for _, ft := range nova.Fatos {
				if vistos[ft.Chave] {
					t.Fatalf("chave %q duplicada em %v", ft.Chave, nova.Fatos)
				}
				vistos[ft.Chave] = true
			}
			m = nova
		}
	})
}

func slicesIguais(a, b []Fato) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Chave != b[i].Chave || a[i].Texto != b[i].Texto {
			return false
		}
	}
	return true
}
