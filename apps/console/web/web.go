package web

import (
	"embed"
	"io/fs"
)

//go:embed okf/*
var arquivos embed.FS

func Arquivos() fs.FS {
	sub, err := fs.Sub(arquivos, "okf")
	if err != nil {
		panic(err)
	}
	return sub
}
