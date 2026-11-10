package memoria

import (
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Tipo diz de onde vem uma coisa lembrada e quanto ela vive.
type Tipo string

const (
	TipoPreferencia Tipo = "preferencia"
	TipoPadrao      Tipo = "padrao_de_uso"
)

const (
	chaveNomePreferido   = "nome_preferido"
	chaveTamanhoResposta = "tamanho_resposta"
	chaveTratamento      = "tratamento"
	chaveFormatoBoleto   = "formato_boleto"

	maxRunasNomePreferido = 30
	maxPalavrasNome       = 2
)

// ChavesDePreferencia são as únicas chaves que a memória aceita, em ordem alfabética. A
// lista cresce por decisão de produto, nunca pelo modelo.
func ChavesDePreferencia() []string {
	return []string{chaveFormatoBoleto, chaveNomePreferido, chaveTamanhoResposta, chaveTratamento}
}

// NormalizarPreferencia devolve o fato com chave e valor normalizados, e true, quando a
// chave está na lista e o valor é válido para ela. Fora disso devolve false: o modelo não
// cria preferência fora da lista. Fonte, condomínio, data e confiança passam intactos.
func NormalizarPreferencia(f Fato) (Fato, bool) {
	f.Chave = NormalizarChave(f.Chave)
	var valor string
	var valido bool
	switch f.Chave {
	case chaveNomePreferido:
		valor, valido = nomeValido(f.Texto)
	case chaveTamanhoResposta:
		valor, valido = valorDaLista(f.Texto, "curta", "detalhada")
	case chaveTratamento:
		valor, valido = valorDaLista(f.Texto, "informal", "formal")
	case chaveFormatoBoleto:
		valor, valido = valorDaLista(f.Texto, "linha_digitavel", "pix", "link")
	}
	if !valido {
		return Fato{}, false
	}
	f.Texto = valor
	return f, true
}

func valorDaLista(texto string, permitidos ...string) (string, bool) {
	valor := NormalizarChave(texto)
	return valor, slices.Contains(permitidos, valor)
}

// nomeValido: só letras, até duas palavras, até 30 caracteres. É a única preferência de
// texto livre, por isso a mais estreita.
func nomeValido(texto string) (string, bool) {
	palavras := strings.Fields(texto)
	if len(palavras) == 0 || len(palavras) > maxPalavrasNome {
		return "", false
	}
	nome := strings.Join(palavras, " ")
	soLetras := func(r rune) bool { return unicode.IsLetter(r) || r == ' ' }
	if utf8.RuneCountInString(nome) > maxRunasNomePreferido || strings.IndexFunc(nome, func(r rune) bool { return !soLetras(r) }) >= 0 {
		return "", false
	}
	return nome, true
}

// SoPreferencias descarta o que NormalizarPreferencia recusa e devolve o resto normalizado.
// É o filtro de leitura: fato gravado por regras antigas (chave livre, valor em frase) some.
func SoPreferencias(fatos []Fato) []Fato {
	out := make([]Fato, 0, len(fatos))
	for _, f := range fatos {
		if p, ok := NormalizarPreferencia(f); ok {
			out = append(out, p)
		}
	}
	return out
}

// PreferenciasVigentes é SoPreferencias sem as que passaram de validade sem serem
// reafirmadas. Fato sem data conta como vencido: não há como provar que é recente.
func PreferenciasVigentes(fatos []Fato, agora time.Time, validade time.Duration) []Fato {
	return slices.DeleteFunc(SoPreferencias(fatos), func(f Fato) bool {
		return agora.Sub(f.Quando) > validade
	})
}
