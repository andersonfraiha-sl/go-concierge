package pii

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math/rand/v2"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
)

func TestMascarar(t *testing.T) {
	tests := []struct {
		name    string
		texto   string
		proximo int
		want    string
		achados map[string]string
	}{
		{"cpf pontuado", "meu cpf é 123.456.789-09", 1, "meu cpf é «cpf:1»", map[string]string{"«cpf:1»": "123.456.789-09"}},
		{"cpf só dígitos", "12345678909", 1, "«cpf:1»", map[string]string{"«cpf:1»": "12345678909"}},
		{"cpf sem hífen", "cpf 123.456.78909.", 1, "cpf «cpf:1».", map[string]string{"«cpf:1»": "123.456.78909"}},
		{"cpf numera a partir do próximo", "cpf 529.982.247-25", 3, "cpf «cpf:3»", map[string]string{"«cpf:3»": "529.982.247-25"}},
		{"celular com +55 e parênteses", "liga +55 (19) 94040-2710", 1, "liga «tel:1»", map[string]string{"«tel:1»": "+55 (19) 94040-2710"}},
		{"celular sem +55", "(19) 94040-2710", 1, "«tel:1»", map[string]string{"«tel:1»": "(19) 94040-2710"}},
		{"celular sem o nono dígito", "19 4040-2710", 1, "«tel:1»", map[string]string{"«tel:1»": "19 4040-2710"}},
		{"celular só dígitos com 55", "5519940402710", 1, "«tel:1»", map[string]string{"«tel:1»": "5519940402710"}},
		{"celular só dígitos sem 55", "zap 19940402710!", 1, "zap «tel:1»!", map[string]string{"«tel:1»": "19940402710"}},
		{"celular com espaço", "11 98765 4321", 1, "«tel:1»", map[string]string{"«tel:1»": "11 98765 4321"}},
		{"celular +55 colado", "+5511987654321", 1, "«tel:1»", map[string]string{"«tel:1»": "+5511987654321"}},
		{"fixo com parênteses", "portaria (11) 3333-4444", 1, "portaria «tel:1»", map[string]string{"«tel:1»": "(11) 3333-4444"}},
		{"fixo com +55 e hífen", "+55-11-3333-4444", 1, "«tel:1»", map[string]string{"«tel:1»": "+55-11-3333-4444"}},
		{
			"dois valores numerados na ordem do texto",
			"tel (19) 94040-2710 e cpf 123.456.789-09",
			1,
			"tel «tel:1» e cpf «cpf:2»",
			map[string]string{"«tel:1»": "(19) 94040-2710", "«cpf:2»": "123.456.789-09"},
		},
		{
			"mesmo valor repetido usa o mesmo marcador",
			"12345678909 de novo 12345678909",
			1,
			"«cpf:1» de novo «cpf:1»",
			map[string]string{"«cpf:1»": "12345678909"},
		},
		// Formatos do dia a dia (review I1). O texto em volta não muda.
		{"celular com o 9 separado", "whats: 19 9 8162-1916", 1, "whats: «tel:1»", map[string]string{"«tel:1»": "19 9 8162-1916"}},
		{"celular com parênteses e o 9 separado", "(19) 9 8162-1916.", 1, "«tel:1».", map[string]string{"«tel:1»": "(19) 9 8162-1916"}},
		{"DDD com zero e parênteses", "(019) 98162-1916", 1, "«tel:1»", map[string]string{"«tel:1»": "(019) 98162-1916"}},
		{"DDD com zero", "019 98162-1916", 1, "«tel:1»", map[string]string{"«tel:1»": "019 98162-1916"}},
		{"telefone com ponto", "19.98162.1916", 1, "«tel:1»", map[string]string{"«tel:1»": "19.98162.1916"}},
		{"celular com DDD sem o 9", "19 8162-1916", 1, "«tel:1»", map[string]string{"«tel:1»": "19 8162-1916"}},
		{"telefone colado nos parênteses", "(19)98162-1916", 1, "«tel:1»", map[string]string{"«tel:1»": "(19)98162-1916"}},
		{"telefone com NBSP", "tel 19\u00A098162-1916 ok", 1, "tel «tel:1» ok", map[string]string{"«tel:1»": "19\u00A098162-1916"}},
		{"telefone com espaço estreito", "+55\u202F19\u202F98162-1916", 1, "«tel:1»", map[string]string{"«tel:1»": "+55\u202F19\u202F98162-1916"}},
		{"telefone com hífen não separável", "19 98162\u20111916", 1, "«tel:1»", map[string]string{"«tel:1»": "19 98162\u20111916"}},
		{"telefone com travessão", "19 98162\u20141916", 1, "«tel:1»", map[string]string{"«tel:1»": "19 98162\u20141916"}},
		{"cpf com espaço antes do DV", "123456789 09", 1, "«cpf:1»", map[string]string{"«cpf:1»": "123456789 09"}},
		{"cpf com espaços", "cpf 123 456 789 09!", 1, "cpf «cpf:1»!", map[string]string{"«cpf:1»": "123 456 789 09"}},
		{"cpf só com pontos", "123.456.789.09", 1, "«cpf:1»", map[string]string{"«cpf:1»": "123.456.789.09"}},
		{"cpf só com hífens", "123-456-789-09", 1, "«cpf:1»", map[string]string{"«cpf:1»": "123-456-789-09"}},
		{"cpf com barra", "123.456.789/09", 1, "«cpf:1»", map[string]string{"«cpf:1»": "123.456.789/09"}},
		{"cpf com largura zero", "x 123.456.789\u200B-09 y", 1, "x «cpf:1» y", map[string]string{"«cpf:1»": "123.456.789\u200B-09"}},
		{"cpf entremeado de largura zero", "123\u200B456\u2060789\uFEFF09", 1, "«cpf:1»", map[string]string{"«cpf:1»": "123\u200B456\u2060789\uFEFF09"}},
		{"cpf fullwidth", "cpf １２３４５６７８９０９ ok", 1, "cpf «cpf:1» ok", map[string]string{"«cpf:1»": "１２３４５６７８９０９"}},
		{"telefone fullwidth", "（１９）９８１６２－１９１６", 1, "«tel:1»", map[string]string{"«tel:1»": "（１９）９８１６２－１９１６"}},
		// Review Focus 2: o que parece CPF ou telefone mas não é continua intacto.
		{"dinheiro com pontos", "R$ 12.345.678,90 e R$ 1.234.567.890,00", 1, "R$ 12.345.678,90 e R$ 1.234.567.890,00", nil},
		{"protocolo agrupado", "protocolo 2026 1103 0001", 1, "protocolo 2026 1103 0001", nil},
		{"código de barras", "34191.79001 01043.510047 91020.150008 1 96610000012345", 1, "34191.79001 01043.510047 91020.150008 1 96610000012345", nil},
		{"número longo seguido de telefone", "pedido 123456789012345678 tel (19) 98162-1916", 1, "pedido 123456789012345678 tel «tel:1»", map[string]string{"«tel:1»": "(19) 98162-1916"}},
		{"cpf com DV inválido", "código 12345678900", 1, "código 12345678900", nil},
		{"cpf de dígitos repetidos", "11111111111", 1, "11111111111", nil},
		{"protocolo de 14 dígitos", "protocolo 20261103000123", 1, "protocolo 20261103000123", nil},
		{"cpf colado em mais dígitos", "123456789091", 1, "123456789091", nil},
		{"data", "dia 03/11/2026 às 14:30", 1, "dia 03/11/2026 às 14:30", nil},
		{"data ISO", "2026-11-03", 1, "2026-11-03", nil},
		{"dinheiro", "boleto de R$ 1.234.567,89 e R$ 350,00", 1, "boleto de R$ 1.234.567,89 e R$ 350,00", nil},
		{"unidade e bloco", "apto 1203 bloco B, unidade 45", 1, "apto 1203 bloco B, unidade 45", nil},
		{"código de retirada de 8 dígitos", "retirada 48213377", 1, "retirada 48213377", nil},
		{"telefone sem DDD", "ramal 4040-2710", 1, "ramal 4040-2710", nil},
		{"texto sem número", "bom dia, tudo bem?", 1, "bom dia, tudo bem?", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, achados := mascarar(tt.texto, tt.proximo)
			if got != tt.want {
				t.Errorf("mascarar(%q) = %q, want %q", tt.texto, got, tt.want)
			}
			if len(achados) != len(tt.achados) || (len(tt.achados) > 0 && !maps.Equal(achados, tt.achados)) {
				t.Errorf("mascarar(%q) achados = %v, want %v", tt.texto, achados, tt.achados)
			}
		})
	}
}

