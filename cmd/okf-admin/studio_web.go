package main

import (
	"errors"
	"io/fs"
	"log"
	"net/http"
	"path"
	"strings"
)

// studioEstatico serve o front compilado do Studio em /studio/. Caminho sem extensão é rota
// do cliente e recebe o index.html; os assets têm hash no nome e ficam em cache para sempre.
func studioEstatico(arquivos fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		index, err := fs.ReadFile(arquivos, "index.html")
		if errors.Is(err, fs.ErrNotExist) {
			writeJSON(w, http.StatusServiceUnavailable, erroAPI{Codigo: "STUDIO_NAO_COMPILADO", Mensagem: "O Studio não foi compilado: rode npm run build em web/studio/."})
			return
		}
		if err != nil {
			log.Printf("studio: ler index.html: %v", err)
			writeJSON(w, http.StatusInternalServerError, erroAPI{Codigo: "ERRO_INTERNO", Mensagem: "Não foi possível abrir o Studio."})
			return
		}
		caminho := strings.TrimPrefix(r.URL.Path, "/studio/")
		if caminho == "index.html" {
			caminho = "" // o index sai sempre sem cache, pelo ramo da rota do cliente
		}
		if ehArquivo(arquivos, caminho) {
			if strings.HasPrefix(caminho, "assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			http.ServeFileFS(w, r, arquivos, caminho)
			return
		}
		if path.Ext(caminho) != "" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(index)
	})
}

func ehArquivo(arquivos fs.FS, caminho string) bool {
	info, err := fs.Stat(arquivos, caminho)
	return err == nil && !info.IsDir()
}
