# Pre-Implementation Audit Report

**Spec**: `docs/superpowers/specs/2026-11-03-studio-design.md` + `go-concierge/docs/superpowers/specs/2026-11-03-fundacao-design.md` (main) | **Base aprovada**: `docs/superpowers/pesquisas/2026-11-04-studio-na-fundacao.md` | **Plan**: `docs/superpowers/plans/2026-11-04-studio-na-fundacao.md` | **Date**: 2026-11-04
**Constitution**: `docs/superpowers/constitution.md` (v1.0)

Este plano é de integração: os requisitos vêm da pesquisa aprovada (seções 1 a 4 e as 10 tarefas da seção 6), presos às duas specs. A rastreabilidade abaixo usa a pesquisa como lista de requisitos e confere cada um contra a spec de onde ele nasce.

## Summary

| Metric | Count |
|--------|-------|
| Requisitos (pesquisa §1 notas c/d/e, §3 gaps 1-7, §4 quebras 1-3, §6 tarefas 1-10, specs: acesso e critérios de pronto ligados) | 26 |
| Plan tasks | 10 |
| Forward coverage (req → task) | 26/26 (100%) |
| Reverse traceability (task → req) | 10/10 (100%) |
| Constitution violations (BLOCK) | 0 |
| Desvios da pesquisa, declarados no plano | 7 |

## Forward Traceability (Spec/Pesquisa → Plan)

| Item | Plan Task(s) | Status |
|------|-------------|--------|
| P6.1 Merge, 4 conflitos, `rotear` com Studio/Copiloto, `Ler(ctx)`, fake, caso `/` | Task 1 | ✅ |
| P4.1 Panic por rota duplicada (`rotasLegado`) | Task 1 `TestRotearNaoEntraEmPanico` | ✅ |
| P4.2 Não compila (`Ler()`, `admin.NewStore`) | Task 1 Step 3 | ✅ |
| P4.3 `TestRotasAutenticacao` espera 200 em `/` | Task 1 Step 4 (302 + `Location`, `/admin` público) | ✅ |
| Achado novo: métodos `anatomia/criar/excluir/mover` duplicados entre `main.go` (main) e `legado.go` | Task 1 Step 2, decisões 1 e 2 | ✅ (não estava na pesquisa) |
| P1 nota d: `casoDeUso.resumo/icone` mantidos | Task 1 Step 2 | ✅ |
| P6.2 Proteger rotas de skills (linhas 2-9) | Task 2 | ✅ |
| P6.3 / P3 gap 1: `/api/eu` pelo `Acesso` (permissões, `administradoraId`, papel, dev) | Task 3 | ✅ |
| P3 gap 2: nome do usuário | Task 3 (`Acesso.Nome` na mesma consulta) | ✅ |
| P3 gap 3: mais de uma administradora → 409 | Task 3 (`ADMINISTRADORA_AMBIGUA`) | ✅ |
| P3 gap 4: lista plana de permissões basta | Task 3 (`PermissoesEmAlgum`) | ✅ |
| P3 gap 5 / P6.5: usuários de dev semeados, guarda vira conjunto, front manda `X-Dev-Usuario` | Task 5 | ✅ |
| P3 gap 6 / P6.4: Copiloto no `Acesso` (spoofing, bloqueante de segurança) | Task 4 `TestCopilotoIgnoraCabecalhoDePapel` | ✅ |
| P3 gap 7: tela de 401 e cache limpo na troca | Task 8 | ✅ |
| P1 nota e / P6.6: criar com POST, 409 legível | Task 6 | ✅ |
| P1 nota c / P6.7: GET orquestração em algum escopo, PUT na Superlógica | Task 7 | ✅ |
| P5 / P6.9: fixtures golden Go ↔ Zod | Task 3 (helper + `eu`), Task 9 | ✅ |
| P6.10: smoke por papel | Task 10 | ✅ (manual) |
| Studio §6: o Studio pergunta ao servidor quem é e esconde o que não pode; servidor recusa | Tasks 2, 3, 4, 7 | ✅ |
| Studio §8: gestor sem botão de estrutura e sem Avançado; com concessão o Avançado aparece | Task 3 (`PermissoesEmAlgum` com concessão), Task 5 (`gestor-avancado`), Task 10 | ✅ |
| Fundação §4: cada handler declara permissão e alvo; 403 sem permissão | Tasks 2, 4, 7 (tabelas `sem`/`com`) | ✅ |
| Fundação §7: gestor de A recebe 403 em dado de B | Task 4 (retrato só de A), testes da main mantidos | ✅ |
| Fundação §4: `AUTH_MODE=dev` só em dev; semeado fora de dev = 401 | Task 5 `TestUsuarioSemeadoSoValeEmModoDev` (4 usuários) | ✅ |
| Constraint do pedido: latência 0 idas no turno quente | Global Constraints + Task 1 Step 5 (`TestTurnoNoPostgresIdasAntesDaResposta`) | ✅ |
| Constraint do pedido: nada antigo apagado (Admin, Backdoor, `mcp/`) | Global Constraints, Task 1 (`/admin`, `/backdoor`), Task 10 | ✅ |
| Constraint do pedido: `-race` em Docker trixie com socket | Comandos + Tasks 1 e 4 | ✅ |

## Reverse Traceability (Plan → Spec)

