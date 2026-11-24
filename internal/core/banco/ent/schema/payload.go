package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Payload são as definições de um recurso do Nexus num condomínio; condominio_id NULL é o
// padrão ("*").
type Payload struct {
	ent.Schema
}

func (Payload) Fields() []ent.Field {
	return []ent.Field{
		field.String("scope_type"),
		field.String("company_id").Optional(),
		field.String("condominio_id").StorageKey("condo_id").Optional().Nillable(),
		field.String("recurso").StorageKey("resource_key").NotEmpty(),
		field.JSON("dados", map[string]any{}).StorageKey("data"),
	}
}

func (Payload) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("condominio", Condominio.Type).Ref("payloads").Field("condominio_id").Unique(),
	}
}

// Indexes: mesmo desenho de Habilitacao (dois índices parciais no lugar do coalesce).
func (Payload) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("condominio_id", "recurso").Unique().Annotations(entsql.IndexWhere("condominio_id IS NOT NULL")),
		index.Fields("recurso").Unique().Annotations(entsql.IndexWhere("condominio_id IS NULL")),
	}
}

func (Payload) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "payloads"}}
}
