package okf

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// Manutenção do routers/rotas.md quando a família muda de forma (porta de
// runtime/rotas.py do adam). Mover, clonar, criar ou excluir um subagente mexe no
// pai que perdeu o filho e no que ganhou: o arquivo de rotas é o classificador
// que o orquestrador lê, e deixá-lo desatualizado é pior do que não tê-lo.
//
// Duas cautelas deliberadas:
//   - não inventamos routers/: um pai sem a pasta roteia pelas descrições, e criar
//     o arquivo mudaria a natureza dele (com routers/ e subagentes, vira orquestrador);
//   - a linha de escape (→ rota `self`) fica por último: uma rota nova inserida
//     depois dela seria inalcançável.

const arquivoRotas = "rotas.md"

var (
	reItem    = regexp.MustCompile(`^\s*[-*]\s+`)
	reRotaRef = regexp.MustCompile("rota\\s+`([^`]+)`")
)

func caminhoRotas(pai string) string { return filepath.Join(pai, "routers", arquivoRotas) }

func rotaDoBloco(bloco []string) string {
	var partes []string
	for _, l := range bloco {
		partes = append(partes, strings.TrimSpace(l))
	}
	if m := reRotaRef.FindStringSubmatch(strings.Join(partes, " ")); m != nil {
		return strings.TrimSpace(m[1])
	}
	return ""
}

// blocos agrupa o arquivo em itens de lista COM suas continuações (uma regra pode
// quebrar entre o critério e o `→ rota`).
func blocos(linhas []string) [][]string {
	var out [][]string
	for _, l := range linhas {
		continuacao := len(out) > 0 &&
			!reItem.MatchString(l) &&
			(strings.HasPrefix(l, " ") || strings.HasPrefix(l, "\t")) &&
			strings.TrimSpace(l) != "" &&
			reItem.MatchString(out[len(out)-1][0])
		if continuacao {
			out[len(out)-1] = append(out[len(out)-1], l)
		} else {
			out = append(out, []string{l})
		}
	}
	return out
}

// RemoverRota tira a rota do agente que saiu (o item inteiro). Devolve o que mudou.
func RemoverRota(pai, nome string) (string, error) {
	arq := caminhoRotas(pai)
	raw, err := os.ReadFile(arq)
	if err != nil {
		return "", nil
	}
	bl := blocos(strings.Split(string(raw), "\n"))
	var sobraram [][]string
	for _, b := range bl {
		if rotaDoBloco(b) != nome {
			sobraram = append(sobraram, b)
		}
	}
	if len(sobraram) == len(bl) {
		return "", nil
	}
	var linhas []string
	for _, b := range sobraram {
		linhas = append(linhas, b...)
	}
	texto := strings.TrimRight(strings.Join(linhas, "\n"), "\n") + "\n"
	if err := os.WriteFile(arq, []byte(texto), 0o644); err != nil {
		return "", err
	}
	return fmt.Sprintf("rota '%s' removida de %s/routers/%s", nome, filepath.Base(pai), arquivoRotas), nil
}

// pontoDeEntrada: sempre ANTES da saída padrão (`self`); se o self mora numa seção
// `##` própria, entra antes do título dessa seção.
func pontoDeEntrada(bl [][]string) int {
	escape := -1
	for i, b := range bl {
		if rotaDoBloco(b) == "self" {
			escape = i
			break
		}
	}
	if escape == -1 {
		ultimo := -1
		for i, b := range bl {
			if rotaDoBloco(b) != "" {
				ultimo = i
			}
		}
		if ultimo == -1 {
			return len(bl)
		}
		return ultimo + 1
	}
	for i := escape - 1; i >= 0; i-- {
		if rotaDoBloco(bl[i]) != "" {
			break
		}
		if len(bl[i]) > 0 && strings.HasPrefix(strings.TrimSpace(bl[i][0]), "#") {
			return i
		}
	}
	return escape
}

