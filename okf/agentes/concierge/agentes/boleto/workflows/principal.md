# Workflow — Segunda Via de Boleto

Lista as cobranças em aberto do morador e passa valor, vencimento, linha digitável, Pix e link do boleto escolhido.

A ordem do acontecimento (os passos de `caso.yaml`, que o harness executa de forma determinística):

1. Chame `gruvi.bff.mobile.fetch-fees-controller-handle-1` (chamar_recurso) com `filter[only-open]` = true, `filter[with-due-status]` = true e guarde em `cobrancas`.
2. Defina `cobranca` a partir de `cobrancas` (mostre `due_date`): com UMA só opção, use-a sem perguntar; com várias, pergunte "Qual vencimento?" oferecendo as opções. Se vazio: "Não achei boleto em aberto disponível para emissão automática. Se a cobrança está vencida há mais tempo ou em negociação, a segunda via sai com a equipe da administradora; posso te passar para um atendente."
3. Chame `gruvi.bff.mobile.get-fee-by-id-controller-handle-2` (chamar_recurso) com `feeId` = $cobranca e guarde em `boleto`.
4. Responda no formato: Boleto de {{.boleto.unit_description}}, vencimento {{.boleto.due_date}}, valor R$ {{.boleto.value}}{{if .boleto.due_status_description}} ({{.boleto.due_status_description}}){{end}}. {{if .boleto.billet_code}}Linha digitável: {{.boleto.billet_code}} {{end}}{{if .boleto.pix_code}}Pix copia e cola: {{.boleto.pix_code}} {{end}}{{if .boleto.billet_view_url}}PDF: {{.boleto.billet_view_url}}{{else if .boleto.billet_url}}PDF: {{.boleto.billet_url}}{{end}}
