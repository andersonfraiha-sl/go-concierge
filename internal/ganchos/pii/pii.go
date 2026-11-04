// Package pii troca CPF e telefone do texto por marcadores («cpf:N», «tel:N») antes
// de ele ir para modelo, log ou histórico, e devolve os valores no mesmo turno.
package pii

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
)

type tipo string

const (
	tipoCPF tipo = "cpf"
	tipoTel tipo = "tel"
)

// trecho é um CPF ou telefone achado em texto[ini:fim].
type trecho struct {
	ini, fim int
	tipo     tipo
}

var (
	reCPF = regexp.MustCompile(`\d{3}\.?\d{3}\.?\d{3}-?\d{2}`)
	// DDD obrigatório: sem ele, código de retirada e protocolo de 8 ou 9 dígitos
	// seriam confundidos com telefone.
	reTelefone = regexp.MustCompile(`(?:\+?55[\s-]?)?(?:\([1-9]\d\)|[1-9]\d)[\s-]?(?:9\d{4}|[2-5]\d{3})[\s-]?\d{4}`)
	reMarcador = regexp.MustCompile(`«(?:cpf|tel):\d+»`)
)

// Mascarar troca cada CPF (com DV válido) e telefone por um marcador numerado a partir
// de proximo, na ordem do texto. Valor repetido reusa o marcador. Marcador digitado é
// neutralizado antes, para não casar com o cofre.
func Mascarar(texto string, proximo int) (mascarado string, achados map[string]string) {
	texto = Neutralizar(texto)
	achados = map[string]string{}
	marcadorDe := map[string]string{}
	mascarado = substituir(texto, achar(texto), func(t trecho, valor string) string {
		if m, ok := marcadorDe[valor]; ok {
			return m
		}
		m := fmt.Sprintf("«%s:%d»", t.tipo, proximo+len(achados))
		marcadorDe[valor], achados[m] = m, valor
		return m
	})
	return mascarado, achados
}

// mascararAnonimo é a máscara do log: sem número, sem cofre.
func mascararAnonimo(texto string) string {
	return substituir(texto, achar(texto), func(t trecho, _ string) string { return "«" + string(t.tipo) + "»" })
}

// Neutralizar tira « e » do texto: o que o morador digitou nunca vira marcador.
func Neutralizar(texto string) string {
	return strings.NewReplacer("«", "", "»", "").Replace(texto)
}

// TemMarcador diz se o texto tem algum marcador de PII.
func TemMarcador(texto string) bool { return reMarcador.MatchString(texto) }

// Devolver troca os marcadores do cofre pelos valores. Marcador fora do cofre fica
// como está. CPF que não veio do cofre (inventado ou copiado pelo modelo) sai «cpf».
func Devolver(texto string, cofre map[string]string) string {
	doCofre := map[string]bool{}
	for _, v := range cofre {
		doCofre[soDigitos(v)] = true
	}
	alheios := slices.DeleteFunc(achar(texto), func(t trecho) bool {
		return t.tipo != tipoCPF || doCofre[soDigitos(texto[t.ini:t.fim])]
	})
	texto = substituir(texto, alheios, func(trecho, string) string { return "«cpf»" })
	return reMarcador.ReplaceAllStringFunc(texto, func(m string) string {
		if v, ok := cofre[m]; ok {
			return v
		}
		return m
	})
}

// MascararEmValor mascara toda string dentro de map[string]any e []any (resultado de
// ferramenta), numerando a partir de proximo na ordem das chaves. Não muda o original.
func MascararEmValor(v any, proximo int) (any, map[string]string) {
	achados := map[string]string{}
	mascarado := mapearStrings(v, func(s string) string {
		m, novos := Mascarar(s, proximo+len(achados))
		maps.Copy(achados, novos)
		return m
	})
	return mascarado, achados
}

