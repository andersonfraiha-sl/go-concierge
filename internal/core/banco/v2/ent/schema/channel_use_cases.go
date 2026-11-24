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

type ChannelUseCase struct{ ent.Schema }

func (ChannelUseCase) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).SchemaType(map[string]string{dialect.Postgres: "uuid"}),
		field.Text("company_id").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.UUID("channel_id", uuid.UUID{}).SchemaType(map[string]string{dialect.Postgres: "uuid"}),
		field.UUID("company_use_case_id", uuid.UUID{}).SchemaType(map[string]string{dialect.Postgres: "uuid"}),
		field.Bool("enabled"),
		field.JSON("config_jsonb", json.RawMessage{}).SchemaType(map[string]string{dialect.Postgres: "jsonb"}).Optional(),
		field.Time("created_at").SchemaType(map[string]string{dialect.Postgres: "timestamptz"}).Annotations(entsql.DefaultExpr("now()")),
		field.Time("updated_at").SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (ChannelUseCase) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("company_id", "channel_id", "company_use_case_id").Unique().StorageKey("channel_use_cases_uq1"),
	}
}

func (ChannelUseCase) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "channel_use_cases"},
		entsql.Checks(map[string]string{
			"channel_use_cases_company_id_check": "company_id IS NULL OR company_id ~ '[^[:space:]]'",
		}),
		annotation.Constraints{
			ForeignKeys: []annotation.ForeignKey{
				{Symbol: "channel_use_cases_fk1", Columns: []string{"company_id", "channel_id"}, Table: "channels", Target: []string{"company_id", "id"}},
				{Symbol: "channel_use_cases_fk2", Columns: []string{"company_id", "company_use_case_id"}, Table: "company_use_cases", Target: []string{"company_id", "id"}},
			},
		},
	}
}
