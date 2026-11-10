# Post-Spec Gate: Memória classificada

Data: 2026-11-10 · Spec: `docs/superpowers/specs/2026-11-03-memoria-classificada-design.md` (200 linhas, 16 seções) · Pesquisa: `docs/superpowers/pesquisas/2026-11-10-memoria-de-agentes.md` · Constituição: v1.0

Conferências rodadas no arquivo: `grep` de termos de tecnologia, de marcadores pendentes e de travessão; contagem de seções; leitura cruzada com a constituição e com o código de `main` (10/11).

## Content Quality

| Item | Status | Evidência |
|---|---|---|
| Sem detalhe de implementação | ⚠️ WARN (aceito) | `grep -i "postgres\|sqlc\|ent\|json\|regex\|slog\|sql"`: só "Postgres" na linha 198 (seção Estado, descreve o que já existe). "cache" (6×) e "lote" (6×) aparecem nas seções 10 e 12: são o vocabulário da regra dura de latência do projeto ("lote a frio", "lote pós envio"), não escolha nova. `/api/memoria` e `/api/memorias` (linha 91) nomeiam rotas que já existem, para dizer onde o item aparece. Nenhuma linguagem, biblioteca ou tabela como decisão |
| Seções obrigatórias | ✅ PASS | cenários: seção 1 (exemplo) e seção 7 (exemplos A a E); requisitos: seções 3 a 10; critérios de sucesso: seção 14 (Metas) |
| Nenhuma seção vazia | ✅ PASS | as 16 seções (`grep -n "^## "`) têm conteúdo |

## Requirement Quality

| Item | Status | Evidência |
|---|---|---|
| Requisitos testáveis | ✅ PASS | números fechados: meia-vida 45 dias, oferta com valor ≥ 2,0 e ≥ 2× o segundo, recusa divide por 2 e suprime 14 dias, limpeza abaixo de 0,1, preferência 180 dias, 12 horas entre ocorrências; lista fechada de 4 preferências com valores; regras de oferta com 5 exemplos de resultado observável |
| Critérios agnósticos de tecnologia | ✅ PASS | metas falam de idas ao banco por turno, chamadas de modelo, leitura vazia depois de esquecer, unidade fora do vínculo e log de aceite |
| Critérios mensuráveis | ✅ PASS | "0 idas no contato quente, 1 no frio"; "nenhuma chamada de modelo nova antes da resposta"; "nenhuma oferta cita unidade fora do vínculo"; taxa de aceite e de recusa por log |
| Sem marcadores pendentes | ✅ PASS | `grep -n "TBD\|TODO\|NEEDS CLARIFICATION"`: 0. A seção "Em aberto" do rascunho saiu; as 3 perguntas foram respondidas: Jev (decisão 1), stack e vínculo (seção 3 e decisão do construtor 1: identidade do Gruvi no turno), mandar direto ou confirmar (decisão 3) |
| Estilo | ✅ PASS | `grep -n "—\|--\| - \|de para"`: só as linhas de separador de tabela (40, 51, 87) |

## Clarification Coverage

| Item | Status | Nota |
|---|---|---|
| Escopo funcional | ✅ PASS | objetivos, não objetivos, anti objetivos (seção 2), fora da primeira versão (seção 15) |
| Domínio e dados | ✅ PASS | 4 tipos com fonte e vida (seção 3); chave por administradora + telefone (seção 9); o que acontece com os fatos antigos (seção 8) |
| Interação | ✅ PASS | oferta, sim, não e resposta que não é nenhum dos dois (seção 7); Console e rotas (seção 6) |
| Não funcional | ✅ PASS | latência (seção 10), LGPD e envenenamento (seção 9), observabilidade (meta de log de oferta e resposta, seção 14) |

## Coerência com o código (conferido em `main` de 10/11)

| Item | Status | Nota |
|---|---|---|
| Condomínio da conversa | ✅ PASS | `atende.Servico.contatoDaMensagem` e `fixarCondominio` resolvem o condomínio pelo mapeamento antes do turno; a spec registra (construtor 1) que a memória não escolhe condomínio |
| Sinal de ocorrência existe | ✅ PASS | `turnoComPedidos` conhece o caso, a carteira antes e depois (`estadosDe`) e o desfecho `pedidos.Concluido`; todos os casos de `okf/agentes/concierge/agentes/*` com uso têm `caso.yaml` |
| Lote a frio e pós envio existem | ✅ PASS | `Servico.aquecer` (memória e pedidos numa ida) e `gravarDepoisDaResposta`/`gravarEmLote`; guarda `TestTurnoNoPostgresIdasAntesDaResposta` |
| Esquecer pela integração | ✅ PASS | `DELETE /integracao/v1/memoria` usa o mesmo handler de `DELETE /api/memoria` (`integracao_morador.go`) |

## Constitution Alignment

| Princípio | Status | Nota |
|---|---|---|
| III. Escrita só por fluxo confirmado | ✅ PASS | oferta pergunta; o sim abre o pedido, que segue a confirmação do próprio fluxo (seção 7) |
| V. IA classifica, código decide e conta | ✅ PASS | ocorrência por regra, contador e oferta em código, modelo só extrai preferência de lista (seção 4) |
| VI. Dado pessoal na origem, com expiração e exclusão | ✅ PASS | ids e números; preferência vence em 180 dias; padrão some pelo decaimento; esquecer apaga tudo (seções 5, 8, 9). Exceção registrada: `nome_preferido` curto e validado |
| VII. Nenhum erro engolido | ➖ N/A na spec | fica para o plano |
| Prioridades | ✅ PASS | confiabilidade (nada executa sem sim), segurança (vínculo do turno, esquema fechado), custo (nenhuma chamada de modelo nova) |

## Readiness

| Item | Status | Nota |
|---|---|---|
| Spec decidida | ✅ PASS | "decidido em 10/11/2026" (linha 4); decisões numeradas com o porquê (seção 12) |

## Decisão: **PASS** (1 observação não crítica)

1. "cache" e "lote" ficam na spec porque são o vocabulário da regra de latência do projeto; o plano diz como.

Segue para `superpowers:writing-plans`.
