# Workflow — Minhas Encomendas

Mostra as encomendas da unidade que chegaram na portaria e aguardam retirada.

A ordem do acontecimento (os passos de `caso.yaml`, que o harness executa de forma determinística):

1. Chame `unidades.listar` (chamar_recurso) e guarde em `unidades`.
2. Escolha `unidade` entre as opções de `unidades` (mostre `label`): "De qual unidade?". Reaproveite se já souber (reuse: always). Se vazio: "Sua unidade ainda não tem encomendas disponíveis por aqui."
3. Chame `encomendas.listar` (chamar_recurso) com `unidade_id` = {{.unidade}} e guarde em `encomendas`.
4. Responda no formato: {{if .encomendas}}Encomendas aguardando retirada ({{.unidade_label}}): {{range .encomendas}}• {{.label}} {{end}} Se quiser o código de retirada, é só pedir.{{else}}Não há encomendas aguardando retirada para {{.unidade_label}}.{{end}}
