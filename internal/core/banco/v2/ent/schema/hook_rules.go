package schema

import (
	"encoding/json"
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

type HookRule struct{ ent.Schema }

func (HookRule) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).SchemaType(map[string]string{dialect.Postgres: "uuid"}),
		field.Text("scope_type").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Text("company_id").SchemaType(map[string]string{dialect.Postgres: "text"}).Optional(),
		field.Other("condo_ids", IntegerArray{}).SchemaType(map[string]string{dialect.Postgres: "integer[]"}).Annotations(entsql.DefaultExpr("'{}'::integer[]")),
		field.Int32("version").SchemaType(map[string]string{dialect.Postgres: "integer"}),
		field.Text("name").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Text("event").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Bool("enabled"),
		field.JSON("definition", json.RawMessage{}).SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.Text("updated_by").SchemaType(map[string]string{dialect.Postgres: "text"}).Annotations(entsql.DefaultExpr("''")),
		field.Time("updated_at").SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (HookRule) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("event", "enabled").StorageKey("hook_rules_ix1"),
		index.Fields("company_id", "event").StorageKey("hook_rules_ix2"),
	}
}

func (HookRule) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "hook_rules"},
		entsql.Checks(map[string]string{
			"hook_rules_company_id_check": "company_id IS NULL OR company_id ~ '[^[:space:]]'",
			"hook_rules_scope_type_check": "scope_type IN ('GLOBAL', 'COMPANY')",
			"hook_rules_scope_check":      "(scope_type = 'GLOBAL' AND company_id IS NULL AND cardinality(condo_ids) = 0) OR (scope_type = 'COMPANY' AND company_id IS NOT NULL)",
			"hook_rules_condo_ids_check":  "array_position(condo_ids, NULL) IS NULL",
		}),
	}
}
