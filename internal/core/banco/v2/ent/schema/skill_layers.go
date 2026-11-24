package schema

import (
	"encoding/json"
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"go-concierge/internal/core/banco/v2/ent/annotation"
)

type SkillLayer struct{ ent.Schema }

func (SkillLayer) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").SchemaType(map[string]string{dialect.Postgres: "bigint"}),
		field.Text("skill").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Text("type").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Text("company_id").SchemaType(map[string]string{dialect.Postgres: "text"}).Optional(),
		field.Int32("condo_id").SchemaType(map[string]string{dialect.Postgres: "integer"}).Optional(),
		field.Int64("head_version_id").SchemaType(map[string]string{dialect.Postgres: "bigint"}).Optional(),
		field.Int64("parent_version_id").SchemaType(map[string]string{dialect.Postgres: "bigint"}).Optional(),
		field.JSON("draft", json.RawMessage{}).SchemaType(map[string]string{dialect.Postgres: "jsonb"}).Optional(),
		field.Text("draft_version").SchemaType(map[string]string{dialect.Postgres: "text"}).Annotations(entsql.DefaultExpr("''")),
	}
}

func (SkillLayer) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("skill", "type", "company_id", "condo_id").Unique().StorageKey("skill_layers_uq1"),
		index.Fields("company_id", "condo_id", "skill").StorageKey("skill_layers_ix1"),
	}
}

func (SkillLayer) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "skill_layers"},
		entsql.Checks(map[string]string{
			"skill_layers_company_id_check": "company_id IS NULL OR company_id ~ '[^[:space:]]'",
			"skill_layers_type_check":       "type IN ('BASE', 'COMPANY', 'CONDO')",
			"skill_layers_scope_check":      "(type = 'BASE' AND company_id IS NULL AND condo_id IS NULL) OR (type = 'COMPANY' AND company_id IS NOT NULL AND condo_id IS NULL) OR (type = 'CONDO' AND company_id IS NOT NULL AND condo_id IS NOT NULL)",
		}),
		annotation.Constraints{
			NullsNotDistinct: []string{"skill_layers_uq1"},
		},
	}
}
