# Workflow — Débito e Acordo

Lista as cobranças em aberto do morador com vencimento, valor e situação, inclusive as que não têm boleto disponível. Acordo e parcelamento seguem com o atendente, com esse resumo.

A ordem do acontecimento (os passos de `caso.yaml`, que o harness executa de forma determinística):

1. Chame `gruvi.bff.mobile.fetch-fees-controller-handle-1` (chamar_recurso) com `filter[only-open]` = true, `filter[with-due-status]` = true e guarde em `abertas`.
2. Responda no formato: {{if .abertas}}Cobranças em aberto: {{range .abertas}}- {{.due_date}}: R$ {{.value}}{{if .due_status_description}} ({{.due_status_description}}){{end}}{{if .billet_unavailable}}, sem boleto disponível{{end}} {{end}}Para negociar ou parcelar, te passo para um atendente com esse resumo.{{else}}Não há cobrança em aberto na sua unidade.{{end}}
