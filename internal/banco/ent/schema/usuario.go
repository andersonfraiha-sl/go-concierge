package schema

import (
	"entgo.io/ent"
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
		field.String("nome").NotEmpty(),
	}
}

func (Usuario) Edges() []ent.Edge {
	return []ent.Edge{edge.To("atribuicoes", Atribuicao.Type)}
}
