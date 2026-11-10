package memoria

import (
	"slices"
	"strings"
	"testing"
	"time"
)

func TestNormalizarPreferencia(t *testing.T) {
	tests := []struct {
		name      string
		chave     string
		texto     string
		wantTexto string
		wantOK    bool
	}{
		{"tamanho com maiuscula", "tamanho_resposta", "Curta", "curta", true},
		{"tamanho com dois valores", "tamanho_resposta", "curta, mas detalhada", "", false},
		{"tratamento formal", "tratamento", "formal", "formal", true},
		{"formato com acento e espaco", "formato_boleto", "Linha Digitável", "linha_digitavel", true},
		{"formato fora da lista", "formato_boleto", "carta", "", false},
		{"nome simples", "nome_preferido", "Van", "Van", true},
		{"nome composto", "nome_preferido", "Ana Paula", "Ana Paula", true},
		{"nome com espacos sobrando", "nome_preferido", "  Ana   Paula ", "Ana Paula", true},
		{"nome com acento", "nome_preferido", "Zé", "Zé", true},
		{"nome em frase", "nome_preferido", "prefere ser chamado de Van", "", false},
		{"nome com digito", "nome_preferido", "V4n", "", false},
		{"nome longo demais", "nome_preferido", strings.Repeat("a", 31), "", false},
		{"nome no limite", "nome_preferido", strings.Repeat("a", 30), strings.Repeat("a", 30), true},
		{"nome vazio", "nome_preferido", "  ", "", false},
		{"chave livre pets", "pets", "um cachorro", "", false},
		{"chave livre unidades", "unidades", "uma", "", false},
		{"chave da lista com caixa e espaco", "Nome Preferido", "Van", "Van", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := NormalizarPreferencia(Fato{Chave: tt.chave, Texto: tt.texto})

			if ok != tt.wantOK || got.Texto != tt.wantTexto {
				t.Fatalf("NormalizarPreferencia(%q, %q) = %q %v, want %q %v", tt.chave, tt.texto, got.Texto, ok, tt.wantTexto, tt.wantOK)
			}
		})
	}
}

func TestNormalizarPreferenciaPreservaOsMetadados(t *testing.T) {
	f := Fato{Chave: "tratamento", Texto: "Formal", Fonte: "P-1", CondominioID: "cond-1", Quando: dia, Confianca: 0.9}

	got, ok := NormalizarPreferencia(f)

	f.Texto = "formal"
	if !ok || got != f {
		t.Fatalf("NormalizarPreferencia = %+v %v, want %+v", got, ok, f)
	}
}

func TestChavesDePreferenciaVemOrdenadas(t *testing.T) {
	want := []string{"formato_boleto", "nome_preferido", "tamanho_resposta", "tratamento"}

	if got := ChavesDePreferencia(); !slices.Equal(got, want) {
		t.Fatalf("ChavesDePreferencia = %v, want %v", got, want)
	}
}

func TestSoPreferenciasDescartaFatoAntigo(t *testing.T) {
	fatos := []Fato{
		{Chave: "nome_preferido", Texto: "prefere ser chamado de Van"},
		{Chave: "pets", Texto: "tem um cachorro"},
		{Chave: "tratamento", Texto: "Formal"},
	}

	got := SoPreferencias(fatos)

	if len(got) != 1 || got[0].Chave != "tratamento" || got[0].Texto != "formal" {
		t.Fatalf("SoPreferencias = %+v", got)
	}
	if got := SoPreferencias(nil); got == nil {
		t.Fatal("SoPreferencias(nil) = nil, want fatia vazia")
	}
}

func TestPreferenciasVigentesDescartaMaisDe180Dias(t *testing.T) {
	validade := AjustesPadrao().ValidadePreferencia
	tests := []struct {
		name   string
		quando time.Time
		want   int
	}{
		{"181 dias vence", agoraDoTeste.Add(-181 * umDia), 0},
		{"179 dias fica", agoraDoTeste.Add(-179 * umDia), 1},
		{"sem data conta como vencido", time.Time{}, 0},
		{"data no futuro fica", agoraDoTeste.Add(time.Hour), 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fatos := []Fato{{Chave: "tratamento", Texto: "formal", Quando: tt.quando}}

			if got := PreferenciasVigentes(fatos, agoraDoTeste, validade); len(got) != tt.want {
				t.Fatalf("PreferenciasVigentes = %+v, want %d", got, tt.want)
			}
		})
	}
}

func TestRegrasExtraiSoDaLista(t *testing.T) {
	tests := []struct {
		name     string
		mensagem string
		want     []string
	}{
		{"nome ignora o resto", "pode me chamar de Van, tenho um cachorro", []string{"nome_preferido=Van"}},
		{"resposta curta", "pode ser mais curto", []string{"tamanho_resposta=curta"}},
		{"resposta curta sem mais", "pode ser direto", []string{"tamanho_resposta=curta"}},
		{"resposta detalhada", "quero com detalhes", []string{"tamanho_resposta=detalhada"}},
		{"unidade e pets nao sao preferencia", "só tenho um apartamento e tenho um cachorro", []string{}},
		{"pergunta nao gera fato", "até que horas a piscina abre?", []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fatos, err := Regras{}.Extrair(t.Context(), tt.mensagem, "", Memoria{})

			got := chaves(Memoria{Fatos: fatos})
			if err != nil || !slices.Equal(got, tt.want) {
				t.Fatalf("Extrair = %v %v, want %v", got, err, tt.want)
			}
		})
	}
}
