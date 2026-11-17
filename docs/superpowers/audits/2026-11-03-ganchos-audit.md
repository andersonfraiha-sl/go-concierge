# Pre-Implementation Audit Report

**Spec**: 2026-11-03-ganchos-design.md | **Plan**: 2026-11-03-ganchos.md | **Date**: 2026-11-03
**Constitution**: constitution.md (v1.0 + 2 emendas)

## Summary

| Metric | Count |
|--------|-------|
| Objetivos (§3) + critérios de pronto (§13) | 7 + 11 |
| Tarefas do plano | 26, em 11 ondas |
| Cobertura spec → plano | 18/18, 3 parciais |
| Rastreio plano → spec | 26/26 |
| Violações da constituição | 0 |
| CRITICAL | 1, corrigido no plano |

## Forward Traceability

| Item da spec | Tarefas | Status |
|---|---|---|
| Obj 1: seis eventos, ponto único | 2, 11, 14, 24 | ✅ |
| Obj 2: um desfecho por gancho, composição | 2 | ✅ |
| Obj 3: fixos com o mesmo comportamento | 6, 7, 11, 14, 24, 25 | ✅ |
| Obj 4: PII fora de modelo, log, trace, auditoria | 3, 7, 11, 14, 15, 25 | ⚠️ Parcial: atributo de span |
| Obj 5: regras declarativas só pela Superlógica | 4, 10, 12, 13, 21 | ✅ |
| Obj 6: auditoria e Histórico | 4, 9, 15, 17, 22 | ✅ |
| Obj 7: zero ida a mais no turno quente | 1, 15, 18 | ✅ |
| Implantação dedicada pelo Importar e exportar (§7) | 13, 23 | ✅ |
| Retenção de 12 meses (§8) | 9, 15 | ✅ |
| Avaliação antecipada (§4) | 16 | ✅ |
| Contrato Go e Zod (§7) | 10, 20 | ✅ |
| Observabilidade: logs, métricas, trace (§12) | 5, 15 | ⚠️ Parcial: sem exportador, sem alertas |
| §13: 0 idas com 20 regras | 18 | ✅ |
| §13: `BenchmarkGanchos` p99 ≤ 50 µs | 12 | ✅ |
| §13: testes atuais sem alteração | 11, 14, 25 | ✅ |
| §13: `grep` sem `Confirmado(` e `fixarContextoB2C(` | 11, 19, 25 | ⚠️ Parcial: padrão estreitado |
| §13: quatro regras ponta a ponta, auditoria visível | 19, 22 | ✅ |
| §13: regra salva muda o turno sem restart | 19 | ✅ |
| §13: `403` para administradora, histórico por escopo | 13, 17, 19 | ✅ |
| §13: fixo não desliga | 13, 15 | ✅ |
| §13: CPF fora do modelo, histórico, memória, logs | 14, 15 | ✅ |
| §13: tabela da política de falha verde | 2, 11, 12, 14, 24 | ✅ depois da correção |
| §13: `go vet`, `go test -race`, testes do Studio | 15, 19, 20 a 26 | ✅ |

## Reverse Traceability

Todas as tarefas apontam para a spec. Task 1 e Task 18: §10. Task 13 (exportar e importar): decisão da §7 e §15. Nenhum fantasma.

## Clarification Coverage

| Categoria | Status | Lacuna |
|---|---|---|
| Escopo funcional | ✅ | nenhuma |
| Domínio e dados | ✅ | nenhuma |
| UX | ✅ | lista, formulário, histórico, `422`, `409`, sem permissão |
| Não funcional | ⚠️ | métricas sem exportador; alertas da §12 sem tarefa |

## Constitution Compliance

| Princípio | Status |
|---|---|
| I. Núcleo funcional | ✅ ganchos puros, I/O só em store, cache, auditoria e bordas |
| II. Postgres, ent + sqlc | ✅ sqlc só no `InserirAuditoria`, com o porquê; histórico, contagem e retenção pelo ent |
| III. Escrita só por fluxo confirmado | ✅ `escrita_confirmada` e `fluxo_valido` |
| IV. RBAC com escopo | ✅ `ganchos.editar` contra `superlogica`, concessão ignorada |
| V. IA não decide | ✅ |
| VI. Dado pessoal na origem | ✅ cofre só em memória, fato com marcador descartado |
| VII. Erros e prazos | ✅ prazo por gancho, 2 s no Copiloto, 30 s na retenção, 3 s nas rotas |

Padrões proibidos: nenhum. `any` só no payload do Nexus (params e resultado), que é a exceção da constituição.

## Verdict: WARN

### Critical Issues
1. **Tabela da política de falha sem teste por evento** (§11 e critério "Tabela da política de falha verde"). O plano testava a política no `Avaliar` genérico (Task 2) e na regra de escrita (Task 12), mas não cada célula: fixo de `mensagem_recebida` transbordando com "não consegui atender agora", fixo de `antes_de_responder` caindo na frase de `responderFalha`, fixo de `depois_da_ferramenta` segurando o resultado. **Corrigido no plano**: `TestPoliticaDeFalhaNaFerramenta` (Task 11), `TestPoliticaDeFalhaNoTurno` (Task 14) e `TestPoliticaDeFalhaCopiloto` (Task 24).

### Warnings
1. **Sem exportador de métricas.** O projeto não tem OTel ligado; o plano usa `noop.NewMeterProvider()` no `main` (Task 15). As métricas existem e são testadas, mas não saem do processo. Os alertas da §12 dependem do exportador e de configuração fora do repositório. Precisa de decisão do Felipe (OTLP para onde).
2. **Atributo de span** (§5): o projeto ainda não cria spans; a Task 5 só põe evento de span sem texto. Quando houver trace, a máscara de atributo entra junto.
3. **`grep` do critério estreitado**: `pedidos.Confirmado` existe e é outra função (extrator de pedidos). O plano procura `harness\.Confirmado(` e `func Confirmado` dentro de `internal/harness`.
4. **Importação na implantação dedicada** (Task 13): quem importa lá é quem tem `studio.avancado` em `administradora:<fixa>`. O arquivo não é assinado, então dá para editar o JSON antes de importar. Se isso contar como "administradora editando regra", a saída é assinar a exportação (HMAC com segredo por implantação). Decisão do Felipe.
5. **Camadas da cascata que ainda não existem** (embedding, Jev): os testes provam os pontos no motor do caso e no LLM. Camada nova herda os ganchos por passar pelo `invocar` e pelo `Sender.Enviar`; o teste dela entra com ela.
6. Tasks 1, 18 e 19 não têm o passo de FAIL: medição e teste de aceitação, por natureza.
7. A leitura das regras no boot (`Cache.Aquecer`) não tinha prazo. Ajustado no plano: 5 s, o mesmo `ConnectTimeout` do pool.
