# Workflow — Confirmação de Pagamento

Lista as cobranças pagas do morador e passa data, forma e valor do pagamento escolhido. Sem registro, explica o prazo de compensação e oferece o atendente.

A ordem do acontecimento (os passos de `caso.yaml`, que o harness executa de forma determinística):

1. Chame `gruvi.bff.mobile.fetch-fees-controller-handle-1` (chamar_recurso) com `filter[paid]` = true e guarde em `pagas`.
2. Defina `cobranca` a partir de `pagas` (mostre `due_date`): com UMA só opção, use-a sem perguntar; com várias, pergunte "Qual vencimento você pagou?" oferecendo as opções. Se vazio: "Ainda não aparece pagamento registrado na sua unidade. Boleto leva até 3 dias úteis para compensar; se já passou disso, me manda o comprovante que te passo para a equipe da administradora dar a baixa."
3. Chame `gruvi.bff.mobile.get-fee-by-id-controller-handle-2` (chamar_recurso) com `feeId` = $cobranca e guarde em `pagamento`.
4. Responda no formato: Pagamento registrado: {{.pagamento.unit_description}}, vencimento {{.pagamento.due_date}}, valor R$ {{.pagamento.value}}{{if .pagamento.paid_date}}, pago em {{.pagamento.paid_date}}{{end}}{{if .pagamento.payment_method}} via {{.pagamento.payment_method}}{{end}}.
