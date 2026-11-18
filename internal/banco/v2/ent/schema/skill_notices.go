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

type SkillNotice struct{ ent.Schema }

func (SkillNotice) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").SchemaType(map[string]string{dialect.Postgres: "bigint"}),
		field.Int64("layer_id").SchemaType(map[string]string{dialect.Postgres: "bigint"}),
		field.Int64("version_id").SchemaType(map[string]string{dialect.Postgres: "bigint"}),
		field.Text("reason").SchemaType(map[string]string{dialect.Postgres: "text"}).Annotations(entsql.DefaultExpr("''")),
		field.JSON("sections", json.RawMessage{}).SchemaType(map[string]string{dialect.Postgres: "jsonb"}).Annotations(entsql.DefaultExpr("'[]'::jsonb")),
		field.Time("expires_at").SchemaType(map[string]string{dialect.Postgres: "timestamptz"}).Optional(),
		field.Text("acknowledged_by").SchemaType(map[string]string{dialect.Postgres: "text"}).Annotations(entsql.DefaultExpr("''")),
		field.Time("acknowledged_at").SchemaType(map[string]string{dialect.Postgres: "timestamptz"}).Optional(),
		field.Time("created_at").SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (SkillNotice) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("layer_id", "acknowledged_at").StorageKey("skill_notices_ix1"),
	}
}

func (SkillNotice) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "skill_notices"},
		annotation.Constraints{
			ForeignKeys: []annotation.ForeignKey{
				{Symbol: "skill_notices_fk1", Columns: []string{"layer_id"}, Table: "skill_layers", Target: []string{"id"}},
				{Symbol: "skill_notices_fk2", Columns: []string{"version_id"}, Table: "skill_versions", Target: []string{"id"}},
			},
		},
	}
}
