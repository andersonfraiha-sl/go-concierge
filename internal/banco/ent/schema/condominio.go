package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// Condominio liga o condomínio aos sistemas (account do Nexus, company do Gruvi, tenant do
// ATENDE). Sem administradora, só o Backdoor o vê.
type Condominio struct {
	ent.Schema
}

func (Condominio) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").NotEmpty().Immutable(),
		field.String("nome").NotEmpty(),
		field.String("administradora_id").Optional().Nillable(),
		field.String("account").Default(""),
		field.String("company_id").Default(""),
		field.String("tenant").Default(""),
	}
}

func (Condominio) Edges() []ent.Edge {
	// CASCADE: remover o condomínio leva junto tudo que era dele.
	cascata := entsql.OnDelete(entsql.Cascade)
	return []ent.Edge{
		edge.From("administradora", Administradora.Type).Ref("condominios").Field("administradora_id").Unique(),
		edge.To("habilitacoes", Habilitacao.Type).Annotations(cascata),
		edge.To("payloads", Payload.Type).Annotations(cascata),
		edge.To("orientacoes", Orientacao.Type).Annotations(cascata),
	}
}