func TestNeutralizar(t *testing.T) {
	tests := []struct{ texto, want string }{
		{"«cpf:1»", `"cpf:1"`},
		{"meu tel é «tel:2», ok", `meu tel é "tel:2", ok`},
		{"««cpf:1»»", `""cpf:1""`},
		{"o «Regimento»", `o "Regimento"`},
		{"sem marcador", "sem marcador"},
	}
	for _, tt := range tests {
		if got := Neutralizar(tt.texto); got != tt.want || TemMarcador(got) {
			t.Errorf("Neutralizar(%q) = %q, want %q sem marcador", tt.texto, got, tt.want)
		}
	}
}

// Review Focus 1: o morador digita um marcador para pegar o valor de outro achado.
func TestMarcadorForjadoNaoCasaComOCofre(t *testing.T) {
	mascarado, achados := mascarar("«cpf:1» meu cpf é 123.456.789-09 «tel:2»", 1)
	if mascarado != `"cpf:1" meu cpf é «cpf:1» "tel:2"` {
		t.Fatalf("mascarado = %q", mascarado)
	}
	got := Devolver(mascarado, achados)
	if want := `"cpf:1" meu cpf é 123.456.789-09 "tel:2"`; got != want {
		t.Errorf("Devolver = %q, want %q", got, want)
	}
}