// RegistrarRota põe a rota do agente que chegou, antes da saída padrão.
func RegistrarRota(pai, nome, descricao string) (string, error) {
	arq := caminhoRotas(pai)
	raw, err := os.ReadFile(arq)
	if err != nil {
		return "", nil // pai sem routers/: roteia pela descrição, nada a fazer
	}
	bl := blocos(strings.Split(strings.TrimRight(string(raw), "\n"), "\n"))
	for _, b := range bl {
		if rotaDoBloco(b) == nome {
			return "", nil
		}
	}
	novo := linhasDaRota(nome, descricao)

	onde := pontoDeEntrada(bl)
	if onde < len(bl) && len(bl[onde]) > 0 && strings.HasPrefix(strings.TrimSpace(bl[onde][0]), "#") {
		novo = append(novo, "")
	}
	var saida [][]string
	saida = append(saida, bl[:onde]...)
	saida = append(saida, novo)
	saida = append(saida, bl[onde:]...)
	var linhas []string
	for _, b := range saida {
		linhas = append(linhas, b...)
	}
	if err := os.WriteFile(arq, []byte(strings.Join(linhas, "\n")+"\n"), 0o644); err != nil {
		return "", err
	}
	return fmt.Sprintf("rota '%s' criada em %s/routers/%s", nome, filepath.Base(pai), arquivoRotas), nil
}

// linhasDaRota monta o item `- <critério> → rota `nome`.` já quebrado em 78 colunas.
func linhasDaRota(nome, descricao string) []string {
	criterio := strings.TrimSpace(descricao)
	if criterio == "" {
		criterio = "assunto de " + nome
	}
	criterio = strings.TrimRight(criterio, ".;")
	r := []rune(criterio)
	criterio = strings.ToUpper(string(r[0])) + string(r[1:])
	return quebrar(fmt.Sprintf("- %s → rota `%s`.", criterio, nome), 78, "  ")
}

// TrocarRota reescreve o item da rota no mesmo lugar do arquivo; sem o item, registra como novo.
func TrocarRota(pai, nome, descricao string) (string, error) {
	arq := caminhoRotas(pai)
	raw, err := os.ReadFile(arq)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil // pai sem routers/: roteia pela descrição, nada a fazer
	}
	if err != nil {
		return "", err
	}
	bl := blocos(strings.Split(strings.TrimRight(string(raw), "\n"), "\n"))
	i := slices.IndexFunc(bl, func(b []string) bool { return rotaDoBloco(b) == nome })
	if i < 0 {
		return RegistrarRota(pai, nome, descricao)
	}
	bl[i] = linhasDaRota(nome, descricao)
	if err := os.WriteFile(arq, []byte(strings.Join(slices.Concat(bl...), "\n")+"\n"), 0o644); err != nil {
		return "", err
	}
	return fmt.Sprintf("rota '%s' atualizada em %s/routers/%s", nome, filepath.Base(pai), arquivoRotas), nil
}

// quebrar: textwrap simples por palavras, com indentação nas continuações.
func quebrar(texto string, largura int, indent string) []string {
	var linhas []string
	atual := ""
	for _, palavra := range strings.Fields(texto) {
		cand := palavra
		if atual != "" {
			cand = atual + " " + palavra
		}
		if len([]rune(cand)) > largura && atual != "" {
			linhas = append(linhas, atual)
			atual = indent + palavra
			continue
		}
		atual = cand
	}
	if atual != "" {
		linhas = append(linhas, atual)
	}
	return linhas
}

// SincronizarRotas acerta os dois lados depois de mover/clonar/criar/excluir.
// paiAntigo vazio = clone ou criação (o original fica); paiNovo vazio = raiz (sem rotas).
func SincronizarRotas(paiAntigo, paiNovo, agente, nome string) ([]string, error) {
	var mudancas []string
	if paiAntigo != "" {
		m, err := RemoverRota(paiAntigo, nome)
		if err != nil {
			return mudancas, err
		}
		if m != "" {
			mudancas = append(mudancas, m)
		}
	}
	if paiNovo != "" {
		m, err := RegistrarRota(paiNovo, nome, Descrever(agente))
		if err != nil {
			return mudancas, err
		}
		if m != "" {
			mudancas = append(mudancas, m)
		}
	}
	return mudancas, nil
}
