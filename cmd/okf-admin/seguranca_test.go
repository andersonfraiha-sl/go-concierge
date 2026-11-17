package main

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestCabecalhosDeSegurancaNasTelas(t *testing.T) {
	amb := novoAmbienteRotas(t)
	casos := []struct {
		caminho string
		csp     string
	}{
		{"/studio/", cspStudio},
		{"/console", cspTelasAntigas},
		{"/admin", cspTelasAntigas},
		{"/backdoor", cspTelasAntigas},
		{"/backdoor/orquestracao", cspTelasAntigas},
		{"/backdoor/payloads", cspTelasAntigas},
		{"/backdoor/administradoras", cspTelasAntigas},
		{"/console.js", cspTelasAntigas},
		{"/healthz", cspTelasAntigas},
	}
	for _, c := range casos {
		t.Run(c.caminho, func(t *testing.T) {
			w := amb.pedir(t, "admin-superlogica", "GET", c.caminho, "")
			if w.Code != http.StatusOK {
				t.Fatalf("status %d: a tela antiga quebrou", w.Code)
			}
			quer := map[string]string{
				"Content-Security-Policy": c.csp,
				"X-Content-Type-Options":  "nosniff",
				"Referrer-Policy":         "strict-origin-when-cross-origin",
				"X-Frame-Options":         "DENY",
				"Permissions-Policy":      permissoesNegadas,
			}
			for nome, valor := range quer {
				if got := w.Header().Get(nome); got != valor {
					t.Errorf("%s = %q, quer %q", nome, got, valor)
				}
			}
		})
	}
	if w := amb.pedir(t, "admin-superlogica", "GET", "/api/agentes", ""); w.Header().Get("Content-Security-Policy") != cspStudio {
		t.Fatalf("a API sai com a CSP estrita: %q", w.Header().Get("Content-Security-Policy"))
	}
}

// As telas antigas não podem ganhar script inline nem origem externa fora da CSP sem que a
// CSP acompanhe: o navegador bloquearia em silêncio.
func TestTelasAntigasCabemNaCSP(t *testing.T) {
	paginas, err := filepath.Glob(filepath.Join("..", "..", "web", "okf", "*.html"))
	if err != nil || len(paginas) == 0 {
		t.Fatalf("páginas de web/okf: %v", err)
	}
	scriptInline := regexp.MustCompile(`<script(\s[^>]*)?>\s*[^<\s]`)
	origemExterna := regexp.MustCompile(`(?:src|href)="(https://[^/"]+)`)
	manipuladorInline := regexp.MustCompile(`\son[a-z]+="`)
	for _, p := range paginas {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		html := string(raw)
		if scriptInline.MatchString(html) || manipuladorInline.MatchString(html) {
			t.Errorf("%s: script inline exige 'unsafe-inline' em script-src", p)
		}
		for _, m := range origemExterna.FindAllStringSubmatch(html, -1) {
			if !strings.Contains(cspTelasAntigas, m[1]) {
				t.Errorf("%s: %s fora da CSP das telas antigas", p, m[1])
			}
		}
	}
}