func TestDevolver(t *testing.T) {
	cofre := map[string]string{"«cpf:1»": "123.456.789-09", "«tel:2»": "(19) 94040-2710"}
	tests := []struct {
		name, texto, want string
	}{
		{"marcadores voltam ao valor", "cpf «cpf:1», tel «tel:2»", "cpf 123.456.789-09, tel (19) 94040-2710"},
		{"cpf fora do cofre sai mascarado", "o cpf 529.982.247-25 está ativo", "o cpf «cpf» está ativo"},
		{"cpf do cofre escrito por extenso fica", "cpf 12345678909 confirmado", "cpf 12345678909 confirmado"},
		{"marcador fora do cofre não vira valor", "cpf «cpf:9»", "cpf «cpf:9»"},
		{"telefone fora do cofre fica", "ligue (11) 3333-4444", "ligue (11) 3333-4444"},
		{"número que não é cpf fica", "protocolo 12345678900", "protocolo 12345678900"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Devolver(tt.texto, cofre); got != tt.want {
				t.Errorf("Devolver(%q) = %q, want %q", tt.texto, got, tt.want)
			}
		})
	}
}

func TestTemMarcador(t *testing.T) {
	tests := []struct {
		texto string
		want  bool
	}{
		{"cpf «cpf:1»", true},
		{"«tel:12»", true},
		{"cpf:1", false},
		{"«cpf»", false},
		{"«nome:1»", false},
	}
	for _, tt := range tests {
		if got := TemMarcador(tt.texto); got != tt.want {
			t.Errorf("TemMarcador(%q) = %v, want %v", tt.texto, got, tt.want)
		}
	}
}

