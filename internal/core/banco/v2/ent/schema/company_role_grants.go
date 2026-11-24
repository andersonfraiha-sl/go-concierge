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

type CompanyRoleGrant struct{ ent.Schema }

func (CompanyRoleGrant) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").SchemaType(map[string]string{dialect.Postgres: "bigint"}),
		field.Text("company_id").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Text("role_id").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Text("permission").SchemaType(map[string]string{dialect.Postgres: "text"}),
	}
}

func (CompanyRoleGrant) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("company_id", "role_id", "permission").Unique().StorageKey("company_role_grants_uq1"),
	}
}

func (CompanyRoleGrant) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "company_role_grants"},
		entsql.Checks(map[string]string{
			"company_role_grants_company_id_check": "company_id IS NULL OR company_id ~ '[^[:space:]]'",
		}),
		annotation.Constraints{
			ForeignKeys: []annotation.ForeignKey{
				{Symbol: "company_role_grants_fk1", Columns: []string{"role_id"}, Table: "roles", Target: []string{"id"}},
			},
		},
	}
}
