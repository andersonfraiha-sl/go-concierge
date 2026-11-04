# Pre-Implementation Audit Report

**Spec**: `docs/superpowers/specs/2026-11-04-skills-base-design.md` | **Plan**: `docs/superpowers/plans/2026-11-04-skills-base.md` | **Date**: 2026-11-04
**Constitution**: `docs/superpowers/constitution.md` (v1.0)

## Summary

| Metric | Count |
|--------|-------|
| Spec requirements (regras + decisões do desenho + metas) | 30 |
| Plan tasks | 21 |
| Forward coverage (req → task) | 30/30 (100%) |
| Reverse traceability (task → req) | 21/21 (100%) |
| Constitution violations | 0 (1 observação no princípio II) |

## Forward Traceability (Spec → Plan)

| Spec Item | Plan Task(s) | Status |
|-----------|-------------|--------|
| Camadas: base + administradora + condomínio, composição em ordem | 2 (`Retrato`), 9 (`DoCondominio`: condomínio → administradora → base) | ✅ |
| "Cada camada guarda só o que mudou" | Decisão 2; 2 (`Comparar(acima, cabeça)`), 8 (`acima_id`) | ✅ (ver Warning 1) |
| Personalizar textos e fluxo das etapas | 2 (trechos), 10 (editor), 11 (`SalvarRascunho`/`PublicarCamada`) | ✅ |
| Camadas de baixo nunca ligam ferramenta | Global Constraints; 4 (`Validar`, `FERRAMENTA_FORA_DA_BASE`); 10 (`permitidas`); 11, 16, 17 (todos os caminhos de escrita) | ✅ |
| Aviso "essa skill tem atualizações" com resumo | 11 (`cascatear`/atualizações), 13 (`AvisoDeAtualizacoes`, selo no card), 18 (`GET /api/atualizacoes`) | ✅ |
| Cascata: administradora decide primeiro; condomínio recebe o que ela aceitou | Decisão 5; 11 (`cascatear`), 16 (`TestCondominioSoVeOQueAAdministradoraAceitou`) | ✅ |
| Prazo de 14 dias, configurável | Decisão 6; 16 (`VencerPrazos`, `RodarPrazos`), 19 (`SKILLS_PRAZO_DIAS`) | ✅ |
| Nada muda sozinho, exceto obrigatório | Decisão 7; 4 (`so_base` sem decisão = manter), 6/9 (turno só lê versão publicada) | ✅ |
| Tela de comparação de três vias por trecho | 12 (`ComparacaoPage`, três colunas) | ✅ |
| `so_base` vem marcado aceitar; só a pessoa mudou não aparece | 2 (tabela de `Comparar`), 12 (`escolhasIniciais`, `acceptsBaseOnlyByDefault`) | ✅ |
| Conflito: ficar com a minha, aceitar a nova, editar a mistura | 4 (`Aplicar` com `Misturar`), 12 (`MisturaDialog`) | ✅ |
| Aceitar tudo, aceitar parte, recusar tudo | 12 (`aceitarTudo`, `recusarTudo`, por trecho) | ✅ |
| Recusar não apaga; revisável até a próxima versão da base | 8 (`CriarAtualizacao` marca substituída), 16 (`TestRecusadaPodeSerRevistaAteAProximaVersao`) | ✅ |
| Granularidade: texto, etapa, ferramenta | Decisão 3; 2 (`Trechos`) | ✅ (ver Warning 2) |
| Obrigatório: versão ou trechos marcados pela Superlógica | 11 (`PublicacaoBase.Obrigatorios`), 14 (`PublicarBaseDialog`) | ✅ |
| Obrigatório entra em todas as camadas, mesmo personalizado, sem esperar | 4 (`AplicarObrigatorio`), 11 (`TestPublicarBaseComObrigatorio...`) | ✅ |
| Aviso do que foi trocado + motivo; versão antiga no histórico | 8 (`skill_avisos`), 11 (`NovoAviso`), 13 (`showsMandatoryNoticeWithReason`) | ✅ |
| Papéis: Superlógica, administradora, condomínio, operador | Decisão 12; 3 (permissões), tabela de proteção, 17/18 (403 por escopo) | ✅ |
| Permissões que faltarem entram junto | 3 (`skills.editar_etapas`, `skills.decidir_atualizacao`, `skills.publicar_base`) | ✅ |
| Revoga "administradora não muda estrutura" para camadas | 3 (gestor com `editar_etapas`), 10 | ✅ |
| Histórico: quem, quando, o que aceitou/recusou; voltar | Decisão 8; 8 (`Historico`), 11 (`Voltar`), 13 (`HistoricoDaCamada`) | ✅ |
| Rascunho e publicação; só publicação avisa | Decisão 1; 10 (autosave + Publicar), 11 | ✅ |
| Validação no resultado composto; junção inválida não salva | 4 (`Validar`), 12 (`showsServerProblemsOnSection`), 16 | ✅ |
| Obrigatório validado antes; publicação lista camadas que quebram | 11 (`ErrCamadasQuebram`), 14 (`showsBrokenLayersOnConflict`), 18 | ✅ |
| Prazo vencido com personalização: só desce o não personalizado | 16 (`TestVencerPrazosAplicaSoOQueAAdministradoraNaoMexeu`, `...DeixaConflitoPendente`) | ✅ |
| Identidade estável de etapas; apagar × alterar é conflito | 1 (`id` no passo), 2 (`TestCompararApagarEAlterarEhConflito`) | ✅ |
| Ciclo de vida: skill nova chega desligada; removida vira aviso obrigatório com data | Decisão 11; 11 (`DesligarSkillNova`, `EncerraEm`), 9 (`TestDoCondominioEscondeSkillEncerrada`), 13 | ✅ |
| Orientações por condomínio passam à camada do condomínio | Decisão 10; 6, 15, 16 (`ImportarOrientacoes`) | ✅ |
| Meta: nenhuma personalização se perde (exceto obrigatório, com aviso) | 4 (propriedade `Aplicar` manter = identidade), 16 | ✅ |
| Meta: até 10 trechos em < 2 min | 12 (`decidesTenSectionsWithAcceptAllInOneClick`), 21 (cronômetro) | ✅ |
| Meta: 0 tempo e 0 ida ao banco a mais no turno | 6 (`TestTurnoNoPostgresIdasAntesDaResposta` com camadas), 9 (memória), 19, 21 | ✅ |
| Meta: toda decisão no histórico | Decisão 8; 16 (versão por decisão, `RegistrarDecisao`) | ✅ |
| Anti-metas (apagar sem ver, ler a skill inteira, condomínio parado) | Review Focus 1; 12 (só trechos que mudaram); decisão 6 | ✅ |

