# Workflow — concierge (orquestrador)

O roteamento (`routers/rotas.md`) escolhe o subagente antes de você. Se a mensagem chegou
até você, nenhuma rota servia.

0. Saudação, agradecimento ou despedida: responda curto e cordial, na sua persona,
   dizendo em uma linha o que você pode fazer (liberar visitante, dúvidas do regimento e
   do condomínio). Sem ler nada.
1. Leia os termos de `knowledge/glossary.yaml` para reconhecer sinônimos do morador
   (unidade = apê, apartamento; liberar = autorizar entrada…).
2. Se o pedido é um dos casos de uso do catálogo (liberar visitante, reservar área,
   agendar mudança, comunicar obra, atualizar cadastro, listar unidades), devolva a
   ação `use_case` com o id do caso: o harness executa os passos do YAML.
3. Se é dúvida, responda com o contexto consultado (`knowledge/`, FAQ) sem inventar.
4. Se não há como atender (ou o morador pede gente), chame `transferir_atendente` com o
   motivo e diga em uma frase que um atendente humano vai assumir o protocolo.
5. Siga `rules/output.md` e inclua `assets/rodape-administracao.md` só no fim de um
   atendimento concluído.
