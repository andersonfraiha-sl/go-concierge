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

type MemoryFact struct{ ent.Schema }

func (MemoryFact) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").SchemaType(map[string]string{dialect.Postgres: "bigint"}),
		field.Text("company_id").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Text("phone_number").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Int32("condo_id").SchemaType(map[string]string{dialect.Postgres: "integer"}).Optional(),
		field.Text("key").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Text("text").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Text("source").SchemaType(map[string]string{dialect.Postgres: "text"}).Annotations(entsql.DefaultExpr("''")),
		field.Time("occurred_at").SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Float32("confidence").SchemaType(map[string]string{dialect.Postgres: "real"}).Annotations(entsql.DefaultExpr("0")),
	}
}

func (MemoryFact) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("company_id", "phone_number", "condo_id", "key").Unique().StorageKey("memory_facts_uq1"),
		index.Fields("company_id", "phone_number", "condo_id").StorageKey("memory_facts_ix1"),
	}
}

func (MemoryFact) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "memory_facts"},
		entsql.Checks(map[string]string{
			"memory_facts_company_id_check": "company_id IS NULL OR company_id ~ '[^[:space:]]'",
		}),
		annotation.Constraints{
			ForeignKeys: []annotation.ForeignKey{
				{Symbol: "memory_facts_fk1", Columns: []string{"company_id", "phone_number"}, Table: "contact_memories", Target: []string{"company_id", "phone_number"}},
			},
			NullsNotDistinct: []string{"memory_facts_uq1"},
		},
	}
}
