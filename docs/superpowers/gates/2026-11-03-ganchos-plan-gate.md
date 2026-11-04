# Post-Plan Gate: Ganchos

Data: 2026-11-03 · Plano: `docs/superpowers/plans/2026-11-03-ganchos.md` · Auditoria: `docs/superpowers/audits/2026-11-03-ganchos-audit.md` · Constituição: v1.0

## Plan Completeness
- ✅ 26/26 tarefas com `Files:` e caminhos exatos (migrations geradas com `<timestamp>`, que é o nome que o `gerar-migration` dá).
- ✅ Interfaces com assinaturas em bloco de código nas tarefas que produzem tipo ou função; passos de verificação com comando e saída esperada.
- ✅ Nenhum placeholder (scan de TBD, TODO, FIXME, "similar to Task", "add appropriate", "handle edge cases": zero).
- ✅ TDD (teste → FAIL → implementa → PASS) em 23/26. Sem passo de FAIL, por natureza: Task 1 (linha de base), Task 18 (medição) e Task 19 (aceitação ponta a ponta).

## Architecture Quality
- ✅ Decisões com justificativa na spec §15 e no plano: núcleo puro, pontos nas fronteiras, sqlc só no lote da auditoria, cache igual ao `admin.Cache`, OTel isolado na Task 5.
- ✅ Estrutura por responsabilidade: `ganchos` (núcleo), `pii`, `fixos`, `regras`, `auditoria`, bordas no `harness`, `atende` e `cmd`.
- ✅ Sem abstração sobrando: uma interface nova só (`pedidos.Previsor`, opcional por type assertion); o fake de teste é o próprio `Conjunto` com ganchos roteirizados.

## Constitution Compliance
- ✅ Nenhum padrão proibido: sem estado global mutável (`Conjunto`, cache e métricas injetados), SQL solto zero, `any` só no payload do Nexus.
- ✅ Stack conforme a tabela de tecnologia: Go 1.27.3, Postgres 18, ent + sqlc sobre pgx v5, migrations goose geradas pelo Atlas.
- ✅ Prioridades na ordem das ondas: confiabilidade e segurança (fixos, PII, política de falha) antes das regras, regras antes das telas.

## Cross-Artifact Consistency
- ✅ Sem requisito órfão: 18/18 na auditoria.
- ✅ Sem tarefa fantasma: 26/26 rastreadas.
- ✅ Todo critério da §13 tem teste ou passo de verificação. O único CRITICAL da auditoria (tabela da política de falha por evento) foi corrigido no plano nas Tasks 11, 14 e 24.

## Execution Readiness
- ✅ Worktree: um por tarefa, saído da `main`, merge na `main` ao fim de cada onda.
- ✅ Execução: subagent-driven com ondas paralelas, até 15 agentes. Maior onda: 6 tarefas (onda 2). Nenhum arquivo é tocado por duas tarefas da mesma onda.
- ✅ Spec aprovada pelo Felipe em 2026-11-03; as duas questões em aberto foram respondidas e registradas.
- ⚠️ Dependência externa: ondas 7 a 11 esperam o merge da PR #3.
- ⚠️ Pendências de decisão: assinatura do arquivo de regras importado na implantação dedicada (aviso 4 da auditoria), antes da onda 3 (Task 13); exportador de métricas, hoje `noop` (aviso 1), antes de ligar os alertas, sem travar onda nenhuma.

## Decisão: **PASS**
Todos os itens automáticos passam e os de julgamento estão confirmados. Os ⚠️ são dependência conhecida e decisões que entram antes das ondas que dependem delas.
