package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
	"go-concierge/internal/core/banco/v2/ent/annotation"
)

type HookAuditLog struct{ ent.Schema }

func (HookAuditLog) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).SchemaType(map[string]string{dialect.Postgres: "uuid"}),
		field.Text("scope_type").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Text("company_id").SchemaType(map[string]string{dialect.Postgres: "text"}).Optional(),
		field.Int32("condo_id").SchemaType(map[string]string{dialect.Postgres: "integer"}).Optional(),
		field.UUID("channel_id", uuid.UUID{}).SchemaType(map[string]string{dialect.Postgres: "uuid"}).Optional(),
		field.Text("session_id").SchemaType(map[string]string{dialect.Postgres: "text"}).Optional(),
		field.Text("protocol").SchemaType(map[string]string{dialect.Postgres: "text"}).Annotations(entsql.DefaultExpr("''")),
		field.UUID("rule_id", uuid.UUID{}).SchemaType(map[string]string{dialect.Postgres: "uuid"}).Optional(),
		field.Int32("rule_version").SchemaType(map[string]string{dialect.Postgres: "integer"}).Annotations(entsql.DefaultExpr("0")),
		field.Text("origin").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Text("event").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Text("hook").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Text("decision").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Text("reason").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Text("user_id").SchemaType(map[string]string{dialect.Postgres: "text"}).Annotations(entsql.DefaultExpr("''")),
		field.Text("screen").SchemaType(map[string]string{dialect.Postgres: "text"}).Annotations(entsql.DefaultExpr("''")),
		field.Text("tool").SchemaType(map[string]string{dialect.Postgres: "text"}).Annotations(entsql.DefaultExpr("''")),
		field.Int32("count").SchemaType(map[string]string{dialect.Postgres: "integer"}).Annotations(entsql.DefaultExpr("0")),
		field.Time("created_at").SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (HookAuditLog) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("company_id", "created_at").StorageKey("hook_audit_logs_ix1"),
		index.Fields("company_id", "condo_id", "created_at").StorageKey("hook_audit_logs_ix2"),
		index.Fields("created_at").StorageKey("hook_audit_logs_ix3"),
	}
}

func (HookAuditLog) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "hook_audit_logs"},
		entsql.Checks(map[string]string{
			"hook_audit_logs_company_id_check": "company_id IS NULL OR company_id ~ '[^[:space:]]'",
			"hook_audit_logs_scope_type_check": "scope_type IN ('GLOBAL', 'COMPANY')",
			"hook_audit_logs_scope_check":      "(scope_type = 'GLOBAL' AND company_id IS NULL AND condo_id IS NULL AND channel_id IS NULL) OR (scope_type = 'COMPANY' AND company_id IS NOT NULL)",
		}),
		annotation.Constraints{
			ForeignKeys: []annotation.ForeignKey{
				{Symbol: "hook_audit_logs_fk1", Columns: []string{"company_id", "channel_id"}, Table: "channels", Target: []string{"company_id", "id"}},
			},
		},
	}
}