func TestValorNaoAlteraOriginal(t *testing.T) {
	original := map[string]any{
		"documento": "123.456.789-09",
		"contatos":  []any{"(19) 94040-2710", map[string]any{"cpf": "529.982.247-25"}},
		"unidade":   1203,
		"ativo":     true,
	}
	copia := clonarFundo(original)

	mascarado, achados := mascararEmValor(original, 1)
	if !reflect.DeepEqual(original, copia) {
		t.Fatalf("MascararEmValor mudou o original: %v", original)
	}
	if len(achados) != 3 {
		t.Fatalf("achados = %v, want 3", achados)
	}
	if s := fmt.Sprint(mascarado); strings.Contains(s, "789-09") || strings.Contains(s, "2710") || strings.Contains(s, "247-25") {
		t.Fatalf("mascarado vazou valor: %s", s)
	}
	mascaradoCopia := clonarFundo(mascarado)

	devolvido := DevolverEmValor(mascarado, achados)
	if !reflect.DeepEqual(mascarado, mascaradoCopia) {
		t.Fatalf("DevolverEmValor mudou o original: %v", mascarado)
	}
	if !reflect.DeepEqual(devolvido, original) {
		t.Errorf("DevolverEmValor = %v, want %v", devolvido, original)
	}
}

func TestMascararEmValorNumeraNaOrdemDasChaves(t *testing.T) {
	v := map[string]any{"b": "12345678909", "a": "(19) 94040-2710"}
	got, achados := mascararEmValor(v, 1)
	want := map[string]any{"a": "«tel:1»", "b": "«cpf:2»"}
	if !reflect.DeepEqual(got, want) || len(achados) != 2 {
		t.Errorf("MascararEmValor = %v (%v), want %v", got, achados, want)
	}
}

func clonarFundo(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, item := range x {
			out[k] = clonarFundo(item)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = clonarFundo(item)
		}
		return out
	default:
		return v
	}
}

// TestIdaEVolta é a propriedade principal: mascarar e devolver dá o texto de volta
// (sem marcador digitado), e nenhum CPF válido nem telefone sobra no mascarado.
func TestIdaEVolta(t *testing.T) {
	r := rand.New(rand.NewPCG(2026, 11))
	for i := range 20000 {
		texto := textoAleatorio(r)
		proximo := 1 + r.IntN(5)
		mascarado, achados := mascarar(texto, proximo)
		if trechos := achar(mascarado); len(trechos) > 0 {
			t.Fatalf("caso %d: %q sobrou em %q", i, mascarado[trechos[0].ini:trechos[0].fim], mascarado)
		}
		if got, want := Devolver(mascarado, achados), Neutralizar(texto); got != want {
			t.Fatalf("caso %d: Devolver(mascarar(%q)) = %q, want %q", i, texto, got, want)
		}
	}
}

func textoAleatorio(r *rand.Rand) string {
	pedacos := []func(*rand.Rand) string{
		cpfAleatorio, telefoneAleatorio,
		func(*rand.Rand) string { return " " },
		func(*rand.Rand) string { return "apto 1203" },
		func(*rand.Rand) string { return "protocolo 20261103000123" },
		func(*rand.Rand) string { return "R$ 1.234,56" },
		func(*rand.Rand) string { return "03/11/2026" },
		func(*rand.Rand) string { return "«cpf:1»" },
		func(r *rand.Rand) string { return fmt.Sprint(r.IntN(1_000_000)) },
		func(*rand.Rand) string { return "olá, " },
		func(r *rand.Rand) string { return disfarcar(r, cpfAleatorio(r)) },
		func(r *rand.Rand) string { return disfarcar(r, telefoneAleatorio(r)) },
		func(*rand.Rand) string { return "(19) 9 8162-1916" },
		func(*rand.Rand) string { return "R$ 12.345.678,90" },
		func(*rand.Rand) string { return "34191.79001 01043.510047" },
	}
	var b strings.Builder
	for range 1 + r.IntN(8) {
		b.WriteString(pedacos[r.IntN(len(pedacos))](r))
		b.WriteString([]string{" ", ", ", "\n", ""}[r.IntN(4)])
	}
	return b.String()
}

