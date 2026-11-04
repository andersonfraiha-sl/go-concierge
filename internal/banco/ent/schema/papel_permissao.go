package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// PapelPermissao é uma permissão própria de um papel.
type PapelPermissao struct {
	ent.Schema
}

func (PapelPermissao) Fields() []ent.Field {
	return []ent.Field{
		field.String("papel_id").NotEmpty(),
		field.String("permissao").NotEmpty(),
	}
}

func (PapelPermissao) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("papel", Papel.Type).Ref("permissoes").Field("papel_id").Unique().Required(),
	}
}

func (PapelPermissao) Indexes() []ent.Index {
	return []ent.Index{index.Fields("papel_id", "permissao").Unique()}
}

func (PapelPermissao) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "papel_permissoes"}}
}
