package okf

import (
	"path"
	"regexp"
	"strings"
)

// Pacotes OKF v0.1 (Open Knowledge Format, Google): um diretório de markdown com
// frontmatter (`type` obrigatório; title, description, resource, tags, timestamp
// opcionais), conceitos ligados por links markdown relativos, `index.md` opcional por
// pasta (progressive disclosure: o agente navega pelo índice em vez de ver tudo) e
// `log.md` com o histórico. Um bundle copiado para knowledge/ é lido como está:
//
//   - o índice do prompt mostra só o index.md de uma pasta que o tem (e esconde os
//     irmãos e as subpastas); o agente abre o index com ler_dado e segue os links;
//   - log.md nunca entra no índice nem na lista de docs, mas continua legível;
//   - ao devolver um .md, os links relativos são reescritos para o caminho canônico
//     a partir da pasta do agente (knowledge/tables/foo.md), que é o que ler_dado aceita.

const (
	ArquivoIndex = "index.md"
	ArquivoLog   = "log.md"
)

// podarIndice aplica a progressive disclosure e tira os log.md da caixinha knowledge/.
// Os assets e tudo fora de knowledge/ passam como estão.
func podarIndice(arquivos []Arquivo) []Arquivo {
	comIndex := map[string]bool{}
	for _, f := range arquivos {
		if strings.HasPrefix(f.Caminho, "knowledge/") && path.Base(f.Caminho) == ArquivoIndex {
			comIndex[path.Dir(f.Caminho)] = true
		}
	}
	out := make([]Arquivo, 0, len(arquivos))
	for _, f := range arquivos {
		if !strings.HasPrefix(f.Caminho, "knowledge/") {
			out = append(out, f)
			continue
		}
		if path.Base(f.Caminho) == ArquivoLog {
			continue
		}
		if cobertoPorIndex(f.Caminho, comIndex) {
			continue
		}
		out = append(out, f)
	}
	return out
}

// cobertoPorIndex: algum diretório acima do arquivo (o próprio, para arquivos comuns;
// só os estritamente acima, para um index.md) tem index.md.
func cobertoPorIndex(caminho string, comIndex map[string]bool) bool {
	d := path.Dir(caminho)
	if path.Base(caminho) == ArquivoIndex {
		d = path.Dir(d)
	}
	for d != "." && d != "/" && d != "" {
		if comIndex[d] {
			return true
		}
		d = path.Dir(d)
	}
	return false
}

var reLinkMD = regexp.MustCompile(`(\]\()([^)\s]+)(\))`)

// ReescreverLinks troca os links markdown relativos de um documento (relativos ao
// próprio arquivo, como manda o OKF) pelo caminho a partir da pasta do agente, para o
// modelo poder passá-los direto a ler_dado. Links absolutos, com esquema (http, mailto)
// ou âncoras ficam como estão; o que sairia da pasta do agente também.
func ReescreverLinks(caminhoDoc, texto string) string {
	dir := path.Dir(caminhoDoc)
	return reLinkMD.ReplaceAllStringFunc(texto, func(m string) string {
		sub := reLinkMD.FindStringSubmatch(m)
		alvo := sub[2]
		if strings.Contains(alvo, "://") || strings.HasPrefix(alvo, "mailto:") || strings.HasPrefix(alvo, "#") || strings.HasPrefix(alvo, "/") {
			return m
		}
		ancora := ""
		if i := strings.Index(alvo, "#"); i >= 0 {
			alvo, ancora = alvo[:i], alvo[i:]
		}
		if i := strings.Index(alvo, "?"); i >= 0 {
			alvo = alvo[:i]
		}
		if alvo == "" {
			return m
		}
		novo := path.Join(dir, alvo)
		if novo == ".." || strings.HasPrefix(novo, "../") {
			return m
		}
		return sub[1] + novo + ancora + sub[3]
	})
}
