package schema

import (
	"encoding/json"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// SkillCamada é uma camada de uma skill: a base da Superlógica, a da administradora ou a do
// condomínio. Guarda o rascunho (nunca lido pelo turno) e aponta para a versão publicada mais
// recente (cabeca_id) e para a versão de cima sobre a qual ela foi feita (acima_id). Os dois
// ponteiros não são chaves estrangeiras: camada e versão se referenciam, e a integridade fica
// com o repositório.
type SkillCamada struct {
	ent.Schema
}

func (SkillCamada) Fields() []ent.Field {
	return []ent.Field{
		field.String("skill").NotEmpty().Immutable(),
		field.Enum("tipo").Values("base", "administradora", "condominio").Immutable(),
		field.String("administradora_id").Optional().Nillable().Immutable(),
		field.String("condominio_id").Optional().Nillable().Immutable(),
		field.Int("cabeca_id").Optional().Nillable(),
		field.Int("acima_id").Optional().Nillable(),
		field.JSON("rascunho", json.RawMessage{}).Optional(),
		field.String("rascunho_versao").Default(""),
	}
}

func (SkillCamada) Edges() []ent.Edge {
	cascata := entsql.OnDelete(entsql.Cascade)
	return []ent.Edge{
		// CASCADE: remover a administradora ou o condomínio leva junto as camadas dele.
		edge.From("administradora", Administradora.Type).Ref("skill_camadas").
			Field("administradora_id").Unique().Immutable(),
		edge.From("condominio", Condominio.Type).Ref("skill_camadas").
			Field("condominio_id").Unique().Immutable(),
		edge.To("versoes", SkillVersao.Type).Annotations(cascata),
		edge.To("atualizacoes", SkillAtualizacao.Type).Annotations(cascata),
		edge.To("avisos", SkillAviso.Type).Annotations(cascata),
	}
}

// Indexes: uma camada por skill e dono. O ent não declara índice de expressão, e dono
// opcional pede um índice parcial por tipo. O predicado está na forma que o Postgres devolve
// para enum em varchar; com outra grafia o Atlas vê diferença que não existe.
func (SkillCamada) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("skill").Unique().Annotations(entsql.IndexWhere("(tipo)::text = 'base'::text")),
		index.Fields("skill", "administradora_id").Unique().Annotations(entsql.IndexWhere("(tipo)::text = 'administradora'::text")),
		index.Fields("skill", "condominio_id").Unique().Annotations(entsql.IndexWhere("(tipo)::text = 'condominio'::text")),
	}
}

func (SkillCamada) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "skill_camadas"},
		entsql.Checks(map[string]string{
			"skill_camadas_dono_do_tipo": "(tipo = 'base' AND administradora_id IS NULL AND condominio_id IS NULL)" +
				" OR (tipo = 'administradora' AND administradora_id IS NOT NULL AND condominio_id IS NULL)" +
				" OR (tipo = 'condominio' AND condominio_id IS NOT NULL AND administradora_id IS NULL)",
		}),
	}
}
