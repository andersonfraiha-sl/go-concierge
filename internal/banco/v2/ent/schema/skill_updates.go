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

type SkillUpdate struct{ ent.Schema }

func (SkillUpdate) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").SchemaType(map[string]string{dialect.Postgres: "bigint"}),
		field.Int64("layer_id").SchemaType(map[string]string{dialect.Postgres: "bigint"}),
		field.Int64("from_version_id").SchemaType(map[string]string{dialect.Postgres: "bigint"}),
		field.Int64("to_version_id").SchemaType(map[string]string{dialect.Postgres: "bigint"}),
		field.Text("status").SchemaType(map[string]string{dialect.Postgres: "text"}).Annotations(entsql.DefaultExpr("'PENDING'")),
		field.Time("deadline_at").SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Bool("deadline_applied").Annotations(entsql.DefaultExpr("false")),
		field.JSON("decisions", json.RawMessage{}).SchemaType(map[string]string{dialect.Postgres: "jsonb"}).Optional(),
		field.Text("decided_by").SchemaType(map[string]string{dialect.Postgres: "text"}).Annotations(entsql.DefaultExpr("''")),
		field.Time("decided_at").SchemaType(map[string]string{dialect.Postgres: "timestamptz"}).Optional(),
		field.Time("created_at").SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (SkillUpdate) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("layer_id", "status").StorageKey("skill_updates_ix1"),
		index.Fields("status", "deadline_at").StorageKey("skill_updates_ix2"),
	}
}

func (SkillUpdate) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "skill_updates"},
		entsql.Checks(map[string]string{
			"skill_updates_status_check": "status IN ('PENDING', 'ACCEPTED', 'REJECTED', 'PARTIAL', 'SUPERSEDED')",
		}),
		annotation.Constraints{
			ForeignKeys: []annotation.ForeignKey{
				{Symbol: "skill_updates_fk1", Columns: []string{"layer_id"}, Table: "skill_layers", Target: []string{"id"}},
				{Symbol: "skill_updates_fk2", Columns: []string{"from_version_id"}, Table: "skill_versions", Target: []string{"id"}},
				{Symbol: "skill_updates_fk3", Columns: []string{"to_version_id"}, Table: "skill_versions", Target: []string{"id"}},
			},
		},
	}
}
