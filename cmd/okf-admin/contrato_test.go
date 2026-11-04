package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

var atualizarContrato = flag.Bool("atualizar-contrato", false, "regrava os contratos em testdata/contrato")

// conferirContrato compara o corpo (JSON indentado) com testdata/contrato/<nome>.json.
func conferirContrato(t *testing.T, nome string, corpo []byte) {
	t.Helper()
	var buf bytes.Buffer
	if err := json.Indent(&buf, corpo, "", "  "); err != nil {
		t.Fatalf("contrato %s: corpo não é JSON: %v", nome, err)
	}
	buf.WriteByte('\n')
	caminho := filepath.Join("testdata", "contrato", nome+".json")
	if *atualizarContrato {
		if err := os.MkdirAll(filepath.Dir(caminho), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(caminho, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	quer, err := os.ReadFile(caminho)
	if err != nil {
		t.Fatalf("contrato %s: %v", nome, err)
	}
	if !bytes.Equal(quer, buf.Bytes()) {
		t.Fatalf("contrato %s mudou\nquer:\n%s\ngot:\n%s", nome, quer, buf.Bytes())
	}
}

func TestContratosDoStudio(t *testing.T) {
	amb := novoAmbienteRotas(t)
	casos := []struct{ nome, usuario, metodo, url, corpo string }{
		{nome: "eu", usuario: "gestor-a", metodo: "GET", url: "/api/eu"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			w := amb.pedir(t, c.usuario, c.metodo, c.url, c.corpo)
			if w.Code != http.StatusOK {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
			conferirContrato(t, c.nome, w.Body.Bytes())
		})
	}
}