// disfarcar troca separadores por variantes Unicode, põe largura zero entre os
// dígitos ou passa os dígitos para largura cheia: o que o modelo lê como o mesmo número.
func disfarcar(r *rand.Rand, numero string) string {
	switch r.IntN(3) {
	case 0:
		return strings.NewReplacer(" ", "\u00A0", "-", "\u2011").Replace(numero)
	case 1:
		return strings.Join(strings.Split(numero, ""), "\u200B")
	default:
		return strings.Map(func(c rune) rune {
			if c >= '0' && c <= '9' {
				return c - '0' + '０'
			}
			return c
		}, numero)
	}
}

func cpfAleatorio(r *rand.Rand) string {
	d := make([]int, 9, 11)
	for i := range d {
		d[i] = r.IntN(10)
	}
	d = append(d, digitoVerificador(d))
	d = append(d, digitoVerificador(d))
	s := ""
	for _, x := range d {
		s += fmt.Sprint(x)
	}
	if r.IntN(2) == 0 {
		return s
	}
	return s[:3] + "." + s[3:6] + "." + s[6:9] + "-" + s[9:]
}

func telefoneAleatorio(r *rand.Rand) string {
	ddd := fmt.Sprint(11 + r.IntN(89))
	numero := fmt.Sprintf("9%04d-%04d", r.IntN(10_000), r.IntN(10_000))
	if r.IntN(2) == 0 {
		numero = fmt.Sprintf("3%03d-%04d", r.IntN(1000), r.IntN(10_000))
	}
	formatos := []string{"+55 (%s) %s", "(%s) %s", "%s %s", "55%s%s"}
	f := formatos[r.IntN(len(formatos))]
	if f == "55%s%s" {
		numero = strings.ReplaceAll(numero, "-", "")
	}
	return fmt.Sprintf(f, ddd, numero)
}

func FuzzMascarar(f *testing.F) {
	for _, s := range []string{
		"123.456.789-09", "12345678909", "+55 (19) 94040-2710", "«cpf:1» 12345678909",
		"12345678900", "20261103000123", "(11) 3333-4444 e 529.982.247-25", "««tel:1»»",
		"19\u00A098162\u20111916", "１２３４５６７８９０９", "123\u200B456\u200B789-09", "(019) 9 8162-1916",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, texto string) {
		mascarado, achados := mascarar(texto, 1)
		if trechos := achar(mascarado); len(trechos) > 0 {
			t.Fatalf("%q sobrou em %q", mascarado[trechos[0].ini:trechos[0].fim], mascarado)
		}
		if got, want := Devolver(mascarado, achados), Neutralizar(texto); got != want {
			t.Fatalf("Devolver(mascarar(%q)) = %q, want %q", texto, got, want)
		}
		// Sem cofre, nenhum marcador vira valor e nenhum CPF sai: os marcadores ficam
		// byte a byte e o que sobra de CPF já estava no texto mascarado.
		semCofre := Devolver(texto, nil)
		if got, want := reMarcador.FindAllString(semCofre, -1), reMarcador.FindAllString(texto, -1); !slices.Equal(got, want) {
			t.Fatalf("Devolver sem cofre mudou marcadores: %q -> %q", texto, semCofre)
		}
		if cpfs := slices.DeleteFunc(achar(semCofre), func(t trecho) bool { return t.tipo != tipoCPF }); len(cpfs) > 0 {
			t.Fatalf("Devolver sem cofre deixou CPF em %q", semCofre)
		}
	})
}

// C1: resultados de ferramenta mascarados em paralelo nunca dividem um marcador, e
// cada marcador volta ao valor do próprio resultado.
func TestCofreConcorrente(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	cpfs := make([]string, 64)
	for i := range cpfs {
		cpfs[i] = cpfAleatorio(r)
	}
	c := NovoCofre()
	mascarados := make([]any, len(cpfs))
	var wg sync.WaitGroup
	for i, cpf := range cpfs {
		wg.Go(func() {
			mascarados[i] = c.MascararEmValor(map[string]any{"morador": map[string]any{"cpf": cpf}})
		})
	}
	wg.Wait()

	cofre := c.Ler()
	vistos := map[string]bool{}
	for i, m := range mascarados {
		marcador := m.(map[string]any)["morador"].(map[string]any)["cpf"].(string)
		if vistos[marcador] {
			t.Fatalf("marcador %s repetido", marcador)
		}
		vistos[marcador] = true
		if got := Devolver(marcador, cofre); got != cpfs[i] {
			t.Errorf("Devolver(%s) = %q, want %q", marcador, got, cpfs[i])
		}
	}
	if len(cofre) != len(cpfs) {
		t.Errorf("len(cofre) = %d, want %d", len(cofre), len(cpfs))
	}
}

