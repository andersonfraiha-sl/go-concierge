package schema

import (
	"database/sql/driver"
	"fmt"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/jackc/pgx/v5/pgtype"
)

// Habilitacao de um caso de uso num condomínio; condominio_id NULL é o padrão ("*").
type Habilitacao struct {
	ent.Schema
}

func (Habilitacao) Fields() []ent.Field {
	return []ent.Field{
		field.String("condominio_id").Optional().Nillable(),
		field.String("caso").NotEmpty(),
		field.Bool("habilitado"),
		field.Other("papeis", Papeis{}).
			SchemaType(map[string]string{dialect.Postgres: "text[]"}).
			Default(Papeis{}),
	}
}

func (Habilitacao) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("condominio", Condominio.Type).Ref("habilitacoes").Field("condominio_id").Unique(),
	}
}

// Indexes: um caso por condomínio e um no padrão. O ent não declara índice de expressão
// (coalesce(condominio_id,'*')) nem NULLS NOT DISTINCT; dois índices parciais dão a mesma
// garantia e servem de alvo ao upsert.
func (Habilitacao) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("condominio_id", "caso").Unique().Annotations(entsql.IndexWhere("condominio_id IS NOT NULL")),
		index.Fields("caso").Unique().Annotations(entsql.IndexWhere("condominio_id IS NULL")),
	}
}

// Papeis é a coluna text[] dos papéis do morador. O driver database/sql do pgx entrega
// arrays como texto; o pgtype faz a conversão nos dois sentidos.
type Papeis []string

func (p *Papeis) Scan(origem any) error {
	if err := pgtype.NewMap().SQLScanner((*[]string)(p)).Scan(origem); err != nil {
		return fmt.Errorf("ler papeis: %w", err)
	}
	return nil
}

func (p Papeis) Value() (driver.Value, error) {
	if p == nil {
		p = Papeis{}
	}
	texto, err := pgtype.NewMap().Encode(pgtype.TextArrayOID, pgtype.TextFormatCode, []string(p), nil)
	if err != nil {
		return nil, fmt.Errorf("gravar papeis: %w", err)
	}
	return string(texto), nil
}

func (Habilitacao) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "habilitacoes"}}
}
