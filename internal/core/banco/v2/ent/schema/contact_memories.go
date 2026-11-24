package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type ContactMemory struct{ ent.Schema }

func (ContactMemory) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").SchemaType(map[string]string{dialect.Postgres: "bigint"}),
		field.Text("company_id").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Text("phone_number").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Text("name").SchemaType(map[string]string{dialect.Postgres: "text"}).Annotations(entsql.DefaultExpr("''")),
		field.Time("updated_at").SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (ContactMemory) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("company_id", "phone_number").Unique().StorageKey("contact_memories_uq1"),
	}
}

func (ContactMemory) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "contact_memories"},
		entsql.Checks(map[string]string{
			"contact_memories_company_id_check": "company_id IS NULL OR company_id ~ '[^[:space:]]'",
		}),
	}
}