func TestCofreMascararNumeraEmSequencia(t *testing.T) {
	c := NovoCofre()
	if got := c.Mascarar("cpf 123.456.789-09"); got != "cpf «cpf:1»" {
		t.Fatalf("1º Mascarar = %q", got)
	}
	if got := c.Mascarar("tel (19) 98162-1916 e cpf 529.982.247-25"); got != "tel «tel:2» e cpf «cpf:3»" {
		t.Fatalf("2º Mascarar = %q", got)
	}
	if got := Devolver("«cpf:1» «tel:2» «cpf:3»", c.Ler()); got != "123.456.789-09 (19) 98162-1916 529.982.247-25" {
		t.Errorf("Devolver = %q", got)
	}
}

func TestCofreGuardar(t *testing.T) {
	c := NovoCofre()
	if err := c.Guardar(map[string]string{"«cpf:1»": "123.456.789-09"}); err != nil {
		t.Fatalf("Guardar: %v", err)
	}
	if err := c.Guardar(map[string]string{"«cpf:1»": "123.456.789-09"}); err != nil {
		t.Errorf("Guardar do mesmo valor: %v", err)
	}
	if err := c.Guardar(map[string]string{"«cpf:1»": "529.982.247-25"}); !errors.Is(err, ErrMarcadorOcupado) {
		t.Errorf("Guardar de outro valor: err = %v, want ErrMarcadorOcupado", err)
	}
	if err := c.Guardar(map[string]string{"cpf:2": "529.982.247-25"}); !errors.Is(err, ErrMarcadorInvalido) {
		t.Errorf("Guardar fora do formato: err = %v, want ErrMarcadorInvalido", err)
	}
	if got := c.Ler()["«cpf:1»"]; got != "123.456.789-09" {
		t.Errorf("valor sobrescrito: %q", got)
	}
	// Numeração continua depois do maior marcador guardado.
	if err := c.Guardar(map[string]string{"«tel:7»": "(19) 98162-1916"}); err != nil {
		t.Fatalf("Guardar: %v", err)
	}
	if got := c.Mascarar("529.982.247-25"); got != "«cpf:8»" {
		t.Errorf("Mascarar depois de Guardar = %q, want «cpf:8»", got)
	}
}

func TestCofreLerDevolveCopia(t *testing.T) {
	c := NovoCofre()
	c.Mascarar("123.456.789-09")
	lido := c.Ler()
	lido["«cpf:1»"] = "outro"
	if c.Ler()["«cpf:1»"] != "123.456.789-09" {
		t.Error("Ler devolveu o mapa interno")
	}
}

func TestCofreNuncaSerializaNemLoga(t *testing.T) {
	c := NovoCofre()
	c.Mascarar("123.456.789-09 (19) 94040-2710")

	var log strings.Builder
	slogJSON(&log).Info("turno", "cofre", c)
	jsonBytes, err := json.Marshal(map[string]any{"cofre": c})
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	saidas := map[string]string{
		"%v":   fmt.Sprintf("%v", c),
		"%+v":  fmt.Sprintf("%+v", c),
		"%#v":  fmt.Sprintf("%#v", c),
		"%s":   fmt.Sprintf("%s", c),
		"json": string(jsonBytes),
		"slog": log.String(),
	}
	for formato, saida := range saidas {
		if strings.Contains(saida, "789-09") || strings.Contains(saida, "2710") || strings.Contains(saida, "«cpf:1»") {
			t.Errorf("%s vazou o cofre: %s", formato, saida)
		}
	}
}
