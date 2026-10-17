# Workflow — Reserva de Área Comum

Reserva uma área comum do condomínio (churrasqueira, salão de festas, quadra) para uma data.

A ordem do acontecimento (os passos de `caso.yaml`, que o harness executa de forma determinística):

1. Chame `unidades.listar` (chamar_recurso) e guarde em `unidades`.
2. Defina `unidade` a partir de `unidades` (mostre `label`): com UMA só opção, use-a sem perguntar; com várias, pergunte "A reserva é para qual unidade?" oferecendo as opções. Se o morador já disse ou a memória/identidade já responde, use sem perguntar. Se vazio: "Não encontrei unidades vinculadas ao seu cadastro."
3. Chame `areas.listar` (chamar_recurso) e guarde em `areas`.
4. Defina `area` a partir de `areas` (mostre `nome`): com UMA só opção, use-a sem perguntar; com várias, pergunte "Qual área você quer reservar?" oferecendo as opções.
5. Colete, um a um e só o que ainda falta: `data`.
6. Confirme com o morador (sim/não): "Confirma a reserva de {{.area_label}} em {{.data_label}} para {{.unidade_label}}? (sim/não)". Não vale sem o sim.
7. ESCRITA: chame `areas.reservar` com `unidade_id` = {{.unidade}}, `area_id` = {{.area}}, `data` = {{.data}}; guarde o retorno em `reserva`. Só depois do confirm; o protocolo devolvido vai na resposta.
8. Responda no formato: Reserva confirmada: {{.area_label}} em {{.data_label}} ({{.unidade_label}}). Protocolo: {{.reserva.protocolo}}.
