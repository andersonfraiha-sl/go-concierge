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

// SkillAtualizacao é a atualização de uma camada de baixo: a versão de cima passou de de_id
// para para_id e a camada ainda decide o que aceita. de_id e para_id são versões da camada de
// cima; o prazo é a data em que o sistema aplica sozinho o que a pessoa nunca mexeu.
type SkillAtualizacao struct {
	ent.Schema
}

func (SkillAtualizacao) Fields() []ent.Field {
	horario := map[string]string{dialect.Postgres: "timestamptz"}
	return []ent.Field{
		field.Int("camada_id").StorageKey("layer_id").Immutable(),
		field.Int("de_id").StorageKey("from_version_id").Immutable(),
		field.Int("para_id").StorageKey("to_version_id").Immutable(),
		field.Enum("estado").StorageKey("status").NamedValues("Pendente", "PENDING", "Aceita", "ACCEPTED", "Recusada", "REJECTED", "Parcial", "PARTIAL", "Substituida", "SUPERSEDED").Default("PENDING"),
		field.Time("prazo_em").StorageKey("deadline_at").SchemaType(horario),
		field.Bool("prazo_aplicado").StorageKey("deadline_applied").Default(false),
		field.JSON("decisoes", json.RawMessage{}).StorageKey("decisions").Optional(),
		field.String("decidida_por").StorageKey("decided_by").Default(""),
		field.Time("decidida_em").StorageKey("decided_at").SchemaType(horario).Optional().Nillable(),
		field.Time("criado_em").StorageKey("created_at").SchemaType(horario).Default(time.Now).Immutable(),
	}
}

func (SkillAtualizacao) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("camada", SkillCamada.Type).Ref("atualizacoes").Field("camada_id").Unique().Required().Immutable(),
		// CASCADE: as versões de cima são das camadas dela; sem elas a atualização não tem o que comparar.
		edge.To("de", SkillVersao.Type).Field("de_id").Unique().Required().Immutable().
			Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("para", SkillVersao.Type).Field("para_id").Unique().Required().Immutable().
			Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}

func (SkillAtualizacao) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("camada_id", "estado"),
		index.Fields("estado", "prazo_em"),
	}
}

func (SkillAtualizacao) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "skill_updates"}}
}
