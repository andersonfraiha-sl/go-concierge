package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	padraoHealthz = "GET /healthz"
	padraoReadyz  = "GET /readyz"

	// subcomandoSaude é o healthcheck do contêiner: a imagem distroless não tem shell nem
	// curl, então o próprio binário consulta o /readyz.
	subcomandoSaude = "saude"
	prazoSaude      = 3 * time.Second
)

// ehSondaDeSaude: as sondas batem a cada poucos segundos e não entram no log de requisições.
func ehSondaDeSaude(padrao string) bool { return padrao == padraoHealthz || padrao == padraoReadyz }

// saude faz GET na url e devolve 0 com 200, 1 com qualquer outra coisa.
func saude(args []string, erro io.Writer) int {
	fs := flag.NewFlagSet(subcomandoSaude, flag.ContinueOnError)
	fs.SetOutput(erro)
	url := fs.String("url", "http://127.0.0.1:8090/readyz", "endereço da sonda")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cli := &http.Client{Timeout: prazoSaude}
	res, err := cli.Get(*url)
	if err != nil {
		fmt.Fprintf(erro, "%s: %v\n", subcomandoSaude, err)
		return 1
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		fmt.Fprintf(erro, "%s: %s respondeu %d\n", subcomandoSaude, *url, res.StatusCode)
		return 1
	}
	return 0
}
