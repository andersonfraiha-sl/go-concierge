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

type SkillVersion struct{ ent.Schema }

func (SkillVersion) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").SchemaType(map[string]string{dialect.Postgres: "bigint"}),
		field.Int64("layer_id").SchemaType(map[string]string{dialect.Postgres: "bigint"}),
		field.Int32("number").SchemaType(map[string]string{dialect.Postgres: "integer"}),
		field.JSON("document", json.RawMessage{}).SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.Bytes("case_yaml").SchemaType(map[string]string{dialect.Postgres: "bytea"}).Optional(),
		field.Int64("parent_version_id").SchemaType(map[string]string{dialect.Postgres: "bigint"}).Optional(),
		field.Int64("update_id").SchemaType(map[string]string{dialect.Postgres: "bigint"}).Optional(),
		field.Text("origin").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Text("author").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Text("summary").SchemaType(map[string]string{dialect.Postgres: "text"}).Annotations(entsql.DefaultExpr("''")),
		field.Text("reason").SchemaType(map[string]string{dialect.Postgres: "text"}).Annotations(entsql.DefaultExpr("''")),
		field.JSON("mandatory_sections", json.RawMessage{}).SchemaType(map[string]string{dialect.Postgres: "jsonb"}).Annotations(entsql.DefaultExpr("'[]'::jsonb")),
		field.Time("expires_at").SchemaType(map[string]string{dialect.Postgres: "timestamptz"}).Optional(),
		field.Time("created_at").SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (SkillVersion) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("layer_id", "number").Unique().StorageKey("skill_versions_uq1"),
	}
}

func (SkillVersion) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "skill_versions"},
		entsql.Checks(map[string]string{
			"skill_versions_origin_check": "origin IN ('IMPORT', 'INITIAL', 'PUBLICATION', 'DECISION', 'MANDATORY', 'DEADLINE', 'ROLLBACK')",
		}),
		annotation.Constraints{
			ForeignKeys: []annotation.ForeignKey{
				{Symbol: "skill_versions_fk1", Columns: []string{"layer_id"}, Table: "skill_layers", Target: []string{"id"}},
				{Symbol: "skill_versions_fk2", Columns: []string{"parent_version_id"}, Table: "skill_versions", Target: []string{"id"}},
				{Symbol: "skill_versions_fk3", Columns: []string{"update_id"}, Table: "skill_updates", Target: []string{"id"}},
			},
		},
	}
}
