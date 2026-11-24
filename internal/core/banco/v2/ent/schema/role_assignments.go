package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"go-concierge/internal/core/banco/v2/ent/annotation"
)

type RoleAssignment struct{ ent.Schema }

func (RoleAssignment) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").SchemaType(map[string]string{dialect.Postgres: "bigint"}),
		field.Text("user_id").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Text("role_id").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Text("scope_type").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Text("company_id").SchemaType(map[string]string{dialect.Postgres: "text"}).Optional(),
		field.Int32("condo_id").SchemaType(map[string]string{dialect.Postgres: "integer"}).Optional(),
	}
}

func (RoleAssignment) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "role_id", "scope_type", "company_id", "condo_id").Unique().StorageKey("role_assignments_uq1"),
		index.Fields("company_id", "user_id").StorageKey("role_assignments_ix1"),
	}
}

func (RoleAssignment) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "role_assignments"},
		entsql.Checks(map[string]string{
			"role_assignments_company_id_check": "company_id IS NULL OR company_id ~ '[^[:space:]]'",
			"role_assignments_scope_type_check": "scope_type IN ('PLATFORM', 'COMPANY', 'CONDO')",
			"role_assignments_scope_check":      "(scope_type = 'PLATFORM' AND company_id IS NULL AND condo_id IS NULL) OR (scope_type = 'COMPANY' AND company_id IS NOT NULL AND condo_id IS NULL) OR (scope_type = 'CONDO' AND company_id IS NOT NULL AND condo_id IS NOT NULL)",
		}),
		annotation.Constraints{
			ForeignKeys: []annotation.ForeignKey{
				{Symbol: "role_assignments_fk1", Columns: []string{"user_id"}, Table: "users", Target: []string{"id"}},
				{Symbol: "role_assignments_fk2", Columns: []string{"role_id"}, Table: "roles", Target: []string{"id"}},
			},
			NullsNotDistinct: []string{"role_assignments_uq1"},
		},
	}
}
