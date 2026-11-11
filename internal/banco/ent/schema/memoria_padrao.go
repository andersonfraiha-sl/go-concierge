package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// MemoriaPadrao é um padrão de uso do contato (telefone): quanto ele pede um caso, num
// condomínio e numa unidade. O valor é um contador com decaimento, gravado como estava em
// atualizado_em; suprimido_ate silencia a oferta depois de uma recusa.
type MemoriaPadrao struct {
	ent.Schema
}

func (MemoriaPadrao) Fields() []ent.Field {
	horario := map[string]string{dialect.Postgres: "timestamptz"}
	return []ent.Field{
		field.String("administradora_id").NotEmpty(),
		field.String("telefone").NotEmpty(),
		field.String("caso").NotEmpty(),
		field.String("caso_nome").Default(""),
		field.String("condominio_id").NotEmpty(),
		field.String("unidade_id").Default(""),
		field.Float("valor"),
		field.Time("atualizado_em").SchemaType(horario),
		field.Time("suprimido_ate").SchemaType(horario).Optional().Nillable(),
	}
}

func (MemoriaPadrao) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("administradora_id", "telefone", "caso", "condominio_id", "unidade_id").Unique(),
	}
}

func (MemoriaPadrao) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "memoria_padroes"},
		// o GravarPadroes (sqlc) não passa pelos validadores do ent: o banco recusa o vazio
		entsql.Checks(map[string]string{
			"memoria_padroes_administradora_preenchida": "administradora_id <> ''",
			"memoria_padroes_telefone_preenchido":       "telefone <> ''",
			"memoria_padroes_caso_preenchido":           "caso <> ''",
			"memoria_padroes_condominio_preenchido":     "condominio_id <> ''",
		}),
	}
}
