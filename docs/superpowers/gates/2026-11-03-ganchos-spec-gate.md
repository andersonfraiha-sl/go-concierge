# Post-Spec Gate: Ganchos

Data: 2026-11-03 · Spec: `docs/superpowers/specs/2026-11-03-ganchos-design.md` · Constituição: v1.0

## Content Quality
- ❌ **Sem detalhe de implementação**: Go (4×), Postgres (2×), além de ent, sqlc, pgx, slog, OTel, react-hook-form, Zod e Vitest.
  **Waiver**: o próprio pedido do Felipe fixa a tecnologia (auditoria no Postgres, formulário com react-hook-form + Zod, ganchos fixos em Go, cache igual ao `admin.Cache`), e Go, Postgres, ent e sqlc estão na constituição como *Non-negotiable*. A spec descreve uma peça de infraestrutura que mora dentro desse código; reescrever em termos neutros apagaria as decisões e o mapeamento das regras que migram.
- ✅ Seções obrigatórias: cenários e papéis (§2, exemplos de regra em §7), requisitos (§3 Objetivos, §4 a §9), critérios de sucesso (§13).
- ✅ Nenhuma seção vazia.

## Requirement Quality
- ✅ Requisitos testáveis: cada objetivo de §3 tem critério em §13 (fixos migrados com os testes atuais intactos, PII fora do modelo e dos logs, regras de exemplo ponta a ponta, 403 para administradora, zero idas no turno quente).
- ❌ Critérios agnósticos de tecnologia: citam `BenchmarkTurnoMockQuente`, `go test`, `grep`. Mesmo waiver acima.
- ✅ Critérios mensuráveis: 0 idas antes do envio, p99 ≤ 50 µs por evento, ≤ 0,5 ms por turno, ≤ 20 MB de índice, ± 2 ms no p50.
- ✅ Nenhum marcador TBD/TODO/NEEDS CLARIFICATION.

## Clarification Coverage
- ✅ Escopo funcional: objetivos, não objetivos (inclui administradora criar regra como fase posterior), anti objetivos, papéis.
- ✅ Domínio e dados: regra, versão, auditoria, cofre do turno, índices e retenção (§7, §8).
- ✅ UX: lista, formulário com frase de resumo, histórico, estados de erro (`422`, `409`, sem permissão) (§9).
- ✅ Não funcional: orçamento de desempenho (§10), política de falha por evento (§11), observabilidade e testes (§12), segurança (PII, RBAC, fixo não desliga).

## Constitution Alignment
- ✅ I (núcleo puro): ganchos são funções puras; I/O só no cache e na auditoria.
- ✅ II: tudo no Postgres; sqlc só na inserção em lote da auditoria, com o porquê.
- ✅ III: `escrita_confirmada` e `fluxo_valido` reforçam o Confirmar; regra não afrouxa fixo.
- ✅ IV: `ganchos.editar` checada contra o escopo `superlogica`; histórico por escopo.
- ✅ V: desfecho decidido em código; IA não decide gancho.
- ✅ VI: CPF e telefone fora do modelo, da memória e do histórico persistido.
- ✅ VII: prazo por gancho, falha nunca engolida (log, métrica, política explícita).
- ✅ Nenhum padrão proibido (sem estado global mutável: cache injetado; sem `localStorage`; sem `any` no evento fora do payload do Nexus).
- ✅ Prioridades: confiabilidade (transbordo nunca bloqueado, comportamento migrado igual) e segurança (falha fechada nos fixos) aparecem nos critérios.

## Readiness
- ⚠️ Aprovação do Felipe pendente.
- ⚠️ Dois itens em aberto (§16): edição de regras em implantação dedicada e retenção da auditoria. Não bloqueiam o plano: o padrão multi administradora não depende deles, e a retenção é um número do job.

## Decisão: **PASS com waiver**
Os únicos ❌ são o detector de tecnologia, dispensados acima com justificativa (mesmo critério do gate da Fundação). Nenhum BLOCK a corrigir na spec. Segue para `writing-plans` depois da aprovação do Felipe.
