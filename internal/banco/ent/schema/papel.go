package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// Papel agrupa permissões e herda as de outro papel (herda_de).
type Papel struct {
	ent.Schema
}

func (Papel) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").NotEmpty().Immutable(),
		field.String("herda_de").Optional().Nillable(),
	}
}

func (Papel) Edges() []ent.Edge {
	return []ent.Edge{
		// RESTRICT: apagar um papel herdado não pode soltar a herança dos filhos em silêncio.
		edge.To("herda", Papel.Type).Field("herda_de").Unique().Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("permissoes", PapelPermissao.Type),
		edge.To("concessoes", Concessao.Type),
	}
}

func (Papel) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "papeis"}}
}
