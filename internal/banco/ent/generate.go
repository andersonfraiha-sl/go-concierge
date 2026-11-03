// Package ent guarda o código gerado pelo ent a partir de ./schema. Não edite à mão:
// rode go generate ./internal/banco/ent.
package ent

//go:generate go run -mod=mod entgo.io/ent/cmd/ent generate --feature sql/upsert,sql/execquery ./schema
