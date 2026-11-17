# Workflow — Código de Retirada

Informa o código para retirar uma encomenda que já está na portaria.

A ordem do acontecimento (os passos de `caso.yaml`, que o harness executa de forma determinística):

1. Chame `gruvi.bff.building-operation.get-company-by-document-controller-handle` (chamar_recurso) e guarde em `condominio`.
2. Se a conversa já tem `unidade`, pergunte "Deseja reutilizar a unidade {{.reuso_label}}?" (sim/não, guardado em `reutilizar_unidade`); com o sim, reaproveite sem perguntar de novo.
3. Defina `unidade` a partir de `condominio` (mostre `unit`): com UMA só opção, use-a sem perguntar; com várias, pergunte "De qual unidade?" oferecendo as opções. Se vazio: "A consulta de encomendas não está disponível para as unidades retornadas."
4. Chame `gruvi.bff.building-operation.fetch-unit-packages-controller-handle` (chamar_recurso) com `unitId` = $unidade, `filter_status` = waiting e guarde em `encomendas`.
5. Defina `encomenda` a partir de `encomendas` (mostre `description`): com UMA só opção, use-a sem perguntar; com várias, pergunte "De qual encomenda?" oferecendo as opções. Se vazio: "Não há encomendas aguardando retirada no momento."
6. Chame `gruvi.bff.building-operation.get-package-delivery-code-controller-handle` (chamar_recurso) com `unitId` = $unidade, `packageId` = $encomenda e guarde em `retirada`.
7. Responda no formato: O código de retirada de {{.encomenda_label}} é *{{.retirada.delivery_code}}*. Apresente na portaria.
