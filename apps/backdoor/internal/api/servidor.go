package api

import (
	"net/http"

	"go-concierge/apps/backdoor/web"
	"go-concierge/internal/httpapp"
)

func Novo(cfg httpapp.Config) (http.Handler, error) {
	proxy, err := httpapp.NovoProxy(cfg.ConciergeURL)
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	httpapp.Saude(mux, cfg.ConciergeURL)
	mux.Handle("/api/", proxy)
	mux.Handle("/mcp/", proxy)
	mux.Handle("GET /{$}", http.RedirectHandler("/studio/", http.StatusFound))
	mux.Handle("GET /studio", http.RedirectHandler("/studio/", http.StatusMovedPermanently))
	mux.Handle("GET /studio/", studioEstatico(web.Studio))
	for rota, nome := range map[string]string{
		"/admin": "index.html", "/backdoor": "backdoor.html", "/backdoor/orquestracao": "orquestracao.html", "/backdoor/payloads": "payloads.html", "/backdoor/administradoras": "administradoras.html",
	} {
		mux.Handle("GET "+rota, httpapp.Pagina(web.OKF, nome))
	}
	httpapp.Estaticos(mux, web.OKF, "styles.css", "favicon.svg", "favicon-32x32.png", "admin.js", "backdoor.js", "orquestracao.js", "catalogo-nexus.js")
	mux.Handle("GET /console", httpapp.Redirecionar(cfg.ConsoleURL, "/console"))
	return mux, nil
}
