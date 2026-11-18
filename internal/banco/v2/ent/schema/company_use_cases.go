package schema

import (
	"encoding/json"
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
	"go-concierge/internal/banco/v2/ent/annotation"
)

type CompanyUseCase struct{ ent.Schema }

func (CompanyUseCase) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).SchemaType(map[string]string{dialect.Postgres: "uuid"}),
		field.Text("company_id").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Text("use_case_id").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Bool("enabled"),
		field.Bool("default_available"),
		field.Other("roles", TextArray{}).SchemaType(map[string]string{dialect.Postgres: "text[]"}).Annotations(entsql.DefaultExpr("'{}'::text[]")),
		field.JSON("config_jsonb", json.RawMessage{}).SchemaType(map[string]string{dialect.Postgres: "jsonb"}).Optional(),
		field.Time("created_at").SchemaType(map[string]string{dialect.Postgres: "timestamptz"}).Annotations(entsql.DefaultExpr("now()")),
		field.Time("updated_at").SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (CompanyUseCase) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("company_id", "id").Unique().StorageKey("company_use_cases_uq1"),
		index.Fields("company_id", "use_case_id").Unique().StorageKey("company_use_cases_uq2"),
	}
}

func (CompanyUseCase) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "company_use_cases"},
		entsql.Checks(map[string]string{
			"company_use_cases_company_id_check": "company_id IS NULL OR company_id ~ '[^[:space:]]'",
		}),
		annotation.Constraints{
			ForeignKeys: []annotation.ForeignKey{
				{Symbol: "company_use_cases_fk1", Columns: []string{"use_case_id"}, Table: "use_cases", Target: []string{"id"}},
			},
		},
	}
}
