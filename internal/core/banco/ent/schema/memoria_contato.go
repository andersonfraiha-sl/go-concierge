package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// MemoriaContato é o cabeçalho da memória de um contato (telefone) na administradora: o nome
// e quando ela mudou. Existe mesmo sem nenhum fato.
type MemoriaContato struct {
	ent.Schema
}

func (MemoriaContato) Fields() []ent.Field {
	return []ent.Field{
		field.String("administradora_id").NotEmpty(),
		field.String("telefone").NotEmpty(),
		field.String("nome").Default(""),
		field.Time("atualizada_em").SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (MemoriaContato) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("administradora_id", "telefone").Unique(),
	}
}

func (MemoriaContato) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "memoria_contatos"},
		entsql.Checks(map[string]string{
			"memoria_contatos_administradora_preenchida": "administradora_id <> ''",
			"memoria_contatos_telefone_preenchido":       "telefone <> ''",
		}),
	}
}
