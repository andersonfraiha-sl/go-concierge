// Package env carrega arquivos .env (NOME=valor) no ambiente do processo, sem
// dependências. O ambiente do sistema tem precedência; arquivo ausente não é erro.
package env

import (
	"bufio"
	"os"
	"strings"
)

// Carregar lê os arquivos na ordem e define só as variáveis ainda vazias. Aceita
// comentários (#), `export `, aspas simples ou duplas e comentário no fim da linha.
func Carregar(caminhos ...string) {
	for _, c := range caminhos {
		f, err := os.Open(c)
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			linha := strings.TrimSpace(sc.Text())
			if linha == "" || strings.HasPrefix(linha, "#") {
				continue
			}
			linha = strings.TrimPrefix(linha, "export ")
			k, v, ok := strings.Cut(linha, "=")
			if !ok {
				continue
			}
			k = strings.TrimSpace(k)
			v = strings.TrimSpace(v)
			if len(v) >= 2 && (v[0] == '"' && v[len(v)-1] == '"' || v[0] == '\'' && v[len(v)-1] == '\'') {
				v = v[1 : len(v)-1]
			} else if i := strings.Index(v, " #"); i >= 0 {
				v = strings.TrimSpace(v[:i])
			}
			if k != "" && os.Getenv(k) == "" {
				_ = os.Setenv(k, v)
			}
		}
		f.Close()
	}
}

// Ou devolve a variável ou o padrão.
func Ou(k, padrao string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return padrao
}
