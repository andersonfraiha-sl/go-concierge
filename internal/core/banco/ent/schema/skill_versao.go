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
		field.Int("camada_id").StorageKey("layer_id").Immutable(),
		field.Int("numero").StorageKey("number").Immutable(),
		field.JSON("documento", json.RawMessage{}).StorageKey("document").Immutable(),
		// caso.yaml cru: só a versão da base o guarda, para o turno não reler o disco.
		field.Bytes("caso_yaml").StorageKey("case_yaml").Optional().Immutable(),
		field.Int("acima_id").StorageKey("parent_version_id").Optional().Nillable().Immutable(),
		field.Int("atualizacao_id").StorageKey("update_id").Optional().Nillable().Immutable(),
		field.Enum("origem").StorageKey("origin").
			NamedValues("Importacao", "IMPORT", "Inicial", "INITIAL", "Publicacao", "PUBLICATION", "Decisao", "DECISION", "Obrigatorio", "MANDATORY", "Prazo", "DEADLINE", "Volta", "ROLLBACK").Immutable(),
		field.String("autor").StorageKey("author").NotEmpty().Immutable(),
		field.Text("resumo").StorageKey("summary").Default("").Immutable(),
		field.Text("motivo").StorageKey("reason").Default("").Immutable(),
		field.JSON("obrigatorios", []string{}).StorageKey("mandatory_sections").Immutable(),
		field.Time("encerra_em").StorageKey("expires_at").SchemaType(horario).Optional().Nillable().Immutable(),
		field.Time("criado_em").StorageKey("created_at").SchemaType(horario).Default(time.Now).Immutable(),
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
	return []schema.Annotation{entsql.Annotation{Table: "skill_versions"}}
}
