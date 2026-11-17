# Pre-Implementation Audit Report

**Spec**: `docs/superpowers/specs/2026-11-03-memoria-classificada-design.md` | **Plan**: `docs/superpowers/plans/2026-11-10-memoria-classificada.md` | **Date**: 2026-11-10
**Constitution**: `docs/superpowers/constitution.md` (v1.0)

## Summary

| Metric | Count |
|--------|-------|
| Spec requirements (regras das seções 3 a 10, decisões e metas) | 31 (+ 13 decisões do construtor, conferidas abaixo da tabela) |
| Plan tasks | 11 |
| Forward coverage (req → task) | 31/31 (100%) |
| Reverse traceability (task → req) | 11/11 (100%) |
| Constitution violations | 0 (1 crítica achada e corrigida no plano, ver abaixo) |

## Forward Traceability (Spec → Plan)

| Spec Item | Plan Task(s) | Status |
|-----------|-------------|--------|
| `padrao_de_uso` por caso, condomínio e unidade | 1 (`Padrao`, `Chave`), 5 (tabela, único), 6 (store), 9 (ocorrência) | ✅ |
| `preferencia` em lista fechada de 4 chaves com valores | 2 (`NormalizarPreferencia`), 4 (extrator) | ✅ |
| `pendencia` derivada da carteira, sem tabela | 3 (`Pendencia`), 8 (`pendenciasDe`) | ✅ |
| Contexto de sessão (oferta feita) | Decisão 5; 8 (`Sessao.Oferta`, `tomarOferta`) | ✅ |
| Identidade e vínculo nunca vêm da memória; só referência | Global Constraints; 1 (`DoAlvo`), 3 (`TestMontarContextoIgnoraUnidadeForaDoVinculo`), 8 (`alvoDoTurno`), 9 (`incluirOfertaAceita`) | ✅ |
| Classificação por regra no caminho quente (pedido concluído) | 3 (`PorRegra`), 9 (`sinalDoTurno`) | ✅ |
| Modelo barato em segundo plano, só preferência, esquema fechado | 4 | ✅ |
| Interface para o Jev entrar depois | 3 (`Classificador`), 9 (`Servico.Classificador`) | ✅ |
| Fórmula com meia-vida de 45 dias, O(1), sem log de eventos | 1 (`ValorEm`, `Registrar`), 5 (só `valor` e `atualizado_em`) | ✅ |
| 12 horas entre ocorrências | 1 (`TestRegistrarIgnoraOcorrenciaEmMenosDeDozeHoras`) | ✅ |
| Oferta com ≥ 2,0 e ≥ 2× o segundo | 1 (`TestEscolherOferta`) | ✅ |
| Recusa ÷ 2 e supressão de 14 dias | 1 (`Recusar`), 9 (`TestTurnoNaoDepoisDaOfertaRecusa`) | ✅ |
| Limpeza diária: padrão < 0,1, preferência > 180 dias | 2 (`PreferenciasVigentes`), 7 (`Limpar`, `Manter`) | ✅ |
| Números configuráveis | 1 (`Ajustes`), 7 (`AjustesDoAmbiente`), 11 (ligação no `Servico`) | ✅ |
| Bloco no prompt: preferências, até 3 padrões, pendências, marcado como dado | 3 (`MontarContexto`), 8 (`ContextoDeMemoria`, texto exato) | ✅ |
| Oferta na saudação ou mensagem vaga, sem pendência | 3 (`MensagemVaga`, `TestMontarContextoNaoOferece`), 8 | ✅ |
| Aba Memória com tipo, valor e data; esquecer preferência ou tudo | 10 (`console.js`) | ✅ (conferência manual, ver Warning 3) |
| Rotas de leitura com itens tipados, inclusive integração | 10 (`TestMemoriaLerDevolveItensComTipo`, `TestIntegracaoMemoriaDevolveItensComTipo`) | ✅ |
| Sem tela no Studio | Fora deste plano | ✅ |
| Sim abre o pedido do caso com a unidade oferecida | Decisão 6; 9 (`TestTurnoSimDepoisDaOfertaAbrePedido`) | ✅ |
| Escrita continua pedindo a confirmação do fluxo | 9 (asserção acrescentada pela auditoria, ver Critical 1) | ✅ (corrigido) |
| Não curto é recusa; outra resposta só expira | 3 (`LerResposta`), 9 (`TestTurnoOutraCoisaDepoisDaOfertaNaoPenaliza`, `TestTurnoSimSemOfertaAnteriorNaoAbrePedido`) | ✅ |
| Oferta só com unidade do vínculo; sem vínculo só padrão sem unidade | 1 (propriedade `TestPropriedadeOfertaRespeitaOAlvo`), 3, 9 (`TestOfertaAceitaSemVinculoNaoPreencheUnidade`) | ✅ |
| Esquecer tudo apaga preferências, padrões e cache numa chamada | 6 (`Store.Esquecer`, `TestEsquecerApagaTudo`), 10 | ✅ |
| Conversa em andamento não grava de volta o esquecido | Decisão 7; 10 (`TestEsquecerDuranteOTurnoNaoRessuscitaPadrao`) | ✅ |
| Esquecer uma preferência pela chave | 10 (caminho de hoje mantido) | ✅ |
| Fatos antigos fora da lista somem da leitura e a limpeza apaga; nada migrado | 6 (`TestLeituraDescartaFatoForaDaLista`, importação sem migrar), 7 | ✅ |
| Chave administradora + telefone | 5 (único inclui os dois), 6 (`TestEsquecerIsolaAdministradora`) | ✅ |
| Nenhum texto livre novo; nome curto validado | 2 (regex e 30 caracteres), 4 | ✅ |
| CPF e telefone mascarados antes da extração | caminho de hoje (`fatosSemPII` em `lembrar`), não tocado pelo plano | ✅ |
| Logs mascarados, com tipo e caso | Decisão 9; 7, 8, 9 | ✅ |
| Latência: 0 idas quente, 1 frio; nenhuma chamada de modelo nova | Global Constraints; 6 (`TestCacheAquecidoNoLoteTrazPadroes`), 8 e 9 (teste de hoje sem mudar número), 11 (`TestTurnoComOfertaNoPostgresIdasAntesDaResposta`) | ✅ |
| Meta: log de oferta e de resposta para taxa de aceite | Decisão 9; 8 (`memoria.oferta`), 9 (`memoria.resposta_oferta`) | ✅ |

