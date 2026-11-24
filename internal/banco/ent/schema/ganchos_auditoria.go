package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// GanchosAuditoria é uma decisão de bloquear, ajustar ou confirmar tomada por um gancho ou
// regra. Deixar passar não gera linha (vira só métrica). regra_id e regra_versao ficam
// vazios/zero quando quem decidiu foi um gancho fixo. Nunca guarda texto da mensagem,
// params, resultado, CPF ou telefone.
type GanchosAuditoria struct {
	ent.Schema
}

func (GanchosAuditoria) Fields() []ent.Field {
	return []ent.Field{
		field.String("scope_type"),
		field.String("channel_id").Optional().Immutable(),
		field.UUID("id", uuid.New()).Default(uuid.New).Immutable(),
		field.Time("criado_em").StorageKey("created_at").SchemaType(map[string]string{dialect.Postgres: "timestamptz"}).Immutable(),
		field.String("origem").StorageKey("origin").NotEmpty().Immutable(),
		field.String("evento").StorageKey("event").NotEmpty().Immutable(),
		field.String("gancho").StorageKey("hook").NotEmpty().Immutable(),
		field.String("regra_id").StorageKey("rule_id").Optional().Immutable(),
		field.Int("regra_versao").StorageKey("rule_version").Default(0).Immutable(),
		field.String("decisao").StorageKey("decision").NotEmpty().Immutable(),
		field.Text("motivo").StorageKey("reason").Immutable(),
		field.String("administradora_id").StorageKey("company_id").Optional().Immutable(),
		field.String("condominio_id").StorageKey("condo_id").Optional().Immutable(),
		field.String("sessao_id").StorageKey("session_id").Optional().Immutable(),
		field.String("protocolo").StorageKey("protocol").Default("").Immutable(),
		field.String("usuario_id").StorageKey("user_id").Default("").Immutable(),
		field.String("tela").StorageKey("screen").Default("").Immutable(),
		field.String("ferramenta").StorageKey("tool").Default("").Immutable(),
		field.Int("quantidade").StorageKey("count").Default(0).Immutable(),
	}
}

// Indexes: as telas do histórico filtram por administradora e por condomínio, mais recentes
// primeiro; o job de retenção (12 meses) varre por criado_em.
func (GanchosAuditoria) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("administradora_id", "criado_em").Annotations(entsql.DescColumns("criado_em")),
		index.Fields("condominio_id", "criado_em").Annotations(entsql.DescColumns("criado_em")),
		index.Fields("criado_em"),
	}
}

func (GanchosAuditoria) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "hook_audit_logs"}}
}
