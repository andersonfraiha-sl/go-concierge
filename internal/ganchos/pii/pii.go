// Package pii troca CPF e telefone do texto por marcadores («cpf:N», «tel:N») antes
// de ele ir para modelo, log ou histórico, e devolve os valores no mesmo turno.
//
// A numeração e a guarda dos valores acontecem juntas, sob a trava do Cofre
// (Cofre.Mascarar, Cofre.MascararEmValor): ferramentas rodam em paralelo no turno e
// dois resultados nunca podem ganhar o mesmo marcador.
//
// O texto é casado numa cópia normalizada (NFKC, sem caracteres de largura zero,
// espaços e traços Unicode viram ASCII), mas a troca é feita no texto original: o
// resto da mensagem não muda.
//
// Limites conhecidos:
//   - Telefone sem DDD (4040-2710) não é mascarado, por decisão de produto: senão
//     código de retirada e protocolo de 8 ou 9 dígitos seriam mascarados junto.
//   - Marcador forjado pelo modelo: o morador pode escrever <<cpf:2>> e pedir ao
//     modelo que o reescreva com « ». Devolver revelaria o valor do cofre do turno,
//     inclusive o que veio de resultado de ferramenta. A mitigação fica na Task 7:
//     pii_na_resposta só revela marcadores permitidos para a resposta.
package pii

