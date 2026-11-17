package main

import (
	"net/http"
	"strings"
)

// cspStudio vale para o Studio, a API e o resto. style-src 'unsafe-inline' é inevitável: o
// MUI (emotion) injeta <style> em tempo de execução. Script só do próprio servidor.
const cspStudio = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data:; font-src 'self' data:; connect-src 'self'; object-src 'none'; " +
	"base-uri 'self'; form-action 'self'; frame-ancestors 'none'"

// cspTelasAntigas vale para as páginas de web/okf (Console, /admin, /backdoor): elas carregam
// o lucide do jsDelivr e as fontes do Google, e usam style="" no HTML. Nenhuma tem script
// inline, então script-src continua sem 'unsafe-inline'.
const cspTelasAntigas = "default-src 'self'; script-src 'self' https://cdn.jsdelivr.net; " +
	"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; img-src 'self' data:; " +
	"font-src 'self' https://fonts.gstatic.com; connect-src 'self'; object-src 'none'; " +
	"base-uri 'self'; form-action 'self'; frame-ancestors 'none'"

const permissoesNegadas = "camera=(), microphone=(), geolocation=(), payment=(), usb=()"

// comCabecalhosDeSeguranca põe os cabeçalhos em toda resposta. Nada embute estas telas em
// iframe, então o enquadramento fica negado (X-Frame-Options para navegador antigo).
func comCabecalhosDeSeguranca(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", cspDoCaminho(r.URL.Path))
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Permissions-Policy", permissoesNegadas)
		next.ServeHTTP(w, r)
	})
}

func cspDoCaminho(caminho string) string {
	if strings.HasPrefix(caminho, "/studio/") || strings.HasPrefix(caminho, "/api/") {
		return cspStudio
	}
	return cspTelasAntigas
}
