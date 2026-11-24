package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"go-concierge/internal/banco/v2/ent/annotation"
)

type RequestEvent struct{ ent.Schema }

func (RequestEvent) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").SchemaType(map[string]string{dialect.Postgres: "bigint"}),
		field.Text("company_id").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Text("protocol").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Int32("sequence").SchemaType(map[string]string{dialect.Postgres: "integer"}),
		field.Text("type").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Text("text").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Text("actor").SchemaType(map[string]string{dialect.Postgres: "text"}).Annotations(entsql.DefaultExpr("''")),
		field.Time("occurred_at").SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (RequestEvent) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("company_id", "protocol", "sequence").Unique().StorageKey("request_events_uq1"),
	}
}

func (RequestEvent) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "request_events"},
		entsql.Checks(map[string]string{
			"request_events_company_id_check": "company_id IS NULL OR company_id ~ '[^[:space:]]'",
		}),
		annotation.Constraints{
			ForeignKeys: []annotation.ForeignKey{
				{Symbol: "request_events_fk1", Columns: []string{"company_id", "protocol"}, Table: "requests", Target: []string{"company_id", "protocol"}},
			},
		},
	}
}
