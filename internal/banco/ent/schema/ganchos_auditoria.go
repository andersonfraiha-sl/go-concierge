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
		field.UUID("id", uuid.New()).Default(uuid.New).Immutable(),
		field.Time("criado_em").SchemaType(map[string]string{dialect.Postgres: "timestamptz"}).Immutable(),
		field.String("origem").NotEmpty().Immutable(),
		field.String("evento").NotEmpty().Immutable(),
		field.String("gancho").NotEmpty().Immutable(),
		field.String("regra_id").Default("").Immutable(),
		field.Int("regra_versao").Default(0).Immutable(),
		field.String("decisao").NotEmpty().Immutable(),
		field.Text("motivo").Immutable(),
		field.String("administradora_id").Default("").Immutable(),
		field.String("condominio_id").Default("").Immutable(),
		field.String("sessao_id").Default("").Immutable(),
		field.String("protocolo").Default("").Immutable(),
		field.String("usuario_id").Default("").Immutable(),
		field.String("tela").Default("").Immutable(),
		field.String("ferramenta").Default("").Immutable(),
		field.Int("quantidade").Default(0).Immutable(),
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
	return []schema.Annotation{entsql.Annotation{Table: "ganchos_auditoria"}}
}
