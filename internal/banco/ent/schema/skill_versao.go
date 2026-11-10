package schema

import (
	"encoding/json"
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// SkillVersao é uma versão publicada de uma camada: o documento composto inteiro daquele
// nível, mais a versão de cima sobre a qual foi feita. Só se acrescenta: versão gravada nunca
// é reescrita, para o histórico mostrar a camada como era.
type SkillVersao struct {
	ent.Schema
}

func (SkillVersao) Fields() []ent.Field {
	horario := map[string]string{dialect.Postgres: "timestamptz"}
	return []ent.Field{
		field.Int("camada_id").Immutable(),
		field.Int("numero").Immutable(),
		field.JSON("documento", json.RawMessage{}).Immutable(),
		// caso.yaml cru: só a versão da base o guarda, para o turno não reler o disco.
		field.Bytes("caso_yaml").Optional().Immutable(),
		field.Int("acima_id").Optional().Nillable().Immutable(),
		field.Int("atualizacao_id").Optional().Nillable().Immutable(),
		field.Enum("origem").
			Values("importacao", "inicial", "publicacao", "decisao", "obrigatorio", "prazo", "volta").Immutable(),
		field.String("autor").NotEmpty().Immutable(),
		field.Text("resumo").Default("").Immutable(),
		field.Text("motivo").Default("").Immutable(),
		field.JSON("obrigatorios", []string{}).Immutable(),
		field.Time("encerra_em").SchemaType(horario).Optional().Nillable().Immutable(),
		field.Time("criado_em").SchemaType(horario).Default(time.Now).Immutable(),
	}
}

func (SkillVersao) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("camada", SkillCamada.Type).Ref("versoes").Field("camada_id").Unique().Required().Immutable(),
	}
}

func (SkillVersao) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("camada_id", "numero").Unique(),
	}
}

func (SkillVersao) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "skill_versoes"}}
}