// DevolverEmValor aplica Devolver a toda string dentro de map[string]any e []any
// (params de ferramenta). Não muda o original.
func DevolverEmValor(v any, cofre map[string]string) any {
	return mapearStrings(v, func(s string) string { return Devolver(s, cofre) })
}

// mapearStrings copia v trocando cada string por f(string). Chaves de mapa são
// visitadas em ordem, para a numeração dos marcadores ser determinística.
func mapearStrings(v any, f func(string) string) any {
	switch x := v.(type) {
	case string:
		return f(x)
	case map[string]any:
		out := make(map[string]any, len(x))
		for _, k := range slices.Sorted(maps.Keys(x)) {
			out[k] = mapearStrings(x[k], f)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = mapearStrings(item, f)
		}
		return out
	default:
		return v
	}
}

// achar devolve CPFs e telefones do texto em ordem, sem sobreposição. CPF ganha do
// telefone quando os dois casam no mesmo trecho.
func achar(texto string) []trecho {
	cpfs := varrer(texto, reCPF, tipoCPF, cpfValido)
	tels := slices.DeleteFunc(varrer(texto, reTelefone, tipoTel, nil), func(t trecho) bool {
		return slices.ContainsFunc(cpfs, func(c trecho) bool { return t.ini < c.fim && c.ini < t.fim })
	})
	return slices.SortedFunc(slices.Values(append(cpfs, tels...)), func(a, b trecho) int { return a.ini - b.ini })
}

// varrer acha os trechos que casam com re sem dígito colado nas pontas (12 dígitos não
// são um CPF). Quando um candidato é recusado, tenta de novo um byte adiante.
func varrer(texto string, re *regexp.Regexp, t tipo, aceita func(string) bool) []trecho {
	var trechos []trecho
	for i := 0; i < len(texto); {
		loc := re.FindStringIndex(texto[i:])
		if loc == nil {
			break
		}
		ini, fim := i+loc[0], i+loc[1]
		if isolado(texto, ini, fim) && (aceita == nil || aceita(texto[ini:fim])) {
			trechos = append(trechos, trecho{ini: ini, fim: fim, tipo: t})
			i = fim
			continue
		}
		i = ini + 1
	}
	return trechos
}

// isolado: dígito colado em dígito emenda o número; "123(19) ..." já tem o "(" separando.
func isolado(texto string, ini, fim int) bool {
	coladoAntes := ini > 0 && ehDigito(texto[ini-1]) && ehDigito(texto[ini])
	coladoDepois := fim < len(texto) && ehDigito(texto[fim])
	return !coladoAntes && !coladoDepois
}

// substituir troca cada trecho (em ordem, sem sobreposição) por troca(trecho, valor).
func substituir(texto string, trechos []trecho, troca func(trecho, string) string) string {
	if len(trechos) == 0 {
		return texto
	}
	var b strings.Builder
	ultimo := 0
	for _, t := range trechos {
		b.WriteString(texto[ultimo:t.ini])
		b.WriteString(troca(t, texto[t.ini:t.fim]))
		ultimo = t.fim
	}
	b.WriteString(texto[ultimo:])
	return b.String()
}

// cpfValido confere os dois dígitos verificadores; sequência repetida não vale.
func cpfValido(s string) bool {
	d := soDigitos(s)
	if strings.Count(d, d[:1]) == len(d) {
		return false
	}
	numeros := make([]int, len(d))
	for i := range d {
		numeros[i] = int(d[i] - '0')
	}
	return digitoVerificador(numeros[:9]) == numeros[9] && digitoVerificador(numeros[:10]) == numeros[10]
}

// digitoVerificador é o DV do CPF para os 9 ou 10 dígitos anteriores.
func digitoVerificador(d []int) int {
	soma := 0
	for i, x := range d {
		soma += x * (len(d) + 1 - i)
	}
	if resto := soma % 11; resto >= 2 {
		return 11 - resto
	}
	return 0
}

func soDigitos(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, s)
}

func ehDigito(b byte) bool { return b >= '0' && b <= '9' }
