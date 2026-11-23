package httpapp

import (
	"context"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const cspStudio = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'"
const cspLegado = "default-src 'self'; script-src 'self' https://cdn.jsdelivr.net; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; img-src 'self' data:; font-src 'self' https://fonts.gstatic.com; connect-src 'self'; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'"

func Cabecalhos(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/mcp/") {
			if err := http.NewResponseController(w).SetWriteDeadline(time.Time{}); err != nil {
				slog.WarnContext(r.Context(), "liberar resposta em fluxo", "erro", err)
			}
		} else {
			ctx, cancelar := context.WithTimeout(r.Context(), 130*time.Second)
			defer cancelar()
			r = r.WithContext(ctx)
		}
		csp := cspLegado
		if strings.HasPrefix(r.URL.Path, "/studio/") || strings.HasPrefix(r.URL.Path, "/api/") {
			csp = cspStudio
		}
		w.Header().Set("Content-Security-Policy", csp)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=()")
		next.ServeHTTP(w, r)
	})
}

func Pagina(arquivos fs.FS, nome string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		http.ServeFileFS(w, r, arquivos, nome)
	})
}

func Estaticos(mux *http.ServeMux, arquivos fs.FS, nomes ...string) {
	for _, nome := range nomes {
		mux.Handle("GET /"+nome, Pagina(arquivos, nome))
	}
}

func Redirecionar(base, caminho string) http.Handler {
	return http.RedirectHandler(strings.TrimRight(base, "/")+caminho, http.StatusFound)
}
