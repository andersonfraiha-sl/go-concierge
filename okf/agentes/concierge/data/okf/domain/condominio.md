---
id: okf-condominio-dominio
type: Domain
title: Domínio — atendimento do condômino pelo concierge
version: 1.0.0
status: ativo
domain: condominio/concierge
description: Quem fala (condômino identificado pelo telefone), o que ele pode pedir (casos de uso por papel e por condomínio), o que é escrita e o que é dúvida, e quando a conversa vai para o humano.
tags: [okf, dominio, condominio, concierge]
---

# Domínio — atendimento do condômino

## Identidade

O telefone identifica o condômino (AUTH → pessoa, condomínio, papel, unidades). Sem
cadastro, a conversa é **pública**: só casos `public` (FAQ público). O condomínio é a
`account` (no ATENDE, o `tenantId`).

## Escrita × dúvida

| pedido | classe | quem resolve |
|---|---|---|
| liberar, reservar, agendar, atualizar, comunicar | **escrita** | caso de uso do catálogo, com confirmação e protocolo |
| "pode?", "até que horas?", "qual o telefone?" | **dúvida** | conhecimento consultado (FAQ, regimento) |
| "quero falar com alguém", sem rota, erro | **transferência** | atendente humano |

Escrita nunca acontece fora de um caso catalogado (POLICIES 1).

## Sessão

Cada conversa é uma sessão. O observador abre uma **pendência por assunto** no início e
as encerra no fim: a sessão só termina sem pendência aberta, quando o morador obteve o
que buscava e não sinalizou continuidade.
