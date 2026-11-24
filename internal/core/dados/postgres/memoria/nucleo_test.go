package memoria

import (
	"slices"
	"testing"
	"time"
)

var dia = time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)

func chaves(m Memoria) []string {
	out := []string{}
	for _, f := range m.Fatos {
		out = append(out, f.Chave+"="+f.Texto)
	}
	return out
}

func TestUpsertNaoAlteraOriginal(t *testing.T) {
	original := Memoria{Fatos: []Fato{{Chave: "pets", Texto: "tem um gato", Quando: dia}}}

	novo, mudou := original.Upsert(Fato{Chave: "pets", Texto: "tem um cachorro", Quando: dia.Add(time.Hour)})

	if !mudou || novo.Fatos[0].Texto != "tem um cachorro" {
		t.Fatalf("novo: %v %+v", mudou, novo.Fatos)
	}
	if original.Fatos[0].Texto != "tem um gato" {
		t.Fatalf("original alterado: %+v", original.Fatos)
	}
}

func TestUpsertMesmaChaveSubstitui(t *testing.T) {
	base := Memoria{Fatos: []Fato{{Chave: "unidades", Texto: "tem só uma unidade"}, {Chave: "pets", Texto: "tem um cachorro"}}}
	tests := []struct {
		name      string
		fato      Fato
		want      []string
		wantMudou bool
	}{
		{"chave normalizada substitui no lugar", Fato{Chave: "Unidades", Texto: "tem duas unidades"},
			[]string{"unidades=tem duas unidades", "pets=tem um cachorro"}, true},
		{"mesmo texto nao muda", Fato{Chave: "unidades", Texto: " tem só uma unidade "},
			[]string{"unidades=tem só uma unidade", "pets=tem um cachorro"}, false},
		{"chave nova entra no fim", Fato{Chave: "nome preferido", Texto: "Van"},
			[]string{"unidades=tem só uma unidade", "pets=tem um cachorro", "nome_preferido=Van"}, true},
		{"texto vazio ignora", Fato{Chave: "veiculo", Texto: "  "},
			[]string{"unidades=tem só uma unidade", "pets=tem um cachorro"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, mudou := base.Upsert(tt.fato)
			if mudou != tt.wantMudou || !slices.Equal(chaves(got), tt.want) {
				t.Fatalf("Upsert = %v %v, want %v %v", mudou, chaves(got), tt.wantMudou, tt.want)
			}
		})
	}
}

func TestEsquecer(t *testing.T) {
	base := Memoria{Fatos: []Fato{{Chave: "unidades", Texto: "uma"}, {Chave: "pets", Texto: "cachorro"}}}
	tests := []struct {
		name      string
		chave     string
		want      []string
		wantMudou bool
	}{
		{"chave existente sai", "Pets", []string{"unidades=uma"}, true},
		{"chave ausente nao muda", "veiculo", []string{"unidades=uma", "pets=cachorro"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, mudou := base.Esquecer(tt.chave)
			if mudou != tt.wantMudou || !slices.Equal(chaves(got), tt.want) {
				t.Fatalf("Esquecer = %v %v, want %v %v", mudou, chaves(got), tt.wantMudou, tt.want)
			}
			if len(base.Fatos) != 2 || base.Fatos[1].Chave != "pets" {
				t.Fatalf("original alterado: %+v", base.Fatos)
			}
		})
	}
}
