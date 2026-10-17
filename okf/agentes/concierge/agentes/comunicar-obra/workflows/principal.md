# Workflow — Comunicação de Obra

Usado quando o morador vai fazer obra, reforma ou reparo na unidade (pintura, elétrica, hidráulica, marcenaria...) e precisa avisar a administração e liberar os prestadores na portaria.

A ordem do acontecimento (os passos de `caso.yaml`, que o harness executa de forma determinística):

1. Chame `unidades.listar` (chamar_recurso) e guarde em `unidades`.
2. Defina `unidade` a partir de `unidades` (mostre `label`): com UMA só opção, use-a sem perguntar; com várias, pergunte "Em qual unidade será a obra?" oferecendo as opções. Se o morador já disse ou a memória/identidade já responde, use sem perguntar. Se vazio: "Não encontrei unidades vinculadas ao seu cadastro."
3. Colete, um a um e só o que ainda falta: `tipo`, `descricao`, `inicio`, `dias`, `comodos`, `ruidosa`, `prestadores`, `empresa`, `responsavel_telefone`.
4. Confirme com o morador (sim/não): "Confirma a comunicação de obra ({{.tipo_label}}) em {{.unidade_label}}, a partir de {{.inicio_label}} por {{.dias}} dia(s), com {{.prestadores_label}}? (sim/não)". Não vale sem o sim.
5. ESCRITA: chame `obras.comunicar` com `unidade_id` = {{.unidade}}, `tipo` = {{.tipo}}, `descricao` = {{.descricao}}, `inicio` = {{.inicio}}, `dias` = {{.dias}}, `comodos` = $comodos, `ruidosa` = {{.ruidosa}}, `prestadores` = $prestadores, `empresa` = {{.empresa}}, `responsavel_telefone` = {{.responsavel_telefone}}; guarde o retorno em `obra`. Só depois do confirm; o protocolo devolvido vai na resposta.
6. Responda no formato: Obra comunicada! Protocolo: {{.obra.protocolo}}. Portaria: {{.obra.liberacao_portaria}}. Horário permitido: {{.obra.horario_permitido}}.
