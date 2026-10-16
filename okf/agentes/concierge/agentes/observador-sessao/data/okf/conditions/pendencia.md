---
id: okf-pendencia
type: Condition
title: Estados de uma pendência da sessão
version: 1.0.0
status: ativo
description: Quando uma pendência abre, muda de estado e fecha; e a condição para a sessão encerrar.
tags: [okf, sessao, pendencia]
---

## Aberta

Há uma demanda ou pergunta explícita do morador ainda sem vínculo (caso de uso, agente
ou conhecimento). Saudação, agradecimento e "ok" nunca abrem pendência.

## Em atendimento

A pendência tem vínculo e o fluxo está em andamento (caso de uso perguntando dados,
agente consultando). Uma resposta do morador a uma pergunta do fluxo não abre pendência
nova: pertence à que está em atendimento.

## Resolvida

O caso de uso terminou com protocolo, ou a dúvida foi respondida com fonte. Nunca marque
resolvida por suposição: precisa do protocolo ou da resposta no histórico.

## Transferida

Foi ao atendente humano (sem rota, confiança baixa repetida, orçamento esgotado, pedido
explícito). Fecha a pendência para o robô; o humano a leva.

## Abandonada

O morador mudou de assunto e não voltou, cancelou, ou ficou em silêncio por 30 minutos.

## Sessão encerrada

Só com zero pendências abertas ou em atendimento, depois da última fala do robô, quando
a fala do morador sinaliza o fim (agradeceu, despediu-se, "era só isso") ou o silêncio
passou do limite. Uma pergunta nova reabre tudo: a próxima mensagem abre outra sessão.
