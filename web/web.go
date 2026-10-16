// Package web embute as interfaces estáticas do go-concierge.
package web

import (
	"embed"
	"io/fs"
)

//go:embed okf/*
var okfFS embed.FS

// OKF é a interface de cadastro de skills (servida na raiz do okf-admin).
var OKF fs.FS = must(fs.Sub(okfFS, "okf"))

func must(f fs.FS, err error) fs.FS {
	if err != nil {
		panic(err)
	}
	return f
}
