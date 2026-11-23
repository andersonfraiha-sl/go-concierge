package httpapp

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

func NovoProxy(base string) (*httputil.ReverseProxy, error) {
	alvo, err := url.Parse(base)
	if err != nil || alvo.Host == "" || (alvo.Scheme != "http" && alvo.Scheme != "https") || alvo.User != nil || alvo.RawQuery != "" || alvo.Fragment != "" {
		return nil, fmt.Errorf("CONCIERGE_URL deve ser uma URL HTTP sem credenciais, query ou fragmento")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = 125 * time.Second
	return &httputil.ReverseProxy{
		Rewrite: func(p *httputil.ProxyRequest) {
			p.SetURL(alvo)
			p.Out.Host = alvo.Host
		},
		Transport:     transport,
		FlushInterval: -1,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			slog.WarnContext(r.Context(), "Concierge indisponível", "erro", err)
			JSON(w, http.StatusBadGateway, map[string]string{"codigo": "CONCIERGE_INDISPONIVEL", "error": "Não foi possível consultar o Concierge."})
		},
	}, nil
}

func JSON(w http.ResponseWriter, status int, valor any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(valor); err != nil {
		slog.Warn("escrever resposta JSON", "erro", err)
	}
}
