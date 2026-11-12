# Workflow — Minhas Encomendas

Mostra as encomendas da unidade. Por padrão consulta as que aguardam retirada; também pode consultar entregues ou canceladas quando o morador pedir esse status.

A ordem do acontecimento (os passos de `caso.yaml`, que o harness executa de forma determinística):

1. Chame `agent_utcp.building-operation.GetCompanyByDocumentController_handle` (chamar_recurso) e guarde em `condominio`.
2. Se a conversa já tem `unidade`, pergunte "Deseja reutilizar a unidade {{.reuso_label}}?" (sim/não, guardado em `reutilizar_unidade`); com o sim, reaproveite sem perguntar de novo.
3. Defina `unidade` a partir de `condominio` (mostre `unit`): com UMA só opção, use-a sem perguntar; com várias, pergunte "De qual unidade?" oferecendo as opções. Se vazio: "A consulta de encomendas não está disponível para as unidades retornadas."
4. Colete, um a um e só o que ainda falta: `status`.
5. Chame `agent_utcp.building-operation.FetchUnitPackagesController_handle` (chamar_recurso) com `unitId` = $unidade, `filter[status]` = {{if .status}}{{.status}}{{else}}waiting{{end}} e guarde em `encomendas`.
6. Responda no formato: {{if .encomendas}}Encomendas {{if eq .status "delivered"}}entregues{{else if eq .status "canceled"}}canceladas{{else}}aguardando retirada{{end}} ({{.unidade_label}}): {{range .encomendas}}• {{if .description}}{{.description}}{{else if .package_size}}{{.package_size}}{{else}}Encomenda{{end}}{{if .tracking_code}} ({{.tracking_code}}){{end}} {{end}}{{else}}Não há encomendas {{if eq .status "delivered"}}entregues{{else if eq .status "canceled"}}canceladas{{else}}aguardando retirada{{end}} para {{.unidade_label}}.{{end}}
