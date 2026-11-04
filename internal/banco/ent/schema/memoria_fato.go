package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// MemoriaFato é um fato durável sobre um contato (telefone) dentro da administradora. A
// mesma chave substitui o fato anterior; condominio_id registra onde ele foi aprendido.
type MemoriaFato struct {
	ent.Schema
}

func (MemoriaFato) Fields() []ent.Field {
	return []ent.Field{
		field.String("administradora_id").NotEmpty(),
		field.String("telefone").NotEmpty(),
		field.String("chave").NotEmpty(),
		field.String("condominio_id").Default(""),
		field.Text("texto"),
		field.String("fonte").Default(""),
		field.Time("quando").SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Float32("confianca").SchemaType(map[string]string{dialect.Postgres: "real"}).Default(0),
	}
}

func (MemoriaFato) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("administradora_id", "telefone", "chave").Unique(),
	}
}

func (MemoriaFato) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "memoria_fatos"},
		// o GravarFatos (sqlc) não passa pelos validadores do ent: o banco recusa o vazio
		entsql.Checks(map[string]string{
			"memoria_fatos_administradora_preenchida": "administradora_id <> ''",
			"memoria_fatos_telefone_preenchido":       "telefone <> ''",
			"memoria_fatos_chave_preenchida":          "chave <> ''",
		}),
	}
}
