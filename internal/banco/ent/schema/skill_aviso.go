package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// SkillAviso é o aviso a quem tinha personalização num trecho que uma versão obrigatória
// trocou, ou numa skill que a base encerrou: o motivo, os trechos (por rótulo) e a data em
// que a skill para de funcionar. Fica até alguém dar ciente.
type SkillAviso struct {
	ent.Schema
}

func (SkillAviso) Fields() []ent.Field {
	horario := map[string]string{dialect.Postgres: "timestamptz"}
	return []ent.Field{
		field.Int("camada_id").Immutable(),
		field.Int("versao_id").Immutable(),
		field.Text("motivo").Default("").Immutable(),
		field.JSON("trechos", []string{}).Immutable(),
		field.Time("encerra_em").SchemaType(horario).Optional().Nillable().Immutable(),
		field.String("ciente_por").Default(""),
		field.Time("ciente_em").SchemaType(horario).Optional().Nillable(),
		field.Time("criado_em").SchemaType(horario).Default(time.Now).Immutable(),
	}
}

func (SkillAviso) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("camada", SkillCamada.Type).Ref("avisos").Field("camada_id").Unique().Required().Immutable(),
		edge.To("versao", SkillVersao.Type).Field("versao_id").Unique().Required().Immutable().
			Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}

func (SkillAviso) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("camada_id", "ciente_em"),
	}
}

func (SkillAviso) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "skill_avisos"}}
}
