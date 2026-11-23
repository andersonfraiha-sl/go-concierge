package main

import (
	"go-concierge/apps/backdoor/internal/api"
	"go-concierge/internal/httpapp"
)

func main() { httpapp.Executar("backdoor", "127.0.0.1:8091", api.Novo) }
