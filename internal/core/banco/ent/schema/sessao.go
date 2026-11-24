package schema

import (
	"encoding/json"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Sessao é uma conversa do ATENDE serializada (turnos e histórico do Eino em dados).
// administradora_id e condominio_id ficam vazios até a conversa resolver o condomínio;
// por isso não são chaves estrangeiras.
type Sessao struct {
	ent.Schema
}

func (Sessao) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").NotEmpty().Immutable(),
		field.String("administradora_id").Default(""),
		field.String("condominio_id").Default(""),
		field.String("protocolo").Default(""),
		field.Time("atualizada_em"),
		field.JSON("dados", json.RawMessage{}),
	}
}

// Indexes: a listagem do Console filtra por administradora, mais recentes primeiro.
func (Sessao) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("administradora_id", "atualizada_em"),
		index.Fields("atualizada_em"),
	}
}

func (Sessao) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "sessoes"}}
}
