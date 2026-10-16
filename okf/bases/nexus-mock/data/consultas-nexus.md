---
title: Consultas prontas ao Nexus
description: As chamadas típicas, já no formato do contrato — use antes de montar uma chamada própria.
---

# Consultas prontas

- Unidades do morador: `POST /v1/resources/unidades.listar/invoke` `{"account_id": "<account>", "params": {}}`
- Áreas reserváveis: `POST /v1/resources/areas.listar/invoke`
- FAQ: `POST /v1/resources/faq.buscar/invoke` `{"params": {"query": "<pergunta>", "escopo": "publico"}}`

Toda chamada de escrita leva `idempotency_key` = `sessão:pendência:passo`, estável entre retentativas.
