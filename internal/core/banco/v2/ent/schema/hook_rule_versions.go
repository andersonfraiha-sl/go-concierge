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
	"go-concierge/internal/core/banco/v2/ent/annotation"
)

type HookRuleVersion struct{ ent.Schema }

func (HookRuleVersion) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").SchemaType(map[string]string{dialect.Postgres: "bigint"}),
		field.UUID("rule_id", uuid.UUID{}).SchemaType(map[string]string{dialect.Postgres: "uuid"}),
		field.Int32("version").SchemaType(map[string]string{dialect.Postgres: "integer"}),
		field.JSON("definition", json.RawMessage{}).SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.Text("updated_by").SchemaType(map[string]string{dialect.Postgres: "text"}).Annotations(entsql.DefaultExpr("''")),
		field.Time("updated_at").SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (HookRuleVersion) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("rule_id", "version").Unique().StorageKey("hook_rule_versions_uq1"),
	}
}

func (HookRuleVersion) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "hook_rule_versions"},
		annotation.Constraints{
			ForeignKeys: []annotation.ForeignKey{
				{Symbol: "hook_rule_versions_fk1", Columns: []string{"rule_id"}, Table: "hook_rules", Target: []string{"id"}},
			},
		},
	}
}
