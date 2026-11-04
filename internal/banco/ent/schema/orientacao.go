package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Orientacao é o texto livre da administradora para o concierge; condominio_id NULL é o
// padrão ("*").
type Orientacao struct {
	ent.Schema
}

func (Orientacao) Fields() []ent.Field {
	return []ent.Field{
		field.String("condominio_id").Optional().Nillable(),
		field.Text("geral").Default(""),
		field.JSON("por_caso", map[string]string{}),
	}
}

func (Orientacao) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("condominio", Condominio.Type).Ref("orientacoes").Field("condominio_id").Unique(),
	}
}

// Indexes: uma orientação por condomínio. A do padrão (NULL) não tem coluna que a torne
// única num índice parcial; quem a grava apaga e recria numa transação serializável.
func (Orientacao) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("condominio_id").Unique().Annotations(entsql.IndexWhere("condominio_id IS NOT NULL")),
	}
}

func (Orientacao) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "orientacoes"}}
}
