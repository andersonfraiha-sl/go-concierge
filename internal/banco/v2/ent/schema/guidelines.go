package schema

import (
	"encoding/json"
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"go-concierge/internal/banco/v2/ent/annotation"
)

type Guideline struct{ ent.Schema }

func (Guideline) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").SchemaType(map[string]string{dialect.Postgres: "bigint"}),
		field.Text("scope_type").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Text("company_id").SchemaType(map[string]string{dialect.Postgres: "text"}).Optional(),
		field.Int32("condo_id").SchemaType(map[string]string{dialect.Postgres: "integer"}).Optional(),
		field.Text("general_text").SchemaType(map[string]string{dialect.Postgres: "text"}).Annotations(entsql.DefaultExpr("''")),
		field.JSON("by_use_case", json.RawMessage{}).SchemaType(map[string]string{dialect.Postgres: "jsonb"}).Annotations(entsql.DefaultExpr("'{}'::jsonb")),
	}
}

func (Guideline) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("scope_type", "company_id", "condo_id").Unique().StorageKey("guidelines_uq1"),
	}
}

func (Guideline) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "guidelines"},
		entsql.Checks(map[string]string{
			"guidelines_company_id_check": "company_id IS NULL OR company_id ~ '[^[:space:]]'",
			"guidelines_scope_type_check": "scope_type IN ('GLOBAL', 'COMPANY', 'CONDO')",
			"guidelines_scope_check":      "(scope_type = 'GLOBAL' AND company_id IS NULL AND condo_id IS NULL) OR (scope_type = 'COMPANY' AND company_id IS NOT NULL AND condo_id IS NULL) OR (scope_type = 'CONDO' AND company_id IS NOT NULL AND condo_id IS NOT NULL)",
		}),
		annotation.Constraints{
			NullsNotDistinct: []string{"guidelines_uq1"},
		},
	}
}
