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
	"go-concierge/internal/banco/v2/ent/annotation"
)

type Channel struct{ ent.Schema }

func (Channel) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).SchemaType(map[string]string{dialect.Postgres: "uuid"}),
		field.Text("company_id").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Int32("condo_id").SchemaType(map[string]string{dialect.Postgres: "integer"}).Optional(),
		field.Text("name").SchemaType(map[string]string{dialect.Postgres: "text"}).Optional(),
		field.Text("service_profile").SchemaType(map[string]string{dialect.Postgres: "text"}).Optional(),
		field.Text("channel_type").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Text("provider").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Text("endpoint_identifier").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Text("external_channel_id").SchemaType(map[string]string{dialect.Postgres: "text"}).Optional(),
		field.Text("feature_mode").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Bool("enabled"),
		field.Text("root_agent").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Text("api_base_url").SchemaType(map[string]string{dialect.Postgres: "text"}).Optional(),
		field.Text("api_key").SchemaType(map[string]string{dialect.Postgres: "text"}).Optional(),
		field.Text("webhook_secret").SchemaType(map[string]string{dialect.Postgres: "text"}).Optional(),
		field.JSON("config_jsonb", json.RawMessage{}).SchemaType(map[string]string{dialect.Postgres: "jsonb"}).Optional(),
		field.Time("created_at").SchemaType(map[string]string{dialect.Postgres: "timestamptz"}).Annotations(entsql.DefaultExpr("now()")),
		field.Time("updated_at").SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (Channel) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("company_id", "id").Unique().StorageKey("channels_uq1"),
		index.Fields("company_id", "provider", "endpoint_identifier").Unique().StorageKey("channels_uq2"),
		index.Fields("provider", "channel_type", "external_channel_id", "endpoint_identifier").Unique().StorageKey("channels_uq3"),
		index.Fields("company_id", "enabled").StorageKey("channels_ix1"),
		index.Fields("company_id", "condo_id").StorageKey("channels_ix2"),
	}
}

func (Channel) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "channels"},
		entsql.Checks(map[string]string{
			"channels_company_id_check":   "company_id IS NULL OR company_id ~ '[^[:space:]]'",
			"channels_provider_check":     "provider ~ '[^[:space:]]'",
			"channels_channel_type_check": "channel_type IN ('WHATSAPP', 'WEB', 'GRUVI')",
			"channels_feature_mode_check": "feature_mode IN ('INHERIT', 'CUSTOM', 'EXPLICIT')",
		}),
		annotation.Constraints{
			NullsNotDistinct: []string{"channels_uq3"},
		},
	}
}
