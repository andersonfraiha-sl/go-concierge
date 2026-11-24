package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go-concierge/apps/console/web"
	"go-concierge/internal/httpapp"
)

func Novo(cfg httpapp.Config) (http.Handler, error) {
	return novo(cfg, NewEmulador())
}

func novo(cfg httpapp.Config, emulador *Emulador) (http.Handler, error) {
	proxy, err := httpapp.NovoProxy(cfg.ConciergeURL)
	if err != nil {
		return nil, err
	}
	proxy.ModifyResponse = func(resp *http.Response) error {
		if resp.StatusCode != http.StatusOK || (resp.Request.URL.Path != "/api/atende/sessao" && resp.Request.URL.Path != "/api/atende/transbordo" && resp.Request.URL.Path != "/api/pedidos/acao") {
			return nil
		}
		defer resp.Body.Close()
		var corpo map[string]json.RawMessage
		if err := json.NewDecoder(resp.Body).Decode(&corpo); err != nil {
			return fmt.Errorf("ler sessão: %w", err)
		}
		var sessao struct {
			ID string `json:"id"`
		}
		if _, presente := corpo["sessao"]; !presente && resp.Request.URL.Path == "/api/pedidos/acao" {
			raw, err := json.Marshal(corpo)
			if err != nil {
				return err
			}
			resp.Body = io.NopCloser(bytes.NewReader(raw))
			resp.ContentLength = int64(len(raw))
			resp.Header.Set("Content-Length", strconv.Itoa(len(raw)))
			return nil
		}
		if err := json.Unmarshal(corpo["sessao"], &sessao); err != nil || sessao.ID == "" {
			return fmt.Errorf("sessão sem identificador")
		}
		recebidas, err := json.Marshal(emulador.Recebidas(sessao.ID))
		if err != nil {
			return err
		}
		corpo["recebidas"] = recebidas
		raw, err := json.Marshal(corpo)
		if err != nil {
			return err
		}
		resp.Body = io.NopCloser(bytes.NewReader(raw))
		resp.ContentLength = int64(len(raw))
		resp.Header.Set("Content-Length", strconv.Itoa(len(raw)))
		return nil
	}
	mux := http.NewServeMux()
	httpapp.Saude(mux, cfg.ConciergeURL)
	for _, rota := range []string{"/api/eu", "/api/atende/", "/api/pedidos", "/api/pedidos/", "/api/memoria", "/api/memorias"} {
		mux.Handle(rota, proxy)
	}
	mux.Handle("POST /emulador/atende/webhook", proxy)
	mux.Handle("POST /emulador/atende/messages", receber(cfg, emulador))
	arquivos := web.Arquivos()
	mux.Handle("GET /{$}", http.RedirectHandler("/console", http.StatusFound))
	mux.Handle("GET /console", httpapp.Pagina(arquivos, "console.html"))
	httpapp.Estaticos(mux, arquivos, "console.js", "styles.css", "favicon.svg", "favicon-32x32.png")
	for _, rota := range []string{"/studio", "/studio/", "/admin", "/backdoor", "/backdoor/"} {
		mux.Handle("GET "+rota, httpapp.Redirecionar(cfg.BackdoorURL, rota))
	}
	return mux, nil
}

func receber(cfg httpapp.Config, emulador *Emulador) http.Handler {
	client := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	alvo := strings.TrimRight(cfg.ConciergeURL, "/") + "/api/atende/entrega-emulada/validar"
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			httpapp.JSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "mensagem excede o limite"})
			return
		}
		req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, alvo, bytes.NewReader(raw))
		if err != nil {
			httpapp.JSON(w, http.StatusBadGateway, map[string]string{"error": "Não foi possível validar a entrega."})
			return
		}
		req.URL.RawQuery = r.URL.RawQuery
		for _, nome := range []string{"Authorization", "Cookie", "X-Dev-Usuario", "X-Authenticated-User-Email", "Content-Type"} {
			for _, valor := range r.Header.Values(nome) {
				req.Header.Add(nome, valor)
			}
		}
		resp, err := client.Do(req)
		if err != nil {
			httpapp.JSON(w, http.StatusBadGateway, map[string]string{"error": "Não foi possível validar a entrega."})
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
			w.WriteHeader(resp.StatusCode)
			_, _ = io.Copy(w, resp.Body)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(raw))
		emulador.Handler(w, r)
	})
}
