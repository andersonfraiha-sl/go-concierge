# Pre-Implementation Audit Report

**Spec**: `docs/superpowers/specs/2026-11-03-studio-design.md` (vocabulário, fluxo, acesso, UX) + `docs/superpowers/analises/2026-11-03-studio-lacunas.md` (itens P0/P1, §7, fonte de escopo da fase 2)
**Plan**: `docs/superpowers/plans/2026-11-03-studio-fase2.md` (33 tasks, 0-32)
**Constitution**: `docs/superpowers/constitution.md`
**Date**: 2026-11-03

## Summary

| Metric | Count |
|---|---|
| Itens P0+P1 da análise (§7, 1-21) | 21 |
| Forward coverage (item → task) | 21/21 (100%) |
| Tasks do plano | 33 (0-32) |
| Reverse traceability (task → item/decisão) | 33/33 (100%, incl. Copiloto e visual por Decisões do Felipe) |
| Constitution violations (novas, introduzidas por este plano) | 0 BLOCK, 1 WARN (herdada) |

## Forward Traceability (análise §7 → plano)

| Item | Descrição | Task(s) | Status |
|---|---|---|---|
| P0-1 | Resumo separado do "Quando usar" (Go) | Task 2 | ✅ |
| P0-2 | Contratos do front (resumo/id/icon, ícones) | Task 4 | ✅ |
| P0-3 | SkillsSecao como lista densa | Task 17 | ✅ |
| P0-4 | Drawer da skill no condomínio | Task 6 | ✅ |
| P0-5 | CampoAjuste (placeholder herdado, unidade, "Usar o padrão") | Task 7 | ✅ |
| P0-6 | Orientações gerais legíveis | Task 8 | ✅ |
| P0-7 | Vazamentos de vocabulário + Concierge×Assistente | Task 5 (+ Decisão do Felipe 1, Task 3) | ✅ |
| P0-8 | Sequência da st16 (servir `/studio`, manter telas antigas) | Task 1 (serviço `/studio` já mergeado nesta worktree per linha 13 do plano) | ✅ |
| P1-9 | Matriz skills × condomínios | Task 18 | ✅ |
| P1-10 | Lista de condomínios em tabela | Task 9 | ✅ |
| P1-11 | Administradoras: renomear, contar | Task 10 | ✅ |
| P1-12 | Identificação com formato | Task 11 | ✅ |
| P1-13 | Ferramenta drawer completo | Task 12 | ✅ |
| P1-14 | "Quando usar" em dois campos + nova skill | Task 13 | ✅ |
| P1-15 | Payload livre e "Limpar" no Avançado | Task 22 | ✅ |
| P1-16 | Rota inicial por papel | Task 16 | ✅ |
| P1-17 | Hub de skills (números, ícone, resumo, problemas) | Task 19 | ✅ |
| P1-18 | Persona com aviso e modelo | Task 14 | ✅ |
| P1-19 | Orquestração com ajuda por campo | Task 15 | ✅ |
| P1-20 | Identidade técnica no Avançado | Task 21 | ✅ |
| P1-21 | Gestor não vê descrição técnica / ids | Task 20 | ✅ |

Nenhum item P0/P1 sem task. Os itens marcados P2 na análise (A1/B5, C1 parcial, C2, C12, C16 parcial, C17) estão fora do escopo da fase 2 por decisão do Felipe (§3 da análise) e aparecem no checklist da Task 31 com o gate explícito de aceite — correto, não é lacuna do plano.

## Reverse Traceability (plano → requisito/decisão)

| Plano | Origem | Status |
|---|---|---|
| Task 0 (base visual) | Decisão do Felipe 4 (canvas aprovado) | ✅ |
| Task 1 | análise §1.0.2 + §7 item 8 | ✅ |
| Tasks 2, 4, 5 | análise §5.3, §1 | ✅ |
| Tasks 3 | Decisão do Felipe 1 | ✅ |
| Tasks 6-22 | análise §7 (ver tabela acima) | ✅ |
| Tasks 23-28 (Copiloto) | Decisão do Felipe 5 (explicitamente fora do spec original, seção "Não objetivos" do `studio-design.md`, mas assumido nesta fase) | ✅ documentado, não é phantom |
| Tasks 29, 30 | Decisão do Felipe 4 (fluxo e Avançado no desenho aprovado) | ✅ |
| Task 31 | Decisão do Felipe 3 (aposentar Admin/Backdoor só com checklist fechada) | ✅ |
| Task 32 | Telas aprovadas (`Concierge.dc.html`): blocos Boas-vindas/Regras gerais/Base comum/Transbordo, sem task anterior que os cobrisse | ✅ (lacuna fechada nesta auditoria do plano, antes da execução) |

Nenhuma task phantom. Nenhum gold-plating: Task 32 reaproveita `RegrasAba`/`ConhecimentoAba` já existentes em vez de criar componentes novos; não introduz schema Go novo.

## Clarification Coverage