Decisões do construtor da spec (seção 13), uma a uma: 1 condomínio do canal → `Alvo.CondominioID` (1, 8); 2 ocorrência = pedido concluído → 3, 9; 3 unidade = parâmetro `unidade` → 9; 4 doze horas → 1; 5 recusa divide → 1; 6 saudação por regra e só com motor → 3, 8; 7 sim e não por regra → 3, 9; 8 nome validado → 2; 9 reafirmar renova → 2 (`TestUpsertRenovaDataDoMesmoValor`); 10 pendência fora das rotas → 10 (`ParaLeitura` sem pendência); 11 sem competência → fora; 12 limpeza não mexe no cache → 7; 13 interface só para ocorrência → 3. Todas cobertas.

## Reverse Traceability (Plan → Spec)

| Plan Task | Spec Item | Status |
|-----------|-----------|--------|
| 1 Núcleo do padrão | Seção 5 (ranking), construtor 1, 4, 5 | ✅ |
| 2 Preferências | Seção 3 (lista), construtor 8, 9 | ✅ |
| 3 Oferta, classificador, leitura | Seções 4, 6, 7; construtor 6, 7, 10, 13 | ✅ |
| 4 Extrator | Seção 4 (segundo plano), seção 9 (envenenamento) | ✅ |
| 5 Esquema | Seção 3 (padrão), seção 10 (lote) | ✅ |
| 6 Store | Seções 8 e 10 | ✅ |
| 7 Limpeza e ajustes | Seção 5 (limpeza, configurável), seção 8 | ✅ |
| 8 Leitura no turno | Seções 6 e 7 | ✅ |
| 9 Escrita no turno | Seções 4, 5, 7 | ✅ |
| 10 Esquecer, rotas, Console | Seções 6 e 8 | ✅ |
| 11 Guarda de latência | Seção 10, metas | ✅ |

## Clarification Coverage

