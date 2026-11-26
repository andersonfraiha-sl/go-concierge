package catalogo

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"
)

const TamanhoTrecho = 2 << 20
const ArquivosPorPagina = 8

type Trecho struct {
	Caminho  string `json:"caminho"`
	Conteudo []byte `json:"conteudo"`
	Inicio   int64  `json:"inicio"`
	Total    int64  `json:"total"`
}

type Pagina struct {
	Versao       string   `json:"versao"`
	Trechos      []Trecho `json:"trechos"`
	Proximo      string   `json:"proximo"`
	Deslocamento int64    `json:"deslocamento"`
	Fim          bool     `json:"fim"`
}

func Paginar(arquivos []Arquivo, caminho string, deslocamento int64) (Pagina, error) {
	if deslocamento < 0 {
		return Pagina{}, fmt.Errorf("deslocamento inválido")
	}
	arquivos = slices.Clone(arquivos)
	slices.SortFunc(arquivos, func(a, b Arquivo) int { return strings.Compare(a.Caminho, b.Caminho) })
	h := sha256.New()
	for _, a := range arquivos {
		fmt.Fprintf(h, "%d:%s%s", len(a.Caminho), a.Caminho, SHA256(a.Conteudo))
	}
	out := Pagina{Versao: hex.EncodeToString(h.Sum(nil)), Trechos: []Trecho{}, Proximo: caminho, Deslocamento: deslocamento, Fim: true}
	for _, a := range arquivos {
		if a.Caminho < caminho || (a.Caminho == caminho && deslocamento >= int64(len(a.Conteudo))) {
			continue
		}
		inicio := int64(0)
		if a.Caminho == caminho {
			inicio = deslocamento
		}
		fim := min(int64(len(a.Conteudo)), inicio+TamanhoTrecho)
		out.Trechos = append(out.Trechos, Trecho{Caminho: a.Caminho, Inicio: inicio, Total: int64(len(a.Conteudo)), Conteudo: a.Conteudo[inicio:fim]})
		out.Proximo, out.Deslocamento = a.Caminho, fim
		if fim < int64(len(a.Conteudo)) || len(out.Trechos) == ArquivosPorPagina {
			out.Fim = false
			break
		}
	}
	return out, nil
}
