package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

// CatalogoArquivo é um arquivo do catálogo (agentes/** e nexus/**). O disco é só a cópia de
// trabalho, refeita desta tabela a cada boot; o sha256 deixa comparar sem trazer os bytes.
type CatalogoArquivo struct {
	ent.Schema
}

func (CatalogoArquivo) Fields() []ent.Field {
	return []ent.Field{
		// o id do ent é o caminho: a chave primária da tabela se chama caminho
		field.String("id").StorageKey("caminho").NotEmpty().Immutable(),
		field.Bytes("conteudo"),
		field.String("sha256").NotEmpty(),
		field.Time("atualizado_em").SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (CatalogoArquivo) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "catalogo_arquivos"},
		entsql.Checks(map[string]string{
			"catalogo_arquivos_caminho_na_raiz": "caminho LIKE 'agentes/%' OR caminho LIKE 'nexus/%'",
		}),
	}
}
