---
id: okf-fim-da-sessao
type: Question
title: Pergunta — o morador já obteve o que buscava?
question: fim_da_sessao
answers: [ENCERRAR, CONTINUAR, TRANSFERIR]
description: Como decidir, a cada mensagem, se a sessão acabou, continua ou vai ao humano.
tags: [okf, sessao, encerramento]
---

## Answers (o espaço de leituras)

`ENCERRAR` (nenhuma pendência aberta ou em atendimento; a última fala do morador é
despedida, agradecimento ou "era só isso"; confiança ≥ 0,8), `CONTINUAR` (há pendência
aberta ou em atendimento, ou a fala traz pergunta/pedido novo), `TRANSFERIR` (a pendência
não tem rota nem caso de uso, ou o morador pediu uma pessoa).

## Method

1. Conte as pendências por estado. 2. Classifique a última fala do morador: despedida,
resposta a pergunta do fluxo, pedido novo, pedido de humano. 3. Se há pendência viva →
CONTINUAR. 4. Sem pendência viva e despedida → ENCERRAR com a confiança. 5. Pedido sem
rota → TRANSFERIR.

## Tie-breaking

- Uma saudação, uma pergunta ou uma resposta do robô **nunca** encerram sozinhas.
- "Obrigado" logo após uma pergunta do fluxo é resposta educada, não despedida: CONTINUAR.
- Na dúvida entre ENCERRAR e CONTINUAR, CONTINUAR: encerrar cedo custa uma sessão nova;
  continuar custa só o silêncio.
