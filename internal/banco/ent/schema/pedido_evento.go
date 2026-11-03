package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// PedidoEvento é um evento do histórico de um pedido, na ordem seq. Só se acrescenta: um
// evento gravado nunca é reescrito.
type PedidoEvento struct {
	ent.Schema
}

func (PedidoEvento) Fields() []ent.Field {
	return []ent.Field{
		field.String("protocolo").NotEmpty().Immutable(),
		field.Int("seq").Immutable(),
		field.String("tipo").Immutable(),
		field.Text("texto").Immutable(),
		field.String("por").Default("").Immutable(),
		field.Time("quando").SchemaType(map[string]string{dialect.Postgres: "timestamptz"}).Immutable(),
	}
}

func (PedidoEvento) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("pedido", Pedido.Type).Ref("eventos").Field("protocolo").Unique().Required().Immutable(),
	}
}

func (PedidoEvento) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("protocolo", "seq").Unique(),
	}
}

func (PedidoEvento) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "pedido_eventos"}}
}
