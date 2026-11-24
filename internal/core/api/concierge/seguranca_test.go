package conciergeapi

import (
	"testing"
)

func TestCabecalhosDeSegurancaNoNucleo(t *testing.T) {
	amb := novoAmbienteRotas(t)
	casos := []struct {
		caminho string
		csp     string
		status  int
	}{
		{"/studio/", cspStudio, 404},
		{"/console", cspTelasAntigas, 404},
		{"/admin", cspTelasAntigas, 404},
		{"/backdoor", cspTelasAntigas, 404},
		{"/console.js", cspTelasAntigas, 404},
		{"/healthz", cspTelasAntigas, 200},
	}
	for _, c := range casos {
		t.Run(c.caminho, func(t *testing.T) {
			w := amb.pedir(t, "admin-superlogica", "GET", c.caminho, "")
			if w.Code != c.status {
				t.Fatalf("status %d, esperado %d", w.Code, c.status)
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
