# Workflow — Listar Unidades

Mostra as unidades (apartamentos/casas) vinculadas ao cadastro de quem pergunta.

A ordem do acontecimento (os passos de `caso.yaml`, que o harness executa de forma determinística):

1. Chame `unidades.listar` (chamar_recurso) e guarde em `unidades`.
2. Responda no formato: Unidades vinculadas ao seu cadastro: {{range .unidades}}• {{.label}} {{end}}
