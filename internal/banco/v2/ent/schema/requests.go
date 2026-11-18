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

type Request struct{ ent.Schema }

func (Request) Fields() []ent.Field {
	return []ent.Field{
		field.Text("id").SchemaType(map[string]string{dialect.Postgres: "text"}).StorageKey("protocol"),
		field.Text("company_id").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Text("phone_number").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Int32("condo_id").SchemaType(map[string]string{dialect.Postgres: "integer"}).Optional(),
		field.UUID("company_use_case_id", uuid.UUID{}).SchemaType(map[string]string{dialect.Postgres: "uuid"}),
		field.Text("source_session_id").SchemaType(map[string]string{dialect.Postgres: "text"}).Optional(),
		field.Text("status").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.JSON("data", json.RawMessage{}).SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.Time("created_at").SchemaType(map[string]string{dialect.Postgres: "timestamptz"}).Annotations(entsql.DefaultExpr("now()")),
		field.Time("updated_at").SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (Request) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("company_id", "id").Unique().StorageKey("requests_uq1"),
		index.Fields("company_id", "phone_number", "condo_id").StorageKey("requests_ix1"),
		index.Fields("company_id", "status").StorageKey("requests_ix2"),
	}
}

func (Request) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "requests"},
		entsql.Checks(map[string]string{
			"requests_company_id_check": "company_id IS NULL OR company_id ~ '[^[:space:]]'",
			"requests_status_check":     "status IN ('COLLECTING', 'READY', 'AWAITING_CONFIRMATION', 'EXECUTING', 'COMPLETED', 'WITH_AGENT', 'CANCELED', 'FAILED')",
		}),
		annotation.Constraints{
			ForeignKeys: []annotation.ForeignKey{
				{Symbol: "requests_fk1", Columns: []string{"company_id", "company_use_case_id"}, Table: "company_use_cases", Target: []string{"company_id", "id"}},
				{Symbol: "requests_fk2", Columns: []string{"company_id", "source_session_id"}, Table: "sessions", Target: []string{"company_id", "id"}, SetNullColumns: []string{"source_session_id"}},
			},
		},
	}
}
