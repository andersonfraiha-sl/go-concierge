package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Concessao libera uma permissão extra a um papel dentro de uma administradora.
type Concessao struct {
	ent.Schema
}

func (Concessao) Fields() []ent.Field {
	return []ent.Field{
		field.String("administradora_id").NotEmpty(),
		field.String("papel_id").NotEmpty(),
		field.String("permissao").NotEmpty(),
	}
}

func (Concessao) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("administradora", Administradora.Type).Field("administradora_id").Unique().Required(),
		edge.From("papel", Papel.Type).Ref("concessoes").Field("papel_id").Unique().Required(),
	}
}

func (Concessao) Indexes() []ent.Index {
	return []ent.Index{index.Fields("administradora_id", "papel_id", "permissao").Unique()}
}

func (Concessao) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "concessoes"}}
}
