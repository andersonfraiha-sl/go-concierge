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
	original := Memoria{Fatos: []Fato{{Chave: "tamanho_resposta", Texto: "curta", Quando: dia}}}

	novo, mudou := original.Upsert(Fato{Chave: "tamanho_resposta", Texto: "detalhada", Quando: dia.Add(time.Hour)})

	if !mudou || novo.Fatos[0].Texto != "detalhada" {
		t.Fatalf("novo: %v %+v", mudou, novo.Fatos)
	}
	if original.Fatos[0].Texto != "curta" {
		t.Fatalf("original alterado: %+v", original.Fatos)
	}
}

func TestUpsertMesmaChaveSubstitui(t *testing.T) {
	base := Memoria{Fatos: []Fato{{Chave: "tamanho_resposta", Texto: "curta"}, {Chave: "tratamento", Texto: "informal"}}}
	tests := []struct {
		name      string
		fato      Fato
		want      []string
		wantMudou bool
	}{
		{"chave normalizada substitui no lugar", Fato{Chave: "Tamanho Resposta", Texto: "Detalhada"},
			[]string{"tamanho_resposta=detalhada", "tratamento=informal"}, true},
		{"mesmo texto nao muda", Fato{Chave: "tamanho_resposta", Texto: " curta "},
			[]string{"tamanho_resposta=curta", "tratamento=informal"}, false},
		{"chave nova entra no fim", Fato{Chave: "nome preferido", Texto: "Van"},
			[]string{"tamanho_resposta=curta", "tratamento=informal", "nome_preferido=Van"}, true},
		{"texto vazio ignora", Fato{Chave: "formato_boleto", Texto: "  "},
			[]string{"tamanho_resposta=curta", "tratamento=informal"}, false},
		{"chave fora da lista ignora", Fato{Chave: "pets", Texto: "um cachorro"},
			[]string{"tamanho_resposta=curta", "tratamento=informal"}, false},
		{"valor fora da lista ignora", Fato{Chave: "tratamento", Texto: "informal e formal"},
			[]string{"tamanho_resposta=curta", "tratamento=informal"}, false},
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

func TestUpsertRenovaDataDoMesmoValor(t *testing.T) {
	base := Memoria{Fatos: []Fato{{Chave: "tratamento", Texto: "formal", Quando: dia}}}
	tests := []struct {
		name      string
		quando    time.Time
		wantMudou bool
		wantData  time.Time
	}{
		{"data mais nova renova", dia.Add(48 * time.Hour), true, dia.Add(48 * time.Hour)},
		{"mesma data nao muda", dia, false, dia},
		{"data mais velha nao muda", dia.Add(-time.Hour), false, dia},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, mudou := base.Upsert(Fato{Chave: "tratamento", Texto: "formal", Quando: tt.quando})

			if mudou != tt.wantMudou || !got.Fatos[0].Quando.Equal(tt.wantData) {
				t.Fatalf("Upsert = %v %v, want %v %v", mudou, got.Fatos[0].Quando, tt.wantMudou, tt.wantData)
			}
		})
	}
}

func TestEsquecer(t *testing.T) {
	base := Memoria{Fatos: []Fato{{Chave: "tratamento", Texto: "formal"}, {Chave: "tamanho_resposta", Texto: "curta"}}}
	tests := []struct {
		name      string
		chave     string
		want      []string
		wantMudou bool
	}{
		{"chave existente sai", "Tamanho_Resposta", []string{"tratamento=formal"}, true},
		{"chave ausente nao muda", "veiculo", []string{"tratamento=formal", "tamanho_resposta=curta"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, mudou := base.Esquecer(tt.chave)
			if mudou != tt.wantMudou || !slices.Equal(chaves(got), tt.want) {
				t.Fatalf("Esquecer = %v %v, want %v %v", mudou, chaves(got), tt.wantMudou, tt.want)
			}
			if len(base.Fatos) != 2 || base.Fatos[1].Chave != "tamanho_resposta" {
				t.Fatalf("original alterado: %+v", base.Fatos)
			}
		})
	}
}
