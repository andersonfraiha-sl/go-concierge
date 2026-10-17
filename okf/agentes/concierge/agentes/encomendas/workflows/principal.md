# Workflow — Minhas Encomendas

Mostra as encomendas da unidade que chegaram na portaria e aguardam retirada.

A ordem do acontecimento (os passos de `caso.yaml`, que o harness executa de forma determinística):

1. Chame `unidades.listar` (chamar_recurso) e guarde em `unidades`.
2. Defina `unidade` a partir de `unidades` (mostre `label`): com UMA só opção, use-a sem perguntar; com várias, pergunte "De qual unidade?" oferecendo as opções. Se o morador já disse ou a memória/identidade já responde, use sem perguntar. Se vazio: "Sua unidade ainda não tem encomendas disponíveis por aqui."
3. Chame `encomendas.listar` (chamar_recurso) com `unidade_id` = {{.unidade}} e guarde em `encomendas`.
4. Responda no formato: {{if .encomendas}}Encomendas aguardando retirada ({{.unidade_label}}): {{range .encomendas}}• {{.label}} {{end}} Se quiser o código de retirada, é só pedir.{{else}}Não há encomendas aguardando retirada para {{.unidade_label}}.{{end}}