| Category | Status | Gap |
|----------|--------|-----|
| Functional Scope | ✅ Covered | objetivos, anti objetivos, fora da primeira versão |
| Domain & Data | ✅ Covered | 4 tipos, chaves, vida de cada um; escala: uma linha por (contato, caso, condomínio, unidade), limpeza diária |
| Interaction & UX | ⚠️ Partial | Console é JS sem teste automatizado; a Task 10 confere à mão |
| Non-Functional | ✅ Covered | latência (guarda), LGPD (esquecer, sem texto livre), observabilidade (decisão 9), confiabilidade (descarte por revisão) |

## Constitution Compliance

| Principle | Status | Detail |
|-----------|--------|--------|
| I. Núcleo funcional, bordas impuras | ✅ | `padrao.go`, `preferencia.go`, `oferta.go`, `leitura.go`, `incluirOfertaAceita`, `padroesDoTurno` recebem valores e `agora`; I/O no store, na limpeza e no `Servico` |
| II. Postgres, ent por padrão, sqlc com porquê | ✅ | tabela pelo ent, migration gerada; 3 queries sqlc, cada uma com `-- por que não ent` (lote a frio, lote pós envio, `power` por linha); esquecer e limpeza de fatos pelo ent |
| III. Escrita só por fluxo confirmado | ✅ (após correção) | ver Critical 1 |
| IV. Acesso por papéis | ✅ | rotas de memória mantêm `protegida(MemoriaVer/MemoriaEsquecer)` e a chave de serviço na integração; nenhuma rota nova |
| V. IA classifica, código decide e conta | ✅ | contador, ranking, oferta, sim e não em código; modelo só preenche preferência de lista |
| VI. Dado pessoal na origem, com expiração e exclusão | ✅ | ids e números; preferência vence em 180 dias; padrão some pelo decaimento; esquecer apaga tudo. `nome_preferido` é dado pessoal curto, coberto pela validade e pelo esquecer |
| VII. Nenhum erro engolido, chamada com prazo | ✅ | classificador com prazo de 1 s e WARN; padrão não gravado em ERROR e no `errors.Join`; limpeza com 30 s por rodada e ERROR; ajuste inválido derruba o boot |
| Forbidden patterns | ✅ | sem estado global (lista de preferências em `switch`; padrões adiados no `Servico`, como a carteira); sem SQL solto; sem JSON como persistência |
| Quality priorities | ✅ | confiabilidade (nada executa sem sim, descarte por revisão), segurança (vínculo do turno, esquema fechado), custo (nenhuma chamada de modelo nova) |

## Verdict: WARN

### Critical Issues (corrigidos no plano nesta auditoria)
1. **O "sim" da oferta podia ser lido como confirmação de escrita.** A Task 9 testava só que o pedido abria. Para um caso de escrita (agendamento de mudança), nada garantia que o motor ainda pediria a confirmação do fluxo. Corrigido: `TestTurnoSimDepoisDaOfertaAbrePedido` agora exige o pedido em `coletando` ou `aguardando_confirmacao` e nenhum recurso de escrita chamado; acrescentado `TestTurnoSimSemOfertaAnteriorNaoAbrePedido`.

### Warnings (recommended to fix)
1. **Id da unidade.** O padrão guarda o valor do parâmetro `unidade` do pedido; o vínculo usa `gruvi.Unidade.ID`. Se forem ids diferentes, padrão com unidade nunca casa (falha segura: só perde a oferta). A Task 9 manda conferir antes de implementar e levar à revisão.
2. **Uma réplica escrevendo memória.** O cache de padrões herda o limite do cache de fatos (README, "Limite conhecido"). Registrado em "Fora deste plano".
3. **Console sem teste automatizado.** A Task 10 confere à mão; o contrato das rotas tem teste Go.
4. **Task 11 pode passar de primeira.** Se a leitura da carteira no `montar` já usa o cache, o teste novo nasce verde; a asserção da oferta depois da tabela garante que o caminho foi exercitado.
5. **Plano maior que a spec (~56 KB × 14 KB).** Proporcional às 31 regras, às 13 decisões do construtor e às 11 tasks; os blocos de código são assinaturas e o texto exato do prompt, não corpos.