## Reverse Traceability (Plan → Spec)

| Plan Task | Spec Item | Status |
|-----------|-----------|--------|
| 1 Ids estáveis + `TextosNoCaso` | Identidade estável; composição | ✅ |
| 2 Trechos e comparação | Tela de comparação, granularidade | ✅ |
| 3 Permissões | Quem faz o quê | ✅ |
| 4 Aplicar, obrigatório, validar | Atualização, obrigatório, validação no composto | ✅ |
| 5 Esquema | Histórico, rascunho, atualização (armazenamento delegado) | ✅ |
| 6 Turno usa efetiva | Meta de latência | ✅ |
| 7 Contratos front | Tela, aviso | ✅ |
| 8 Repositório | Histórico, atualização | ✅ |
| 9 Composição em memória | Meta de latência, ciclo de vida | ✅ |
| 10 Editor da camada | Personalizar, rascunho/publicar | ✅ |
| 11 Serviço camada/base | Publicar, obrigatório, cascata, nova desligada | ✅ |
| 12 Comparação | Tela de comparação | ✅ |
| 13 Aviso e histórico | Aviso, histórico | ✅ |
| 14 Publicar base | Obrigatório, ciclo de vida | ✅ |
| 15 Orientação | Orientações na camada do condomínio | ✅ |
| 16 Decidir, prazo, semente | Cascata, prazo, histórico; semente = ligação com o que existe (delegada) | ✅ |
| 17, 18 API | Todas as interações | ✅ |
| 19 Boot | Prazo configurável, ligação com o turno | ✅ |
| 20 Contrato golden | Confiabilidade (contrato Go ↔ Zod) | ✅ |
| 21 Smoke | Metas | ✅ |

## Clarification Coverage

