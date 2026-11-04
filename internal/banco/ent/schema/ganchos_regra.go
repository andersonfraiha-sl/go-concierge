package schema

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// GanchosRegra é o estado atual de uma regra declarativa de gancho; cada escrita confirmada
// soma uma linha em GanchosRegraVersao, então o histórico mostra a regra como ela era.
type GanchosRegra struct {
	ent.Schema
}

func (GanchosRegra) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.New()).Default(uuid.New).Immutable(),
		field.Int("versao"),
		field.String("nome").NotEmpty(),
		field.String("evento").NotEmpty(),
		field.String("administradora_id").Default(""),
		field.Other("condominios", GanchosCondominios{}).
			SchemaType(map[string]string{dialect.Postgres: "text[]"}).
			Default(GanchosCondominios{}),
		field.Bool("ativa"),
		field.JSON("definicao", json.RawMessage{}),
		field.String("alterada_por").Default(""),
		field.Time("alterada_em").SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (GanchosRegra) Edges() []ent.Edge {
	return []ent.Edge{
		// CASCADE: a versão só existe para contar a história da regra dela.
		edge.To("versoes", GanchosRegraVersao.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}

// Indexes: o cache compila o índice por evento, olhando só as ativas.
func (GanchosRegra) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("evento", "ativa"),
	}
}

func (GanchosRegra) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "ganchos_regras"}}
}

// GanchosCondominios é a coluna text[] dos condomínios alvo da regra; vazia vale para todos
// os condomínios da administradora. Mesmo padrão de Papeis em habilitacao.go: o driver
// database/sql do pgx entrega arrays como texto, o pgtype converte nos dois sentidos.
type GanchosCondominios []string

func (c *GanchosCondominios) Scan(origem any) error {
	if err := pgtype.NewMap().SQLScanner((*[]string)(c)).Scan(origem); err != nil {
		return fmt.Errorf("ler condominios: %w", err)
	}
	return nil
}

func (c GanchosCondominios) Value() (driver.Value, error) {
	if c == nil {
		c = GanchosCondominios{}
	}
	texto, err := pgtype.NewMap().Encode(pgtype.TextArrayOID, pgtype.TextFormatCode, []string(c), nil)
	if err != nil {
		return nil, fmt.Errorf("gravar condominios: %w", err)
	}
	return string(texto), nil
}
