package schema

import (
	"encoding/json"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Pedido é um pedido de um contato (telefone) dentro da administradora. dados guarda o
// pedido inteiro serializado; as demais colunas existem para filtrar sem abrir o JSON.
type Pedido struct {
	ent.Schema
}

func (Pedido) Fields() []ent.Field {
	horario := map[string]string{dialect.Postgres: "timestamptz"}
	return []ent.Field{
		field.String("id").StorageKey("protocolo").NotEmpty().Immutable(),
		field.String("administradora_id").NotEmpty(),
		field.String("condominio_id").Default(""),
		field.String("telefone").NotEmpty(),
		field.String("caso"),
		field.String("estado"),
		field.JSON("dados", json.RawMessage{}),
		field.Time("criado_em").SchemaType(horario),
		field.Time("atualizado_em").SchemaType(horario),
	}
}

func (Pedido) Edges() []ent.Edge {
	return []ent.Edge{
		// CASCADE: remover o pedido leva junto o histórico dele.
		edge.To("eventos", PedidoEvento.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}

func (Pedido) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("administradora_id", "telefone"),
	}
}

func (Pedido) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "pedidos"}}
}
