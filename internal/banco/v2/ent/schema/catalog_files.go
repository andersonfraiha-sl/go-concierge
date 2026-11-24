package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

type CatalogFile struct{ ent.Schema }

func (CatalogFile) Fields() []ent.Field {
	return []ent.Field{
		field.Text("id").SchemaType(map[string]string{dialect.Postgres: "text"}).StorageKey("path"),
		field.Bytes("content").SchemaType(map[string]string{dialect.Postgres: "bytea"}),
		field.Text("sha256").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Time("updated_at").SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (CatalogFile) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "catalog_files"},
		entsql.Checks(map[string]string{
			"catalog_files_path_check": "path ~ '^(agentes|nexus)/[^/]+(/[^/]+)*$' AND path !~ '(^|/)[.]{1,2}(/|$)' AND position(chr(92) in path) = 0",
		}),
	}
}
