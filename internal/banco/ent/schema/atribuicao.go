package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Atribuicao dá um papel a um usuário num escopo (superlogica, administradora ou condomínio).
// escopo_id é vazio no escopo superlogica. Atribuição de condomínio guarda também a
// administradora dona dele no momento da atribuição: se o condomínio mudar de administradora,
// a atribuição deixa de valer.
type Atribuicao struct {
	ent.Schema
}

func (Atribuicao) Fields() []ent.Field {
	return []ent.Field{
		field.String("usuario_id").NotEmpty(),
		field.String("papel_id").NotEmpty(),
		field.String("escopo_tipo").NotEmpty(),
		field.String("escopo_id").Default(""),
		field.String("administradora_id").Optional().Nillable(),
	}
}

func (Atribuicao) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("usuario", Usuario.Type).Ref("atribuicoes").Field("usuario_id").Unique().Required(),
		edge.To("papel", Papel.Type).Field("papel_id").Unique().Required(),
		// RESTRICT: o check exige administradora_id nas atribuições de condomínio, então SET NULL violaria.
		edge.To("administradora", Administradora.Type).Field("administradora_id").Unique().
			Annotations(entsql.OnDelete(entsql.Restrict)),
	}
}

func (Atribuicao) Indexes() []ent.Index {
	return []ent.Index{index.Fields("usuario_id", "papel_id", "escopo_tipo", "escopo_id").Unique()}
}

func (Atribuicao) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{
		Table: "atribuicoes",
		Checks: map[string]string{
			"atribuicoes_escopo_tipo": "escopo_tipo IN ('superlogica', 'administradora', 'condominio')",
			// só o escopo superlogica dispensa o id; os outros precisam dele
			"atribuicoes_administradora_id": "(escopo_tipo = 'condominio') = (administradora_id IS NOT NULL)",
			"atribuicoes_escopo_id": "(escopo_tipo = 'superlogica') = (escopo_id = '')",
		},
	}}
}
