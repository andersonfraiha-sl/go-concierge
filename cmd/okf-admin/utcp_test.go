package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestManualUTCPRepasseEPermissao(t *testing.T) {
	var caminhos []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		caminhos = append(caminhos, r.URL.RequestURI())
		for _, header := range []string{"Authorization", "Cookie", "Origin", "X-Dev-Usuario"} {
			if r.Header.Get(header) != "" {
				t.Errorf("credencial ou origem encaminhada: %s", header)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tools":[]}`))
	}))
	defer upstream.Close()
	amb := novoAmbienteRotas(t)
	var err error
	amb.s.utcp, err = proxyManualUTCP(upstream.URL + "/utcp")
	if err != nil {
		t.Fatal(err)
	}
	amb.handler = amb.s.rotear(nil)
	for _, caminho := range []string{"/utcp", "/utcp/"} {
		for _, caso := range []struct {
			usuario string
			status  int
		}{{"", 401}, {"operador-a", 403}, {"gestor-av", 403}, {"admin", 200}} {
			if w := amb.pedir(t, caso.usuario, "GET", caminho, ""); w.Code != caso.status {
				t.Fatalf("GET %s por %q: status %d, esperado %d: %s", caminho, caso.usuario, w.Code, caso.status, w.Body.String())
			}
		}
	}
	if len(caminhos) != 2 || caminhos[0] != "/utcp" || caminhos[1] != "/" {
		t.Fatalf("caminhos no publicador: %v", caminhos)
	}
	r := httptest.NewRequest("GET", "/utcp?token=descartar", nil)
	for _, header := range []string{"Authorization", "Cookie", "Origin", "X-Dev-Usuario"} {
		r.Header.Set(header, "valor-privado")
	}
	amb.s.utcp.ServeHTTP(httptest.NewRecorder(), r)
	if caminhos[2] != "/utcp" {
		t.Fatalf("query encaminhada: %s", caminhos[2])
	}
}

func TestManualUTCPRecusaURLInvalida(t *testing.T) {
	for _, endpoint := range []string{"", "file:///utcp", "http://localhost:3345/", "http://user:secret@localhost:3345/utcp", "http://localhost:3345/utcp?token=x", "http://localhost:3345/utcp#x"} {
		if _, err := proxyManualUTCP(endpoint); err == nil {
			t.Errorf("URL inválida aceita: %s", endpoint)
		}
	}
}
