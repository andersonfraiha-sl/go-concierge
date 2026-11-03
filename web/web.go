// Package web embute as interfaces estáticas do go-concierge.
package web

import (
	"embed"
	"io/fs"
)

//go:embed okf/*
var okfFS embed.FS

// OKF é o Console e as telas antigas que o Studio ainda não cobre (Admin e Backdoor),
// servidos na raiz do okf-admin.
var OKF fs.FS = must(fs.Sub(okfFS, "okf"))

//go:embed all:studio/dist
var studioFS embed.FS

// Studio é o front compilado do Studio (npm run build em web/studio/), servido em /studio/.
// Só a saída do build entra no binário, nunca o código-fonte nem node_modules. Sem
// compilar, dist só tem o .gitkeep e o servidor responde 503.
var Studio fs.FS = must(fs.Sub(studioFS, "studio/dist"))

func must(f fs.FS, err error) fs.FS {
	if err != nil {
		panic(err)
	}
	return f
}