| Categoria | Status | Observação |
|---|---|---|
| Functional Scope | ✅ Covered | Objetivos/não objetivos/anti objetivos (spec §3) + Decisões do Felipe (plano) resolvem todo desvio do spec original (Copiloto, visual, "Concierge") |
| Domain & Data | ✅ Covered | Entidades (skill, condomínio, administradora, recurso, operação do Copiloto) com tipos explícitos em cada task |
| Interaction & UX (jornadas, vazio, erro) | ✅ Covered | Estados vazios (`SkillsVazio`), erro (`ErroComRetry`, 422/503/504 do Copiloto), confirmação de exclusão em toda task relevante |
| Acessibilidade | ⚠️ Partial | `aria-label`/`aria-pressed` especificados campo a campo, mas nenhuma task roda `axe-core`/`eslint-plugin-jsx-a11y` (pedido pela constraint nova de Atende) — não há verificação automatizada de acessibilidade no plano |
| Non-Functional: performance | ⚠️ Partial | Prazos explícitos em chamadas (90s Go, 95s front) mas sem orçamento de latência definido (constituição também deixa "a definir") — aceitável, não é lacuna deste plano |
| Non-Functional: observabilidade | ⚠️ Partial | Task 24 pede "log sem conteúdo da conversa" pontualmente; não há task dedicada a métricas/logs estruturados do Studio como um todo — proporcional ao risco (ferramenta interna), não bloqueante |
| Non-Functional: segurança/privacidade | ✅ Covered | Escopo e permissão resolvidos em todo caminho de escrita (telas e Copiloto), Task 23 filtra por papel/escopo/catálogo, Task 24 valida de novo no servidor |

Nenhuma lacuna de Functional Scope ou de segurança/compliance — sem CRITICAL por esta via.

## Constitution Compliance

| Principle | Status | Detail |
|---|---|---|
| I. Núcleo funcional, bordas impuras | ✅ | `internal/copiloto` (Task 23) é puro por desenho; harness/HTTP isolados na Task 24 |
| II. Postgres é fonte de verdade | ⚠️ WARN (herdada, não introduzida por este plano) | O Studio continua sobre arquivo (`okf/agentes/**`) e `admin.NewStore`/`mapeamento.json` para habilitações/orientações/payloads — persistência fora do Postgres. É decisão pré-existente (spec original §3: "Catálogo de skills no banco: continua em arquivo", e `mapeamento.json` já existe antes desta fase). Este plano não migra isso (fora de escopo, depende da Fundação) nem agrava o desvio. Ver recomendação. |
| III. Escrita só por fluxo confirmado | ✅ | `ValidarFluxoProposto` (Task 23) e recusa 422 reaplicam a regra até nas propostas do Copiloto |
| IV. Acesso por papéis com herança e escopo | ✅ | `Filtrar` (Task 23) cruza permissão × escopo × catálogo; front espelha com `usePodeEditar`/`pode` em toda tela nova |
| V. IA classifica e redige; código decide | ✅ | O Copiloto só propõe; `aplicarOperacao` (Task 26) e a validação do servidor (Task 24) são quem decide e grava |
| VI. Dado pessoal fica na origem | ✅ | Nenhuma task grava CPF/cadastro; Copiloto não guarda conversa em disco (Task 25: "conversa só na memória da sessão") |
| VII. Nenhum erro engolido, nenhuma chamada sem prazo | ✅ | Prazos explícitos em Go (90s) e front (`AbortSignal`); erros de gravação viram mensagem no cartão, nunca engolidos (Task 26) |
| Forbidden: `localStorage` como persistência | ✅ | Task 25 usa só para preferência de tela (`copilotoAberto`), dentro da exceção |
| Forbidden: `any`/`interface{}` | ✅ | `Campos map[string]any` (Task 23) já citado como exceção documentada (payload dinâmico do recurso) |

### Recomendação (não bloqueante)
A persistência em arquivo/JSON do Studio é uma dívida arquitetural anterior a esta fase e maior que o escopo dela (migrar para Postgres é trabalho da Fundação, citada no próprio spec). Sugiro uma linha nas Global Constraints do plano deixando isso explícito, para quem executar não confundir "manter `admin.NewStore`" com uma escolha nova desta fase. **Não bloqueio a execução por isso.**

## Verdict: WARN

### Critical Issues (must fix)
Nenhum.

### Warnings (recomendado, não bloqueia)
1. Persistência em arquivo (`mapeamento.json`, `okf/agentes/**`) continua fora do Postgres (constituição II) — dívida herdada, fora do escopo desta fase; deixar isso explícito no plano evita que pareça uma decisão nova.
2. Nenhuma task roda verificação automatizada de acessibilidade (`axe-core`/`eslint-plugin-jsx-a11y`), apesar da Global Constraint nova citar as regras do Atende.
3. Sem task dedicada de observabilidade (logs estruturados/métricas) para o Studio como um todo — proporcional ao risco de uma ferramenta interna, mas vale registrar para quando o Copiloto (que chama modelo) entrar em produção.

Nenhum desses três é consequência do trabalho desta auditoria (amend de boards); são observações sobre o plano já existente. Decisão: **seguir para `phase-gate`** com os warnings registrados; nenhum item CRITICAL foi encontrado, então nada foi alterado no plano nesta etapa de "fix CRITICAL".
