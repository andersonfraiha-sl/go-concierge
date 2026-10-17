# Workflow — Código de Retirada

Informa o código para retirar uma encomenda que já está na portaria.

A ordem do acontecimento (os passos de `caso.yaml`, que o harness executa de forma determinística):

1. Chame `unidades.listar` (chamar_recurso) e guarde em `unidades`.
2. Defina `unidade` a partir de `unidades` (mostre `label`): com UMA só opção, use-a sem perguntar; com várias, pergunte "De qual unidade?" oferecendo as opções. Se o morador já disse ou a memória/identidade já responde, use sem perguntar. Se vazio: "Sua unidade ainda não tem encomendas disponíveis por aqui."
3. Chame `encomendas.listar` (chamar_recurso) com `unidade_id` = {{.unidade}} e guarde em `encomendas`.
4. Defina `encomenda` a partir de `encomendas` (mostre `label`): com UMA só opção, use-a sem perguntar; com várias, pergunte "De qual encomenda?" oferecendo as opções. Se vazio: "Não há encomendas aguardando retirada no momento."
5. Chame `encomendas.codigo` (chamar_recurso) com `unidade_id` = {{.unidade}}, `encomenda_id` = {{.encomenda}} e guarde em `retirada`.
6. Responda no formato: O código de retirada de {{.encomenda_label}} é *{{.retirada.codigo}}*. Apresente na portaria.
