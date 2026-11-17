# Workflow — Liberação de Visitante

Usado para o dono ou morador da unidade autorizar a entrada de um visitante na portaria (amigo, parente, prestador de serviço, entregador). Liberações para depois de amanhã ou recorrentes devem ser feitas no app Gruvi: https://gruvi.link/guest.

A ordem do acontecimento (os passos de `caso.yaml`, que o harness executa de forma determinística):

1. Chame `gruvi.bff.building-operation.get-company-by-document-controller-handle` (chamar_recurso) e guarde em `condominio`.
2. Se a conversa já tem `unidade`, pergunte "Deseja reutilizar a unidade {{.reuso_label}}?" (sim/não, guardado em `reutilizar_unidade`); com o sim, reaproveite sem perguntar de novo.
3. Defina `unidade` a partir de `condominio` (mostre `unit`): com UMA só opção, use-a sem perguntar; com várias, pergunte "Para qual unidade é a visita?" oferecendo as opções. Se vazio: "A liberação não está disponível para as unidades retornadas."
4. Se a conversa já tem `motivo`, `categoria_motivo`, pergunte "Deseja reutilizar o motivo de visita {{.reuso_label}}?" (sim/não, guardado em `reutilizar_motivo`); com o sim, reaproveite sem perguntar de novo.
5. Colete, um a um e só o que ainda falta: `visitante_nome`, `visitante_telefone`, `data_visita`, `categoria_motivo`.
6. Chame `gruvi.bff.building-operation.fetch-visit-reasons-controller-handle` (chamar_recurso) com `filter_visitor_type` = {{.categoria_motivo}} e guarde em `motivos`.
7. Defina `motivo` a partir de `motivos` (mostre `reason`): com UMA só opção, use-a sem perguntar; com várias, pergunte "Qual motivo de prestador?" oferecendo as opções. Se vazio: "Não há motivos disponíveis para essa categoria de visita."
8. Confirme com o morador (sim/não): "Confirma a liberação de {{.visitante_nome}} ({{.motivo_label}}) para {{.unidade_label}} {{dateLabel .data_visita}}? (sim/não)". Não vale sem o sim.
9. ESCRITA: chame `gruvi.bff.building-operation.create-visit-controller-handle` com `unitId` = $unidade, `start_datetime` = {{dateTime .data_visita "inicio"}}, `end_datetime` = {{dateTime .data_visita "fim"}}, `reason` = $motivo, `visitors` = [{"mobile_number":"$visitante_telefone","name":"{{.visitante_nome}}"}]; guarde o retorno em `liberacao`. Só depois do confirm; o protocolo devolvido vai na resposta.
10. Responda no formato: Prontinho! O {{.visitante_nome}} já está liberado para {{dateLabel .data_visita}} na unidade {{.unidade_label}}. {{if .liberacao.visit.shareable_link}}Ele pode usar este link para fazer o cadastro e acessar o condomínio: {{.liberacao.visit.shareable_link}} Compartilhe esta mensagem com sua visita para facilitar o acesso.{{end}}
