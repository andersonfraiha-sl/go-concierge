# Workflow — Agendamento de Mudança

Usado para o morador agendar uma data e período para sua mudança, garantindo a reserva do elevador de serviço.

A ordem do acontecimento (os passos de `caso.yaml`, que o harness executa de forma determinística):

1. Chame `unidades.listar` (chamar_recurso) e guarde em `unidades`.
2. Defina `unidade` a partir de `unidades` (mostre `label`): com UMA só opção, use-a sem perguntar; com várias, pergunte "Para qual unidade é a mudança?" oferecendo as opções. Se o morador já disse ou a memória/identidade já responde, use sem perguntar. Se vazio: "Não encontrei unidades vinculadas ao seu cadastro. Por favor, entre em contato com a administração."
3. Colete, um a um e só o que ainda falta: `data_mudanca`, `periodo_mudanca`.
4. Confirme com o morador (sim/não): "Confirma o agendamento da mudança para a unidade {{.unidade_label}} em {{.data_mudanca_label}}, {{.periodo_mudanca_label}}? (sim/não)". Não vale sem o sim.
5. ESCRITA: chame `mudancas.agendar` com `unidade_id` = {{.unidade}}, `data` = {{.data_mudanca}}, `periodo` = {{.periodo_mudanca}}; guarde o retorno em `agendamento`. Só depois do confirm; o protocolo devolvido vai na resposta.
6. Responda no formato: Sua mudança para {{.unidade_label}} em {{.data_mudanca_label}}, {{.periodo_mudanca_label}}, foi agendada com sucesso! Protocolo: {{.agendamento.protocolo}}. O elevador de serviço estará reservado para você.