import (
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

type tipo string

const (
	tipoCPF tipo = "cpf"
	tipoTel tipo = "tel"
)

// trecho é um CPF ou telefone achado em texto[ini:fim] (offsets do texto original).
type trecho struct {
	ini, fim int
	tipo     tipo
}

const (
	// Uma faixa com menos dígitos que o menor telefone (DDD + 8) não tem PII.
	minDigitosNaFaixa = 10
	// Nenhum padrão tem mais que 14 dígitos seguidos (+55 0 19 981621916): um grupo
	// maior é outro número (protocolo, código de barras) e corta a faixa.
	maxDigitosSeguidos = 14
	// O trecho mais longo possível: "+55 (019) 9 8162-1916" tem 21 bytes. Cortar a
	// entrada do regexp nisso deixa cada tentativa O(1) em vez de O(resto da faixa).
	maxBytesNoTrecho = 24
)

var (
	// Os padrões rodam na cópia ASCII normalizada da faixa.
	reCPF = regexp.MustCompile(`^\d{3}[ .\-]?\d{3}[ .\-]?\d{3}[ .\-/]?\d{2}`)
	// DDD obrigatório, com 0 de operadora opcional. Depois do DDD: celular com o 9
	// (separado ou não), ou 8 dígitos começando em 2 a 9 (fixo ou celular sem o 9).
	reTelefone = regexp.MustCompile(`^(?:\+?55[ .\-]?)?(?:\(0?[1-9]\d\)|0?[1-9]\d)[ .\-]?(?:9[ .\-]?\d{4}|[2-9]\d{3})[ .\-]?\d{4}`)
	reMarcador = regexp.MustCompile(`«(?:cpf|tel):\d+»`)

	neutralizador = strings.NewReplacer("«", `"`, "»", `"`)
)

// mascarar troca cada CPF (com DV válido) e telefone por um marcador numerado a partir
// de proximo, na ordem do texto. Valor repetido reusa o marcador. Marcador digitado é
// neutralizado antes, para não casar com o cofre. Puro: quem numera e guarda junto é
// o Cofre.
func mascarar(texto string, proximo int) (mascarado string, achados map[string]string) {
	p := planejar(texto)
	if len(p.trechos) == 0 {
		return p.texto, nil
	}
	valores := distintos([]plano{p})
	achados = make(map[string]string, len(valores))
	marcas := marcar(valores, proximo, achados)
	return aplicar(p, indexar(valores, marcas)), achados
}

// plano é o texto já neutralizado com os trechos achados, antes da numeração. Achar é
// a parte cara e roda fora da trava do cofre; só marcar roda dentro.
type plano struct {
	texto   string
	trechos []trecho
}

func planejar(texto string) plano {
	neutro := Neutralizar(texto)
	return plano{texto: neutro, trechos: achar(neutro)}
}

// valorAchado é um CPF ou telefone ainda sem marcador.
type valorAchado struct {
	tipo  tipo
	valor string
}

// distintos lista os valores dos planos sem repetição, na ordem em que aparecem.
func distintos(planos []plano) []valorAchado {
	var (
		valores []valorAchado
		vistos  map[string]bool
	)
	for _, p := range planos {
		for _, t := range p.trechos {
			valor := p.texto[t.ini:t.fim]
			if vistos[valor] {
				continue
			}
			if vistos == nil {
				vistos = map[string]bool{}
			}
			vistos[valor] = true
			valores = append(valores, valorAchado{tipo: t.tipo, valor: valor})
		}
	}
	return valores
}

// marcar dá a cada valor o marcador de número proximo, proximo+1, ... e o grava em
// destino (marcador → valor). É o que roda sob a trava do cofre: só concatena e grava.
func marcar(valores []valorAchado, proximo int, destino map[string]string) []string {
	marcas := make([]string, len(valores))
	var buf []byte
	for i, v := range valores {
		buf = append(buf[:0], "«"...)
		buf = append(buf, v.tipo...)
		buf = append(buf, ':')
		buf = strconv.AppendInt(buf, int64(proximo+i), 10)
		buf = append(buf, "»"...)
		marcas[i] = string(buf)
		destino[marcas[i]] = v.valor
	}
	return marcas
}

// indexar monta valor → marcador para aplicar.
func indexar(valores []valorAchado, marcas []string) map[string]string {
	marcadorDe := make(map[string]string, len(valores))
	for i, v := range valores {
		marcadorDe[v.valor] = marcas[i]
	}
	return marcadorDe
}

// aplicar troca cada trecho do plano pelo marcador do seu valor.
func aplicar(p plano, marcadorDe map[string]string) string {
	return substituir(p.texto, p.trechos, func(_ trecho, valor string) string { return marcadorDe[valor] })
}

// mascararAnonimo é a máscara do log: sem número, sem cofre.
func mascararAnonimo(texto string) string {
	return substituir(texto, achar(texto), func(t trecho, _ string) string { return "«" + string(t.tipo) + "»" })
}

// Neutralizar troca « e » por aspas: o que o morador digitou nunca vira marcador.
func Neutralizar(texto string) string {
	if !strings.Contains(texto, "«") && !strings.Contains(texto, "»") {
		return texto
	}
	return neutralizador.Replace(texto)
}

// TemMarcador diz se o texto tem algum marcador de PII.
func TemMarcador(texto string) bool { return reMarcador.MatchString(texto) }

// Devolver troca os marcadores do cofre pelos valores. Marcador fora do cofre fica
// como está. CPF que não veio do cofre (inventado ou copiado pelo modelo) sai «cpf».
func Devolver(texto string, cofre map[string]string) string {
	if trechos := achar(texto); len(trechos) > 0 {
		doCofre := map[string]bool{}
		for _, v := range cofre {
			doCofre[digitosDe(v)] = true
		}
		alheios := slices.DeleteFunc(trechos, func(t trecho) bool {
			return t.tipo != tipoCPF || doCofre[digitosDe(texto[t.ini:t.fim])]
		})
		texto = substituir(texto, alheios, func(trecho, string) string { return "«cpf»" })
	}
	if !strings.Contains(texto, "«") {
		return texto
	}
	return reMarcador.ReplaceAllStringFunc(texto, func(m string) string {
		if v, ok := cofre[m]; ok {
			return v
		}
		return m
	})
}

// mascararEmValor mascara toda string dentro de map[string]any e []any (resultado de
// ferramenta), numerando a partir de proximo na ordem das chaves. Não muda o original.
func mascararEmValor(v any, proximo int) (any, map[string]string) {
	planos := planejarValor(v)
	valores := distintos(planos)
	achados := make(map[string]string, len(valores))
	marcas := marcar(valores, proximo, achados)
	return aplicarValor(v, planos, indexar(valores, marcas)), achados
}

// planejarValor planeja cada string de v, na ordem de mapearStrings.
func planejarValor(v any) []plano {
	var planos []plano
	visitarStrings(v, func(s string) { planos = append(planos, planejar(s)) })
	return planos
}

// aplicarValor copia v trocando cada string pelo seu plano aplicado; planos vem de
// planejarValor(v), na mesma ordem.
func aplicarValor(v any, planos []plano, marcadorDe map[string]string) any {
	proxima := 0
	return mapearStrings(v, func(string) string {
		p := planos[proxima]
		proxima++
		return aplicar(p, marcadorDe)
	})
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

// visitarStrings passa cada string de v a f na mesma ordem de mapearStrings, sem
// copiar nada.
func visitarStrings(v any, f func(string)) {
	switch x := v.(type) {
	case string:
		f(x)
	case map[string]any:
		for _, k := range slices.Sorted(maps.Keys(x)) {
			visitarStrings(x[k], f)
		}
	case []any:
		for _, item := range x {
			visitarStrings(item, f)
		}
	}
}

// achar devolve CPFs e telefones do texto em ordem, sem sobreposição, com offsets do
// texto original. CPF ganha do telefone quando os dois casam no mesmo trecho.
func achar(texto string) []trecho {
	var trechos []trecho
	for _, f := range faixas(texto) {
		trechos = append(trechos, f.achar()...)
	}
	return trechos
}

// faixa é uma sequência do texto feita só de dígitos, separadores e caracteres de
// largura zero, com a cópia ASCII normalizada e, para cada byte dela, onde ele
// começa e termina no original.
type faixa struct {
	ascii    []byte
	ini, fim []int
}

func (f faixa) achar() []trecho {
	s := string(f.ascii)
	cpfs := varrer(s, reCPF, podeSerCPF, func(ini, fim int) bool { return cpfValido(s[ini:fim]) })
	// Telefone que encosta num CPF é recusado ali e a varredura segue: o "(19) ..."
	// logo depois de um CPF terminado em 55 ainda é achado.
	tels := varrer(s, reTelefone, podeSerTelefone, func(ini, fim int) bool {
		return !slices.ContainsFunc(cpfs, func(c [2]int) bool { return ini < c[1] && c[0] < fim })
	})
	trechos := make([]trecho, 0, len(cpfs)+len(tels))
	for _, c := range cpfs {
		trechos = append(trechos, f.trecho(c, tipoCPF))
	}
	for _, t := range tels {
		trechos = append(trechos, f.trecho(t, tipoTel))
	}
	slices.SortFunc(trechos, func(a, b trecho) int { return a.ini - b.ini })
	return trechos
}

func (f faixa) trecho(loc [2]int, t tipo) trecho {
	return trecho{ini: f.ini[loc[0]], fim: f.fim[loc[1]-1], tipo: t}
}

type classe int

const (
	outro classe = iota
	digito
	separador
	invisivel
)

// classificar diz o papel da runa numa faixa e o byte ASCII que a representa.
func classificar(r rune) (byte, classe) {
	if r < utf8.RuneSelf {
		return classificarASCII(byte(r))
	}
	switch r {
	case '\u00AD', '\u200B', '\u200C', '\u200D', '\u200E', '\u200F', '\u2060', '\uFEFF':
		return 0, invisivel
	case '\u2010', '\u2011', '\u2012', '\u2013', '\u2014', '\u2015', '\u2212', '\uFE63':
		return '-', separador
	}
	if unicode.IsSpace(r) {
		return ' ', separador
	}
	// NFKC só onde ele pode dar ASCII: formas de largura cheia (１, （, －) e dígitos
	// Unicode. Acento comum da prosa não paga a normalização.
	if ehLarguraCheia(r) || unicode.IsDigit(r) {
		if n := norm.NFKC.String(string(r)); len(n) == 1 && n[0] < utf8.RuneSelf {
			return classificarASCII(n[0])
		}
	}
	return 0, outro
}

// podeTerDigito é o atalho do texto sem número: nenhum dígito ASCII e nenhum byte que
// abra largura cheia (EF) ou dígito matemático (F0), os únicos que o NFKC leva a ASCII.
func podeTerDigito(texto string) bool {
	for i := 0; i < len(texto); i++ {
		if abreDigito[texto[i]] {
			return true
		}
	}
	return false
}

var abreDigito = [256]bool{'0': true, '1': true, '2': true, '3': true, '4': true, '5': true,
	'6': true, '7': true, '8': true, '9': true, 0xEF: true, 0xF0: true}

func ehLarguraCheia(r rune) bool { return r >= '＀' && r <= '￯' }

func classificarASCII(b byte) (byte, classe) {
	if b >= utf8.RuneSelf {
		return 0, outro
	}
	p := papelASCII[b]
	return p.b, p.c
}

type papel struct {
	b byte
	c classe
}

// papelASCII é a tabela de classificar para ASCII (o caminho quente da prosa).
// Só leitura depois de montada.
var papelASCII = func() (t [utf8.RuneSelf]papel) {
	for d := byte('0'); d <= '9'; d++ {
		t[d] = papel{d, digito}
	}
	for _, sep := range []byte(" .-/()+") {
		t[sep] = papel{sep, separador}
	}
	t['\t'] = papel{' ', separador}
	return t
}()

// faixas corta o texto nas sequências que podem conter PII. Texto sem dígito sai
// sem alocar nada.
func faixas(texto string) []faixa {
	if !podeTerDigito(texto) {
		return nil
	}
	var (
		saida     []faixa
		atual     faixa
		seguidos  int  // dígitos seguidos no fim da faixa atual
		pendente  = -1 // início dos separadores antes do primeiro dígito
		descartar bool
	)
	// atual é rascunho reaproveitado: só a faixa que fica é copiada.
	digitosNaFaixa := 0
	fechar := func() {
		if len(atual.ascii) == 0 {
			seguidos, pendente, digitosNaFaixa = 0, -1, 0
			return
		}
		if digitosNaFaixa >= minDigitosNaFaixa {
			saida = append(saida, faixa{ascii: slices.Clone(atual.ascii), ini: slices.Clone(atual.ini), fim: slices.Clone(atual.fim)})
		}
		atual = faixa{ascii: atual.ascii[:0], ini: atual.ini[:0], fim: atual.fim[:0]}
		seguidos, pendente, digitosNaFaixa = 0, -1, 0
	}
	for i, r := range texto {
		var (
			b byte
			c classe
		)
		if r < utf8.RuneSelf {
			b, c = papelASCII[r].b, papelASCII[r].c
		} else {
			b, c = classificar(r)
		}
		switch {
		case c == outro && len(atual.ascii) == 0 && pendente < 0: // letra da prosa: nada a fechar
			descartar = false
		case c == outro:
			fechar()
			descartar = false
		case c == invisivel:
		case c == digito && descartar:
		case c == digito:
			seguidos++
			if seguidos > maxDigitosSeguidos {
				corte := len(atual.ascii) - (seguidos - 1)
				digitosNaFaixa -= seguidos - 1
				atual.ascii, atual.ini, atual.fim = atual.ascii[:corte], atual.ini[:corte], atual.fim[:corte]
				fechar()
				descartar = true
				continue
			}
			if len(atual.ascii) == 0 && pendente >= 0 {
				atual.acrescentarSeparadores(texto[:i], pendente)
			}
			digitosNaFaixa++
			atual.acrescentar(b, i, i+utf8.RuneLen(r))
		case len(atual.ascii) == 0: // separador antes de qualquer dígito: só anota
			seguidos, descartar = 0, false
			if pendente < 0 {
				pendente = i
			}
		default: // separador
			seguidos, descartar = 0, false
			atual.acrescentar(b, i, i+utf8.RuneLen(r))
		}
	}
	fechar()
	return saida
}

// acrescentarSeparadores põe na faixa vazia os separadores de texto[desde:], que
// vieram antes do primeiro dígito ("+55", "(").
func (f *faixa) acrescentarSeparadores(texto string, desde int) {
	for j, r := range texto[desde:] {
		if b, c := classificar(r); c == separador {
			f.acrescentar(b, desde+j, desde+j+utf8.RuneLen(r))
		}
	}
}

// acrescentar põe o byte na faixa. O fim do byte anterior estica até aqui para
// cobrir os invisíveis no meio, que somem junto com o trecho.
func (f *faixa) acrescentar(b byte, ini, fim int) {
	if n := len(f.fim); n > 0 {
		f.fim[n-1] = ini
	}
	f.ascii = append(f.ascii, b)
	f.ini = append(f.ini, ini)
	f.fim = append(f.fim, fim)
}

// varrer acha os trechos que casam com re (ancorado em ^) sem dígito colado nas
// pontas (12 dígitos não são um CPF). Tenta só onde um número pode começar: no
// primeiro dígito de um grupo, em "(" e em "+". Ancorar evita reprocurar o resto da
// faixa a cada candidato recusado.
func varrer(s string, re *regexp.Regexp, podeIniciar func(string, int) bool, aceita func(ini, fim int) bool) [][2]int {
	var achados [][2]int
	for i := 0; i < len(s); i++ {
		if !podeIniciar(s, i) {
			continue
		}
		loc := re.FindStringIndex(s[i:min(len(s), i+maxBytesNoTrecho)])
		if loc == nil {
			continue
		}
		fim := i + loc[1]
		if isolado(s, i, fim) && aceita(i, fim) {
			achados = append(achados, [2]int{i, fim})
			i = fim - 1
		}
	}
	return achados
}

// podeSerCPF: o primeiro grupo de dígitos de um CPF tem 3, 6, 9 ou 11 dígitos
// (o separador vem depois de um grupo de 3, ou não vem). Poupa o regexp.
func podeSerCPF(s string, i int) bool {
	if !ehDigito(s[i]) || (i > 0 && ehDigito(s[i-1])) {
		return false
	}
	g := 0
	for i+g < len(s) && ehDigito(s[i+g]) {
		g++
	}
	return g == 3 || g == 6 || g == 9 || g == 11
}

// podeSerTelefone: um telefone começa onde um número pode começar e tem ao menos 10
// dígitos nos seus até maxBytesNoTrecho bytes. Poupa o regexp.
func podeSerTelefone(s string, i int) bool {
	if !podeComecar(s, i) {
		return false
	}
	digitos := 0
	for _, b := range []byte(s[i:min(len(s), i+maxBytesNoTrecho)]) {
		if ehDigito(b) {
			digitos++
		}
	}
	return digitos >= minDigitosNaFaixa
}

func podeComecar(s string, i int) bool {
	switch b := s[i]; {
	case b == '(' || b == '+':
		return true
	case ehDigito(b):
		return i == 0 || !ehDigito(s[i-1])
	}
	return false
}

// isolado: dígito colado em dígito emenda o número; "123(19) ..." já tem o "(" separando.
func isolado(s string, ini, fim int) bool {
	coladoAntes := ini > 0 && ehDigito(s[ini-1]) && ehDigito(s[ini])
	coladoDepois := fim < len(s) && ehDigito(s[fim])
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
	d := soDigitosASCII(s)
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

// digitosDe devolve os dígitos do valor já normalizados (fullwidth vira ASCII).
func digitosDe(s string) string {
	var b strings.Builder
	for _, r := range s {
		if d, c := classificar(r); c == digito {
			b.WriteByte(d)
		}
	}
	return b.String()
}

func soDigitosASCII(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, s)
}

func ehDigito(b byte) bool { return b >= '0' && b <= '9' }