| Plan Task | Origem | Status |
|-----------|--------|--------|
| 1 Merge | P6.1, P4 | ✅ |
| 2 Skills protegidas | P6.2, Studio §6, Fundação §4 | ✅ |
| 3 `/api/eu` | P6.3, P3 gaps 1-4, Studio §6 | ✅ |
| 4 Copiloto | P6.4, P3 gap 6, constituição IV | ✅ |
| 5 Usuários de dev | P6.5, P3 gap 5 | ✅ |
| 6 POST para criar | P6.6, P1 nota e | ✅ |
| 7 Orquestração | P6.7, P1 nota c | ✅ |
| 8 401 e troca | P6.8, P3 gap 7 | ✅ |
| 9 Contrato golden | P6.9, P5 | ✅ |
| 10 Smoke | P6.10 | ✅ |

Nenhuma task fantasma. `Conceder` (Task 5) não estava na pesquisa; é o mínimo para a concessão de dev existir sem semear administradora em produção (decisão 5).

## Clarification Coverage

| Category | Status | Gap |
|----------|--------|-----|
| Functional Scope | ✅ Covered | objetivos, fora do escopo e papéis (admin, construtor, gestor, gestor com Avançado) explícitos |
| Domain & Data | ✅ Covered | `Acesso`, atribuições, concessões, FK de `concessoes` tratada; catálogo segue em arquivo por decisão |
| Interaction & UX | ⚠️ Partial | 401 e 409 cobertos; o usuário sem papel (200 com nada) não tem tela própria de "sem acesso" além da que já existe |
| Non-Functional | ⚠️ Partial | segurança e latência cobertas; observabilidade só troca o campo do log do Copiloto para `usuario_id`; nenhuma meta nova |

## Constitution Compliance

| Principle | Status | Detail |
|-----------|--------|--------|
| I. Núcleo funcional, bordas impuras | ✅ | `PermissoesEmAlgum`, `PapelPrincipal`, `montarEu`, `quemDoCopiloto`, `deveTentarDeNovo` puras; I/O só em handler/store |
| II. Postgres é a fonte de verdade | ⚠️ JUSTIFY | catálogo de skills continua em arquivo (`okf/agentes`), decisão registrada nas duas specs; o plano não acrescenta persistência fora do Postgres. Usuários e concessão de dev vão para o banco |
| III. Escrita só por fluxo confirmado | ✅ | não tocado; Copiloto continua só propondo |
| IV. Acesso por papéis com herança e escopo | ✅ | é o assunto do plano; `X-Studio-Papel` some dos dois lados |
| V. IA classifica, código decide | ✅ | escopo do Copiloto vem do `Acesso`, não do modelo |
| VI. Dado pessoal na origem | ✅ | não tocado; log do Copiloto sem conteúdo |
| VII. Nenhum erro engolido, chamada com prazo | ✅ | `Conceder` com `timeoutCarregar`; erro do `semearAcessoDev` derruba o boot de dev; erros de `/api/eu` vão para `responderErroInterno` |
| Forbidden: `localStorage` como persistência | ⚠️ | `studio.usuarioDev` só em dev, mesma natureza do `studio.papelDev` atual (preferência de tela de dev). Aceitável pela exceção; registrar no PR |
| Forbidden: estado global mutável | ✅ | `usuariosSemeadosDev` é `map` de pacote, mas só lido; se o Biome/vet do time preferir, trocar por `switch` numa função |
| Tech constraints (Go, Postgres 18, ent+sqlc, React+MUI) | ✅ | |

## Verdict: WARN

### Critical Issues (must fix)

Nenhum.

### Warnings (recommended to fix)

1. **`corepack@latest` sem pin.** O pnpm fica pinado pelo `packageManager`, mas o corepack muda com o tempo. Alternativa: pinar a versão do corepack conferida na execução, como pede a regra de versões da constituição.
2. **`atlas.sum` (Task 5).** O plano diz "ou a mesma ferramenta que a main usou" para regerar o hash. O implementador precisa descobrir como a `20261103170000_usuario_dev.sql` entrou no `atlas.sum` antes de rodar. Se o Atlas não estiver no contêiner, falha na Task 5.
3. **Conflito de tabela em `rotas_acesso_test.go`.** Tasks 2, 3, 4, 5 e 7 mexem no mesmo arquivo. O plano fixa a ordem de integração (2, 7, 3, 6), mas cada rebase pede atenção a casos duplicados ou perdidos.
4. **Sete desvios da pesquisa** (decisões 1 a 7 do plano). Todos têm motivo verificado no código (`main.go:425-575`, FK de `concessoes`, `render.tsx:21`), mas a pesquisa foi aprovada sem eles. Vale o Felipe ler as decisões antes de executar.
5. **Task 10 é manual.** É o único jeito de ver as telas por papel com o Postgres de verdade. As regras dela já têm teste automático nas Tasks 2 a 9.
6. **Studio spec §9 diz que Backdoor e Admin "saem quando o Studio cobrir tudo"**, e o plano da fase 2 tinha a Task 31 para isso. Este plano proíbe apagar, por pedido do Felipe. Não contradiz a spec (ela não fixa quando), mas a Task 31 da fase 2 fica pendente.
7. **Pesquisa pedia o "gestor com concessão" na tabela de `/api/eu`.** O plano cobre isso no teste puro `TestPermissoesEmAlgum`, porque o fake só ganha concessões na Task 7. Se a Task 7 integrar antes da 3 (ordem prevista), dá para pôr a linha `gestor-av` em `TestEu` também.
8. **Nota b da pesquisa** (front libera persona com `editar_textos`, back exige `editar_fluxo`) fica fora, como a pesquisa decidiu. Fica listada em "Fora deste plano".

Audit result: **WARN**. Nenhum bloqueio. Seguir para o gate com os avisos registrados.
