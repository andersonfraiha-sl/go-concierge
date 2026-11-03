// Package web embute as interfaces estáticas do go-concierge.
package web

import (
	"embed"
	"io/fs"
)

//go:embed okf/*
var okfFS embed.FS

// OKF é o Console e as duas páginas antigas que o Studio ainda não cobre (servidos na raiz do okf-admin).
var OKF fs.FS = must(fs.Sub(okfFS, "okf"))

//go:embed all:studio
var studioFS embed.FS

// Studio é o front compilado do Studio (npm run build em studio/), servido em /studio/.
// Sem compilar, a pasta só tem o .gitkeep e o servidor responde 503.
var Studio fs.FS = must(fs.Sub(studioFS, "studio/dist"))

func must(f fs.FS, err error) fs.FS {
	if err != nil {
		panic(err)
	}
	return f
}
