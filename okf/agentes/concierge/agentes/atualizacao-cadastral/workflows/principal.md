# Workflow — Atualização Cadastral

Atualiza dados cadastrais da unidade (e-mail, veículo, pets...). Os campos do formulário são dinâmicos: vêm do Nexus para a unidade escolhida.

A ordem do acontecimento (os passos de `caso.yaml`, que o harness executa de forma determinística):

1. Chame `unidades.listar` (chamar_recurso) e guarde em `unidades`.
2. Escolha `unidade` entre as opções de `unidades` (mostre `label`): "Qual unidade você quer atualizar?". Reaproveite se já souber (reuse: always). Se vazio: "Não encontrei unidades vinculadas ao seu cadastro."
3. Chame `cadastro.campos` (chamar_recurso) com `unidade_id` = {{.unidade}} e guarde em `campos`.
4. Monte o formulário dinâmico com os campos de `campos` e guarde as alterações em `dados`.
5. Confirme com o morador (sim/não): "Vou salvar para {{.unidade_label}}: {{.dados_resumo}}Confirma? (sim/não)". Não vale sem o sim.
6. ESCRITA: chame `cadastro.atualizar` com `unidade_id` = {{.unidade}}, `dados` = $dados; guarde o retorno em `atualizacao`. Só depois do confirm; o protocolo devolvido vai na resposta.
7. Responda no formato: Cadastro atualizado com sucesso. Protocolo: {{.atualizacao.protocolo}}.
