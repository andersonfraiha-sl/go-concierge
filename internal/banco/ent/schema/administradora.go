package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// Administradora é a cliente da Superlógica dona de um conjunto de condomínios.
type Administradora struct {
	ent.Schema
}

func (Administradora) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").NotEmpty().Immutable(),
		field.String("nome").NotEmpty(),
	}
}

func (Administradora) Edges() []ent.Edge {
	return []ent.Edge{
		// RESTRICT: o domínio recusa remover administradora com condomínio; o banco garante.
		edge.To("condominios", Condominio.Type).Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("skill_camadas", SkillCamada.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}
