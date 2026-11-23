package main

import (
	"go-concierge/apps/console/internal/api"
	"go-concierge/internal/httpapp"
)

func main() { httpapp.Executar("console", "127.0.0.1:8092", api.Novo) }
