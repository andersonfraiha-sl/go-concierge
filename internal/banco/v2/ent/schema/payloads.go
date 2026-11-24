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

type Payload struct{ ent.Schema }

func (Payload) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").SchemaType(map[string]string{dialect.Postgres: "bigint"}),
		field.Text("scope_type").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Text("company_id").SchemaType(map[string]string{dialect.Postgres: "text"}).Optional(),
		field.Int32("condo_id").SchemaType(map[string]string{dialect.Postgres: "integer"}).Optional(),
		field.Text("resource_key").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.JSON("data", json.RawMessage{}).SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
	}
}

func (Payload) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("scope_type", "company_id", "condo_id", "resource_key").Unique().StorageKey("payloads_uq1"),
	}
}

func (Payload) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "payloads"},
		entsql.Checks(map[string]string{
			"payloads_company_id_check": "company_id IS NULL OR company_id ~ '[^[:space:]]'",
			"payloads_scope_type_check": "scope_type IN ('GLOBAL', 'COMPANY', 'CONDO')",
			"payloads_scope_check":      "(scope_type = 'GLOBAL' AND company_id IS NULL AND condo_id IS NULL) OR (scope_type = 'COMPANY' AND company_id IS NOT NULL AND condo_id IS NULL) OR (scope_type = 'CONDO' AND company_id IS NOT NULL AND condo_id IS NOT NULL)",
		}),
		annotation.Constraints{
			NullsNotDistinct: []string{"payloads_uq1"},
		},
	}
}
