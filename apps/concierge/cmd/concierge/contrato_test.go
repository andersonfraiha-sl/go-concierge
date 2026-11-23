package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go-concierge/internal/copiloto"
)

var atualizarContrato = flag.Bool("atualizar-contrato", false, "regrava os contratos em testdata/contrato")

// conferirContrato compara o corpo (JSON indentado) com testdata/contrato/<nome>.json.
func conferirContrato(t *testing.T, nome string, corpo []byte) {
	t.Helper()
	var buf bytes.Buffer
	if err := json.Indent(&buf, bytes.TrimSpace(corpo), "", "  "); err != nil {
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
	titulo := "Agendar mudança"
	falso := &copilotoFalso{ops: []copiloto.Operacao{{
		Tipo: copiloto.PreencherTextos, Skill: "agendar-mudanca", Textos: &copiloto.TextosParciais{Titulo: &titulo},
	}}}
	amb.s.copiloto = falso.responder
	amb.s.limiteCopiloto = novoLimiteDoCopiloto(time.Now)
	pedidoDoCopiloto := corpoJSON(t, map[string]any{"mensagem": "Ajuste os textos", "historico": []any{}, "contexto": copiloto.Contexto{Tela: copiloto.TelaSkills}})
	casos := []struct{ nome, usuario, metodo, url, corpo string }{
		{nome: "eu", usuario: "gestor-a", metodo: "GET", url: "/api/eu"},
		{nome: "skills", usuario: "admin", metodo: "GET", url: "/api/skills"},
		{nome: "skill-textos", usuario: "admin", metodo: "GET", url: "/api/skills/agendar-mudanca"},
		{nome: "skill-fluxo", usuario: "admin", metodo: "GET", url: "/api/skills/agendar-mudanca/fluxo"},
		{nome: "estado", usuario: "gestor-a", metodo: "GET", url: "/api/admin/estado?administradora=adm-a"},
		{nome: "copiloto", usuario: "admin", metodo: "POST", url: "/api/copiloto", corpo: pedidoDoCopiloto},
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
