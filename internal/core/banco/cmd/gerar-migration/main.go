package main

import "log"

func main() {
	log.Fatal("geração de migrations pelo Ent interno desativada: a fonte física canônica é internal/core/banco/v2/ent/schema; valide a baseline com go run ./internal/core/banco/v2/cmd/gerar-baseline -check e versione mudanças posteriores em internal/core/banco/v2/migrations")
}
