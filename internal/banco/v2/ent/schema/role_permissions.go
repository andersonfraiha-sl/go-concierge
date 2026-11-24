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

type RolePermission struct{ ent.Schema }

func (RolePermission) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").SchemaType(map[string]string{dialect.Postgres: "bigint"}),
		field.Text("role_id").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Text("permission").SchemaType(map[string]string{dialect.Postgres: "text"}),
	}
}

func (RolePermission) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("role_id", "permission").Unique().StorageKey("role_permissions_uq1"),
	}
}

func (RolePermission) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "role_permissions"},
		annotation.Constraints{
			ForeignKeys: []annotation.ForeignKey{
				{Symbol: "role_permissions_fk1", Columns: []string{"role_id"}, Table: "roles", Target: []string{"id"}},
			},
		},
	}
}
