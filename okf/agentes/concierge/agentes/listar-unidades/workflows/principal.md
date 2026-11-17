# Workflow — Listar Unidades

Mostra as unidades (apartamentos/casas) vinculadas ao cadastro de quem pergunta.

A ordem do acontecimento (os passos de `caso.yaml`, que o harness executa de forma determinística):

1. Chame `gruvi.bff.building-operation.get-company-by-document-controller-handle` (chamar_recurso) e guarde em `condominio`.
2. Responda no formato: Unidades vinculadas ao seu cadastro: {{range .condominio.units}}• {{.unit}} {{end}}
