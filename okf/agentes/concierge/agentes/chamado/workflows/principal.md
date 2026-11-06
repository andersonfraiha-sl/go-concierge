# Workflow — Abertura de Chamado

Usado quando o morador relata um problema de manutenção ou uma ocorrência no condomínio (vazamento, lâmpada queimada, portão quebrado, elevador parado, barulho) e quer registrar para a administração.

A ordem do acontecimento (os passos de `caso.yaml`, que o harness executa de forma determinística):

1. Chame `gruvi.bff.building-operation.get-company-by-document-controller-handle` (chamar_recurso) e guarde em `condominio`.
2. Defina `unidade_chamado` a partir de `condominio` (mostre `unit`): com UMA só opção, use-a sem perguntar; com várias, pergunte "Para qual unidade é o chamado?" oferecendo as opções. Se vazio: "Não encontrei unidades vinculadas ao seu cadastro."
3. Chame `gruvi.bff.mobile.get-parameters-controller-handle-get-ticket-parameters-c4fee62606` (chamar_recurso) com `filter[unit.id]` = $unidade_chamado e guarde em `parametros`.
4. Defina `tipo` a partir de `parametros` (mostre `description`): com UMA só opção, use-a sem perguntar; com várias, pergunte "Qual o tipo do chamado?" oferecendo as opções. Se vazio: "A administração não liberou abertura de chamado pelo WhatsApp para a sua unidade. Posso te passar para um atendente."
5. Colete, um a um e só o que ainda falta: `descricao`.
6. Confirme com o morador (sim/não): "Confirma a abertura do chamado de {{.tipo_label}} para {{.unidade_chamado_label}}: "{{.descricao}}"? (sim/não)". Não vale sem o sim.
7. ESCRITA: chame `gruvi.bff.mobile.create-ticket-controller-handle` com `unit_id` = $unidade_chamado, `type_id` = $tipo, `description` = {{.descricao}}; guarde o retorno em `chamado`. Só depois do confirm; o protocolo devolvido vai na resposta.
8. Responda no formato: Chamado aberto{{if .chamado.id}}, protocolo {{.chamado.id}}{{end}}. A administração acompanha por lá e o andamento aparece no app.
