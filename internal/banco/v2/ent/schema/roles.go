package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"go-concierge/internal/banco/v2/ent/annotation"
)

type Role struct{ ent.Schema }

func (Role) Fields() []ent.Field {
	return []ent.Field{
		field.Text("id").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Text("inherits_from").SchemaType(map[string]string{dialect.Postgres: "text"}).Optional(),
	}
}

func (Role) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "roles"},
		annotation.Constraints{
			ForeignKeys: []annotation.ForeignKey{
				{Symbol: "roles_fk1", Columns: []string{"inherits_from"}, Table: "roles", Target: []string{"id"}},
			},
		},
	}
}
