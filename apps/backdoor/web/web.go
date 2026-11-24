package web

import (
	"embed"
	"io/fs"
)

//go:embed okf/*
var legado embed.FS

//go:embed all:studio/dist
var studio embed.FS

var OKF = sub(legado, "okf")
var Studio = sub(studio, "studio/dist")

func sub(arquivos fs.FS, nome string) fs.FS {
	sub, err := fs.Sub(arquivos, nome)
	if err != nil {
		panic(err)
	}
	return sub
}
