package auditoria

import dominio "go-concierge/internal/harness/ganchos/auditoria"

type Auditor = dominio.Auditor

var NovoAuditorComGravador = dominio.NovoAuditorComGravador
var ComBase = dominio.ComBase

type Coletor = dominio.Coletor

var ComColetor = dominio.ComColetor

const decisaoFalhou = dominio.DecisaoFalhou

type Linha = dominio.Linha

var LinhasDe = dominio.LinhasDe
