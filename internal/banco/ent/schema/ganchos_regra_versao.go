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
	"github.com/google/uuid"
)

// GanchosRegraVersao é uma versão gravada de GanchosRegra, na ordem de versao. Só se
// acrescenta: uma versão gravada nunca é reescrita, então a auditoria pode apontar para ela
// e mostrar a regra como era naquele momento.
type GanchosRegraVersao struct {
	ent.Schema
}

func (GanchosRegraVersao) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("regra_id", uuid.New()).Immutable(),
		field.Int("versao").Immutable(),
		field.JSON("definicao", json.RawMessage{}).Immutable(),
		field.String("alterada_por").Default("").Immutable(),
		field.Time("alterada_em").SchemaType(map[string]string{dialect.Postgres: "timestamptz"}).Immutable(),
	}
}

func (GanchosRegraVersao) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("regra", GanchosRegra.Type).Ref("versoes").Field("regra_id").Unique().Required().Immutable(),
	}
}

func (GanchosRegraVersao) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("regra_id", "versao").Unique(),
	}
}

func (GanchosRegraVersao) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "ganchos_regras_versoes"}}
}
