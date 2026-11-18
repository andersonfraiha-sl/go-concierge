package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"go-concierge/internal/banco/v2/ent/annotation"
)

type UseCase struct{ ent.Schema }

func (UseCase) Fields() []ent.Field {
	return []ent.Field{
		field.Text("id").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Text("file_path").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Text("agent_path").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Text("name").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Text("summary").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Text("icon").SchemaType(map[string]string{dialect.Postgres: "text"}).Optional(),
		field.Text("visibility").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Text("source_sha256").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Time("updated_at").SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (UseCase) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("file_path").Unique().StorageKey("use_cases_uq1"),
	}
}

func (UseCase) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "use_cases"},
		entsql.Checks(map[string]string{
			"use_cases_id_check": "id ~ '[^[:space:]]'",
		}),
		annotation.Constraints{
			ForeignKeys: []annotation.ForeignKey{
				{Symbol: "use_cases_fk1", Columns: []string{"file_path"}, Table: "catalog_files", Target: []string{"path"}},
			},
		},
	}
}