| Category | Status | Gap |
|----------|--------|-----|
| Functional Scope | ✅ Covered | objetivos, fora do escopo, anti-metas, 4 papéis |
| Domain & Data | ✅ Covered | entidades e estados explícitos no plano (`origem`, `estado`); escala: plano assume dezenas de skills × milhares de condomínios e compõe por versão distinta (Task 9) |
| Interaction & UX | ⚠️ Partial | carregamento/erro/vazio cobertos por task; acessibilidade não citada na spec. O plano herda os componentes de hoje (teclado no canvas, rótulos); sem critério novo |
| Non-Functional | ✅ Covered | desempenho (meta de latência), segurança (papéis, ferramentas), observabilidade (decisão 14, observação do gate da spec), confiabilidade (transação serializável, idempotência do prazo) |

## Constitution Compliance

| Principle | Status | Detail |
|-----------|--------|--------|
| I. Núcleo funcional, bordas impuras | ✅ | `Comparar`, `Aplicar`, `AplicarObrigatorio`, `Validar`, `Efetivar` puros (Tasks 2, 4, 9); serviço é a casca |
| II. Postgres é a fonte de verdade | ✅ (observação) | camadas, versões, atualizações e avisos no Postgres via ent; migration goose gerada. A base continua nos arquivos do catálogo, mas só como **rascunho** (decisão já vigente na spec do Studio: "Catálogo de skills no banco: continua em arquivo"); toda versão publicada, que é o que o turno usa, vai para o Postgres. Não é persistência nova fora do banco |
| III. Escrita só por fluxo confirmado | ✅ | `okf.ValidarFluxo` (gravar sem confirmar) roda no composto de toda camada (Task 4) |
| IV. Acesso por papéis com escopo | ✅ | toda rota com `protegida`/escopo da camada; listagem filtrada por `Permitido` (Task 18) |
| V. IA classifica, código decide | ✅ | nenhuma decisão de camada passa por modelo |
| VI. Dado pessoal na origem | ✅ | camadas guardam textos de skill, sem dado de morador |
| VII. Nenhum erro engolido, chamada com prazo | ✅ | erros tipados mapeados na API; `RodarPrazos` com prazo por rodada e log ERROR; composição que falha loga ERROR e cai para a camada de cima |
| Forbidden patterns | ✅ | sem estado global (`Efetivas` é instância injetada); sem SQL solto; sem `localStorage` |
| Quality priorities | ✅ | confiabilidade (preflight do obrigatório, idempotência, 409), segurança (ferramentas, escopo), custo (nenhuma chamada de modelo nova) |

## Verdict: WARN

### Critical Issues (must fix)
Nenhum.

### Warnings (recommended to fix)
1. **"Cada camada guarda só o que mudou" × retrato inteiro (decisão 2).** O comportamento é o pedido (o que a camada mudou = `Comparar(acima, cabeça)`), e a spec delega o armazenamento ("diferença ou cópia inteira"). Conferir na revisão da Task 8 que nenhuma tela mostra o retrato inteiro como "o que mudou".
2. **Trecho de texto por campo, não por parágrafo (decisão 3).** A spec diz "texto por parágrafo ou linha" e deixa o construtor refinar. Os campos de hoje são curtos; se `quandoUsar` crescer, a tela passa a comparar o campo inteiro. Registrado em "Fora deste plano".
3. **Prazo também na camada do condomínio (decisão 6).** A spec fala do prazo da administradora; o plano estende ao condomínio para cumprir a anti-meta. É escolha do plano: o Felipe pode reverter tirando condomínio de `AtualizacoesVencidas`.
4. **Roteador (`rotas.md`) não personalizado (decisão 13).** A personalização de "quando usar" vale no motor de pedidos e na projeção, não no roteador por texto. Marcado com `ponytail:` na Task 6.
5. **Tasks sem ciclo de teste próprio.** Task 5 (esquema) se apoia nos testes de migration existentes e Task 20 só acrescenta casos; aceitável, mas o revisor deve rodar `go test ./internal/banco/...` na 5.
6. **Plano maior que a spec (~82 KB × 7,7 KB).** Proporcional à quantidade de regras (30) e de tasks (21); os blocos de código são assinaturas, não corpos.
