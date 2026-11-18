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
		field.String("usuario_id").StorageKey("user_id").NotEmpty(),
		field.String("papel_id").StorageKey("role_id").NotEmpty(),
		field.String("escopo_tipo").StorageKey("scope_type").NotEmpty(),
		field.String("escopo_id").StorageKey("condo_id").Optional(),
		field.String("administradora_id").StorageKey("company_id").Optional().Nillable(),
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
	return []schema.Annotation{entsql.Annotation{Table: "role_assignments"}}
}
