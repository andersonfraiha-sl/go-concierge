// Package catalogo guarda no Postgres os arquivos do catálogo (agentes/** e nexus/**). O
// banco é a fonte da verdade; o disco é a cópia de trabalho que o okf.Catalogo lê, refeita
// do banco a cada boot. O núcleo (Diferenca, ValidarCaminho) é puro; LerDisco, Materializar
// e Postgres são as bordas com I/O.
package catalogo

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"
)

// Pastas do catálogo na raiz: só o que está sob elas vai ao banco.
const (
	DirAgentes = "agentes"
	DirNexus   = "nexus"
)

var pastasDoCatalogo = []string{DirAgentes, DirNexus}

// ErrCaminhoInvalido é o caminho que não é um arquivo limpo sob agentes/ ou nexus/.
var ErrCaminhoInvalido = errors.New("caminho inválido no catálogo")

// Arquivo é um arquivo do catálogo: caminho relativo à raiz, com barras, e os bytes como são.
type Arquivo struct {
	Caminho  string
	Conteudo []byte
}

// SHA256 é o resumo do conteúdo em hexadecimal, o que a tabela guarda para comparar sem
// trazer os bytes.
func SHA256(conteudo []byte) string {
	soma := sha256.Sum256(conteudo)
	return hex.EncodeToString(soma[:])
}

// ValidarCaminho aceita só caminho relativo, limpo, com barras, sob agentes/ ou nexus/.
// Recusa o que escaparia da raiz ao virar caminho de disco: "..", absoluto, barra invertida
// e ":" (letra de unidade do Windows). É a fronteira de segurança de Materializar.
func ValidarCaminho(caminho string) error {
	if strings.ContainsAny(caminho, "\\:\x00") || path.Clean(caminho) != caminho {
		return fmt.Errorf("%w: %q", ErrCaminhoInvalido, caminho)
	}
	pasta, resto, temPasta := strings.Cut(caminho, "/")
	if !temPasta || resto == "" || !slices.Contains(pastasDoCatalogo, pasta) {
		return fmt.Errorf("%w: %q (use agentes/... ou nexus/...)", ErrCaminhoInvalido, caminho)
	}
	return nil
}

// Diferenca compara o banco (caminho → sha256) com o disco e diz o que o banco precisa:
// gravar o que é novo ou mudou, remover o que não está mais no disco. Ordenada por caminho.
func Diferenca(noBanco map[string]string, noDisco []Arquivo) (gravar []Arquivo, remover []string) {
	presentes := make(map[string]struct{}, len(noDisco))
	for _, a := range noDisco {
		presentes[a.Caminho] = struct{}{}
		if resumo, existe := noBanco[a.Caminho]; !existe || resumo != SHA256(a.Conteudo) {
			gravar = append(gravar, a)
		}
	}
	for caminho := range noBanco {
		if _, existe := presentes[caminho]; !existe {
			remover = append(remover, caminho)
		}
	}
	slices.SortFunc(gravar, func(a, b Arquivo) int { return strings.Compare(a.Caminho, b.Caminho) })
	slices.Sort(remover)
	return gravar, remover
}
