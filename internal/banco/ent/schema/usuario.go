package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// Usuario é quem opera o concierge; o que pode fazer vem das atribuições.
type Usuario struct {
	ent.Schema
}

func (Usuario) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").NotEmpty().Immutable(),
		field.String("nome").StorageKey("name").NotEmpty(),
	}
}

func (Usuario) Edges() []ent.Edge {
	return []ent.Edge{edge.To("atribuicoes", Atribuicao.Type)}
}

func (Usuario) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "users"}}
}
