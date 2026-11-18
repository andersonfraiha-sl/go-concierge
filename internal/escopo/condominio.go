package escopo

import (
	"strconv"
	"strings"
)

func ChaveCondominio(company, condo string) string {
	return strconv.Itoa(len(company)) + ":" + company + ":" + condo
}

func CondominioDaChave(chave string) (company, condo string, ok bool) {
	tamanho, resto, existe := strings.Cut(chave, ":")
	n, err := strconv.Atoi(tamanho)
	if !existe || err != nil || n < 1 || len(resto) <= n || resto[n] != ':' {
		return "", "", false
	}
	return resto[:n], resto[n+1:], resto[n+1:] != ""
}
