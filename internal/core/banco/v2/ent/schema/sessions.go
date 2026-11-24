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

type Session struct{ ent.Schema }

func (Session) Fields() []ent.Field {
	return []ent.Field{
		field.Text("id").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Text("company_id").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.UUID("channel_id", uuid.UUID{}).SchemaType(map[string]string{dialect.Postgres: "uuid"}),
		field.Text("connector").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Text("protocol").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Text("phone_number").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Int32("condo_id").SchemaType(map[string]string{dialect.Postgres: "integer"}).Optional(),
		field.JSON("data", json.RawMessage{}).SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.Time("updated_at").SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (Session) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("company_id", "id").Unique().StorageKey("sessions_uq1"),
		index.Fields("company_id", "channel_id", "connector", "protocol").Unique().StorageKey("sessions_uq2"),
		index.Fields("company_id", "phone_number", "updated_at").StorageKey("sessions_ix1"),
		index.Fields("company_id", "condo_id", "updated_at").StorageKey("sessions_ix2"),
	}
}

func (Session) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "sessions"},
		entsql.Checks(map[string]string{
			"sessions_company_id_check": "company_id IS NULL OR company_id ~ '[^[:space:]]'",
		}),
		annotation.Constraints{
			ForeignKeys: []annotation.ForeignKey{
				{Symbol: "sessions_fk1", Columns: []string{"company_id", "channel_id"}, Table: "channels", Target: []string{"company_id", "id"}},
			},
		},
	}
}
