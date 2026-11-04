# Skills base com atualizações em camadas Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A Superlógica publica a skill base, a administradora e o condomínio personalizam por cima em camadas guardadas no Postgres, e cada atualização da base chega como comparação de três vias por trecho, com cascata, prazo de 14 dias, trecho obrigatório e histórico; o concierge usa a skill composta já pronta, sem ida ao banco a mais.

**Architecture:** A base continua nos arquivos do catálogo (`okf/agentes`), que viram o rascunho da Superlógica; **Publicar** tira um retrato (textos, fluxo, ferramentas e o `caso.yaml` cru) para o Postgres. As camadas da administradora e do condomínio vivem só no Postgres (princípio II): cada versão publicada guarda o **documento composto inteiro** daquela camada mais a versão de cima sobre a qual foi feita (o "merge-base" do git), e a diferença por trecho é calculada na hora. O núcleo puro `internal/camadas` decompõe um documento em trechos com identidade estável, compara três vias, aplica decisões, aplica obrigatório e valida o composto (princípio I). O serviço do mesmo pacote orquestra as transações. A composição para o turno é um cache em memória (`camadas.Efetivas`) trocado inteiro a cada escrita, no mesmo desenho do `admin.Cache`; o harness recebe uma função de consulta e troca `Caso`, descrição, `principal.md` e orientação do subagente pela versão efetiva.

**Tech Stack:** Go 1.27 (net/http, ent, pgx v5, goose, testcontainers, log/slog), Postgres 18, React 19.3, MUI 9.5, `@xyflow/react` 12.12, TanStack Query 5, React Router 8, Zod 4, Vitest 5 + Testing Library, Biome 2.6.

**Spec:** `docs/superpowers/specs/2026-11-04-skills-base-design.md` (aprovada pelo Felipe em 04/11/2026) · Gate da spec: `docs/superpowers/gates/2026-11-04-skills-base-spec-gate.md` · Constituição: `docs/superpowers/constitution.md` · Contexto: `docs/superpowers/specs/2026-11-03-studio-design.md`, `C:\Users\felip\Desktop\Superlogica\go-concierge\docs\superpowers\specs\2026-11-03-fundacao-design.md` · Plano anterior (estilo e comandos): `docs/superpowers/plans/2026-11-04-studio-na-fundacao.md`

## Pré-requisito: a integração Studio ↔ Fundação fecha antes

Este plano começa só quando as Tasks 8, 9 e 10 de `docs/superpowers/plans/2026-11-04-studio-na-fundacao.md` estiverem mergeadas em `feat/studio-react` (tela de 401, contrato golden com `conferirContrato` e `src/test/contrato.ts`, smoke por papel). O Step 0 da Task 1 confere. Tudo aqui se apoia no `Acesso` real, em `s.protegida`, no `admin.Cache` e no `TestTurnoNoPostgresIdasAntesDaResposta` que aquela integração deixou prontos.

## Decisões tomadas neste plano (a spec delega ao construtor)

1. **Camadas no Postgres; a base fica nos arquivos.** A Superlógica já edita a skill pelo Studio, que grava `caso.yaml` e `agent.yaml` (autosave de hoje). Esses arquivos passam a ser o **rascunho da base**. "Publicar" grava um retrato em `skill_versoes`. Administradora e condomínio não têm arquivo: rascunho e versões ficam em `skill_camadas` e `skill_versoes`. Com isso o turno deixa de ler o rascunho da base: só versão publicada vale (spec, "Rascunho e publicação").
2. **Cada versão guarda o documento composto inteiro, não a diferença.** A spec deixa a escolha ao construtor. Guardar o composto e a versão de cima (`acima_id`) é o modelo do git (retratos + merge-base): a comparação de três vias é `Comparar(antes = versão de cima usada, depois = versão de cima nova, minha = cabeça da camada)`, e "o que a camada mudou" é `Comparar(acima, cabeça)`, calculado na hora. Voltar a uma versão é copiar o retrato. Custo: JSONB repetido por versão (dezenas de KB), aceitável.
3. **Trechos.** `texto:titulo`, `texto:resumo`, `texto:quandoUsar`, `texto:frasesDeExemplo`, `texto:palavrasChave`, `orientacao`, `etapa:<id>` (um por bloco, inclusive a resposta, id `responder`), `informacao:<nome>` e `fluxo:ordem` (a sequência dos ids das etapas). Texto é **um trecho por campo**: os campos de hoje são curtos (título, resumo, quando usar, listas), e a spec deixa o construtor refinar. Parágrafo por parágrafo entra quando houver campo longo. Ferramentas (`agent.yaml recursos`) e visibilidade **não são trechos**: são da base, e a camada de baixo herda sempre (spec: "as ferramentas de uma skill são as da base").
4. **Identidade estável das etapas.** Hoje `numerar` (`internal/okf/fluxo.go:734`) dá `b1..bN` pela posição. Cada passo do `caso.yaml` ganha a chave `id`, lida e gravada pelo okf; passo sem `id` recebe o posicional na leitura e o grava no primeiro salvamento. O Studio cria ids novos aleatórios (`e-` + 8 hex), nunca `bN`, para que um bloco criado na administradora não colida com um criado depois na base. `informacao:<nome>` usa o nome do parâmetro (renomear = apagar + criar; limite conhecido).
5. **Toda administradora fica presa a uma versão da base.** A spec diz que a administradora decide primeiro. Ao publicar a base, cada administradora sem camada para a skill ganha uma, com uma versão `inicial` igual à base anterior, e recebe a atualização como qualquer outra. Condomínio sem camada segue a da administradora direto (não tem personalização a perder).
6. **O prazo vale para as duas camadas de baixo.** Vencidos `SKILLS_PRAZO_DIAS` (padrão 14), o sistema aplica numa versão `prazo` só os trechos `so_base` (a camada não mexeu) e deixa os conflitos pendentes. Na administradora é a regra da spec. No condomínio é a mesma regra, para não ter "condomínio parado numa versão velha para sempre" (anti-meta) por indecisão própria.
7. **Decisão sem escolha num trecho `so_base` é "manter".** O padrão "aceitar" é da tela (spec), não do servidor: o servidor nunca muda sozinho. Conflito sem escolha recusa o pedido (`422 CONFLITO_SEM_ESCOLHA`). "Recusar tudo" manda `manter` em todos.
8. **Toda decisão vira versão**, inclusive recusar tudo (documento igual, `origem = decisao`), para o histórico ter quem, quando e o quê, e para a camada andar o `acima_id`.
9. **Concorrência.** Rascunho com versão vista (`VERSAO_DESATUALIZADA`, como o Studio já faz). Decisão manda o número da cabeça vista; se a camada andou, `409 VERSAO_DESATUALIZADA`. Escritas em transação serializável com repetição, como `admin.Postgres.alterar`.
10. **Orientação por condomínio vira trecho da camada do condomínio.** Na primeira subida, o boot copia cada `orientacoes.por_caso` de condomínio para a camada dele (versão `importacao`, idempotente). A gaveta da skill no condomínio passa a editar o rascunho da camada e ganha "Publicar". A coluna `por_caso` e a orientação padrão (`*`) ficam como estão (nada antigo é apagado) e continuam valendo onde a camada não tem orientação.
11. **Primeira subida.** Se `skill_versoes` não tem nenhuma base, o boot publica a v1 de todas as skills do catálogo (`origem = importacao`, autor `sistema`), sem desligar nada: o comportamento de hoje continua. Depois disso, skill nova publicada chega **desligada** (habilitação padrão `*` com `Habilitado: false`).
12. **Permissões novas:** `skills.editar_etapas` e `skills.decidir_atualizacao` (gestor, herdadas por construtor e admin) e `skills.publicar_base` (construtor). Editar textos na camada continua `skills.editar_textos`. O escopo é o da camada: `escopoAdministradora(id)` ou `escopoCondominio(id)`.
13. **Roteamento por `rotas.md` não é personalizado.** O que muda por camada entra no `Caso` (que o motor de pedidos usa para escolher o caso), na projeção e no `principal.md`. O `rotas.md` do orquestrador continua o da base. Limite conhecido, anotado em `ponytail:` na Task 6.
14. **Observabilidade (observação do gate da spec):** `slog` com `operacao` ∈ {`camadas.publicar_base`, `camadas.publicar`, `camadas.decidir`, `camadas.obrigatorio`, `camadas.prazo`, `camadas.voltar`, `camadas.compor`}, mais `skill`, `camada`, `numero` e `durationMs`. INFO para eventos de negócio; ERROR para composição que falhou (a camada cai para a de cima).

## Global Constraints

- **Ordem:** a integração Studio ↔ Fundação fecha primeiro (seção acima).
- **Camadas de baixo nunca ligam ferramenta que a base não usa.** Vale em todo caminho de escrita: rascunho, publicar, decidir (inclusive mistura), voltar, prazo e obrigatório. O guarda é `camadas.Validar` (Task 4), código `FERRAMENTA_FORA_DA_BASE`.
- **Latência (regra dura):** o turno lê a skill composta pronta; um turno quente faz **0 idas ao banco a mais** antes da resposta. Guarda: `TestTurnoNoPostgresIdasAntesDaResposta` (`internal/atende/turno_bench_test.go`) com as camadas ligadas (Task 6). Nada de `camadas.Postgres` no caminho do turno.
- **Back-end Go nativo.** Front com pnpm via corepack: `npm install -g corepack@latest`, depois `corepack enable && corepack install` (lê `packageManager` do `web/studio/package.json`). Nunca `npm install -g pnpm`. Node 26 LTS (`node:26.13-trixie-slim`).
- **Testes que tocam banco com `-p 1`.** **`-race` em Docker, na `golang:1.27-trixie`, com o socket do Docker montado.**
- **Persistência:** ent por padrão; sqlc só com comentário do porquê (nenhum previsto). Migration gerada por `go run ./internal/banco/cmd/gerar-migration <nome>` depois de `go generate ./internal/banco/ent`; migration de dados escrita à mão e `atlas.sum` refeito.
- **Nada antigo é apagado:** arquivos do catálogo, `orientacoes.por_caso`, Admin, Backdoor, `mcp/` e as rotas atuais de skills ficam.
- **Front:** componentes com menos de 200 linhas, nenhuma cor hexadecimal fora de `src/theme/tokens.ts`, Biome (`pnpm run lint`) verde, TypeScript strict, sem `any`, sem `as` fora de teste, sem `!`, Zod em toda borda de API.
- **Go:** erro com contexto e sem "failed to", nada engolido, nenhum estado global mutável, toda chamada externa com `context` e prazo. Handler fino; regra pura em `internal/camadas`.
- **Commits:** conventional sem ticket (`feat: …`, `fix: …`, `test: …`, `chore: …`, `docs: …`), descrição minúscula, até 72 caracteres, `git commit` num comando Bash próprio, sem heredoc, **sem trailer de atribuição** e sem mencionar IA. Commit e push assim que a task fecha.
- **Skills de quem implementa:** `code-quality` + `go` (tasks Go), `code-quality` + `typescript` + `react` (tasks de front), `+ functional-programming` nas Tasks 2, 4 e 9 (núcleo puro), `+ api-design` nas Tasks 17 e 18, `+ frontend-design` nas Tasks 10, 12 e 13, `+ architecture-patterns` na Task 11. `code-quality` prevalece sobre precedente local em código novo.
- Nome de teste em verbo na 3ª pessoa, sem "should" (Go: `TestXxx` descritivo; Vitest: `it('acceptsAllByDefault')`).

### Comandos (rodar da raiz do worktree, em Git Bash)

```bash
DOCKER_DO_HOST="-v /var/run/docker.sock:/var/run/docker.sock -e TESTCONTAINERS_HOST_OVERRIDE=host.docker.internal -e TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock --add-host=host.docker.internal:host-gateway"
GO="MSYS_NO_PATHCONV=1 docker run --rm $DOCKER_DO_HOST -v $PWD:/src -w /src -v sl-concierge-gomod:/go/pkg/mod -v sl-concierge-gocache:/root/.cache/go-build golang:1.27-trixie"
STUDIO="MSYS_NO_PATHCONV=1 docker run --rm -e HUSKY=0 -v $PWD:/src -v /src/web/studio/node_modules -w /src/web/studio node:26.13-trixie-slim"
PNPM="npm install -g corepack@latest >/dev/null && corepack enable && corepack install && pnpm install --frozen-lockfile"
```

- Go, um teste: `eval "$GO" sh -c "'go test -p 1 -run <Regex> ./<pacote>/'"`
- Go, tudo: `eval "$GO" sh -c "'gofmt -l . && go vet ./... && go test -p 1 ./...'"` (esperado: `gofmt -l` sem saída, todos `ok`)
- Race: `eval "$GO" sh -c "'CGO_ENABLED=1 go test -race -p 1 ./internal/camadas/ ./internal/atende/ ./internal/harness/ ./cmd/okf-admin/'"` (esperado: `ok`, nenhum `WARNING: DATA RACE`)
- Vitest, um arquivo: `eval "$STUDIO" sh -c "'$PNPM && pnpm exec vitest run <arquivo>'"`
- Front, tudo: `eval "$STUDIO" sh -c "'$PNPM && pnpm run lint && pnpm run typecheck && pnpm test'"`
- Tudo de uma vez: `./build.sh --test`

## Review Focus

1. **Apagar de um lado e alterar do outro.** A base apaga a etapa que a administradora editou (ou o contrário): é conflito, a etapa nunca some sozinha, e "aceitar" apaga só com a escolha da pessoa. → Task 2 `TestCompararApagarEAlterarEhConflito`; Task 4 `TestAplicarAceitarRemocaoApaga`.
2. **Ferramenta entrando pela porta dos fundos.** Mistura que cita ferramenta nova, rascunho com `buscar_dados` de outro recurso, ou voltar a uma versão antiga cuja ferramenta a base já tirou: tudo recusado com `FERRAMENTA_FORA_DA_BASE`. → Task 4 `TestValidarRecusaFerramentaForaDaBase`; Task 11 `TestVoltarRecusaVersaoComFerramentaQueABaseTirou`; Task 16 `TestDecidirRecusaMisturaComFerramentaNova`; Task 17 `TestRascunhoComFerramentaForaDaBaseResponde422`.
3. **Duas decisões ao mesmo tempo.** Duas abas decidem a mesma atualização: só uma grava; a outra recebe `409 VERSAO_DESATUALIZADA` e nada se perde. → Task 16 `TestDecidirComCabecaVelhaResponde409`.
4. **Prazo rodando duas vezes.** Reinício no meio ou dois disparos do ticker: uma única versão `prazo` por atualização. → Task 16 `TestVencerPrazosDuasVezesGravaUmaVez`.
5. **Turno com skill que não deve rodar.** Skill sem versão publicada (criada depois da primeira subida), skill encerrada pela base depois da data, e composição que falha: as duas primeiras não são oferecidas ao morador; a terceira cai para a camada de cima com log ERROR, nunca para o rascunho. → Task 6 `TestBuildExcluiSkillSemVersaoEfetiva`; Task 9 `TestDoCondominioEscondeSkillEncerrada`, `TestMontarCaiParaACamadaDeCimaQuandoAComposicaoFalha`.

---

## Contrato da API (fonte para Go e Zod)

Todas as respostas de erro: `{codigo, mensagem}` (+ `problemas` ou `camadas` onde dito). `Documento`, `Diferenca` e `Trecho` são os tipos da Task 2 em JSON.

```text
Documento  = {textos: Textos, fluxo: {blocos: Bloco[], informacoes: Informacao[]}, ferramentas: string[], orientacao: string}
             (Textos, Bloco, Informacao = os JSON de okf de hoje)
Diferenca  = {trecho: string, tipo: "texto"|"etapa"|"informacao"|"ordem"|"orientacao", rotulo: string,
              situacao: "so_base"|"conflito", obrigatorio: boolean, antes: string|null, depois: string|null, minha: string|null}
             (conteúdo canônico: texto puro em texto/orientacao; JSON compacto de Bloco, Informacao ou string[] nos outros)
Camada     = {skill: string, tipo: "administradora"|"condominio", id: string}

GET  /api/skills/{nome}/{administradoras|condominios}/{id}             -> 200 {camada, documento, versao, temRascunho, publicada: {numero, autor, criadoEm}|null, ferramentasDaBase: string[], problemas: ProblemaFluxo[]}
PUT  /api/skills/{nome}/{administradoras|condominios}/{id}/rascunho    {versao, documento} -> 200 {versao, problemas} | 409 VERSAO_DESATUALIZADA | 422 FERRAMENTA_FORA_DA_BASE {problemas}
POST /api/skills/{nome}/{administradoras|condominios}/{id}/publicar    {resumo} -> 201 {numero} | 409 NADA_A_PUBLICAR | 422 FLUXO_INVALIDO {problemas}
GET  /api/skills/{nome}/{administradoras|condominios}/{id}/historico   -> 200 {versoes: [{numero, origem, autor, resumo, motivo, criadoEm, decisao: {para, aceitos, mantidos, misturados}|null}]}
POST /api/skills/{nome}/{administradoras|condominios}/{id}/voltar      {numero} -> 201 {numero} | 404 VERSAO_NAO_ENCONTRADA | 422 FLUXO_INVALIDO {problemas}

GET  /api/skills/{nome}/publicar        -> 200 {numero: number|null, diferencas: Diferenca[]}      (prévia: base publicada × arquivos)
POST /api/skills/{nome}/publicar        {resumo, obrigatorios: string[], motivo, encerraEm: "AAAA-MM-DD"|null} -> 201 {numero, camadas}
                                         | 409 CAMADAS_QUEBRAM {camadas: [{tipo, id, problemas}]} | 422 MOTIVO_OBRIGATORIO | 422 FLUXO_INVALIDO {problemas}

GET  /api/atualizacoes                   -> 200 {atualizacoes: [{id, camada, estado, para: {numero, resumo}, prazoEm, trechos, conflitos}], avisos: [{id, camada, motivo, trechos: string[], encerraEm: string|null, criadoEm}]}
GET  /api/atualizacoes/{id}              -> 200 {id, camada, estado, cabeca, para: {numero, resumo}, prazoEm, diferencas: Diferenca[]}
POST /api/atualizacoes/{id}/decisao      {cabeca, decisoes: [{trecho, escolha: "aceitar"|"manter"|"misturar", mistura?: string}]}
                                         -> 201 {numero, estado} | 409 VERSAO_DESATUALIZADA | 409 ATUALIZACAO_SUBSTITUIDA | 422 CONFLITO_SEM_ESCOLHA {trechos} | 422 FLUXO_INVALIDO {problemas}
POST /api/avisos/{id}/ciente             -> 204
```

`estado` ∈ `pendente | aceita | recusada | parcial | substituida`. `origem` ∈ `importacao | inicial | publicacao | decisao | obrigatorio | prazo | volta`. As rotas de atualização e aviso ficam fora de `/api/skills/` de propósito: `GET /api/skills/atualizacoes/{id}` e `GET /api/skills/{nome}/fluxo` colidiriam no `ServeMux` (panic no boot).

| Rota | Proteção |
|---|---|
| GET camada, histórico | `protegida(SkillsLer, alvoDaCamada)` |
| PUT rascunho | `protegida(SkillsLer, alvoDaCamada)` + no handler: textos mudaram ⇒ `SkillsEditarTextos`; fluxo ou orientação mudou ⇒ `SkillsEditarEtapas` (403 se faltar) |
| POST publicar camada, voltar | `protegida(SkillsEditarTextos, alvoDaCamada)` |
| GET/POST publicar base | `protegida(SkillsPublicarBase, alvoSuperlogica)` |
| GET `/api/atualizacoes` | `protegidaEmAlgum(SkillsLer)` + filtro por `Permitido(SkillsLer, escopo da camada)` |
| GET/POST atualização, ciente | `protegida(SkillsLer / SkillsDecidirAtualizacao, alvoDaAtualizacao)` |

## Mapa de arquivos

| Arquivo | Responsabilidade | Task |
|---|---|---|
| `internal/okf/fluxo.go`, `fluxo_test.go` | `id` estável nos passos | 1 |
| `internal/okf/skills.go`, `skills_test.go` | `TextosNoCaso`, `FixarIDs` | 1 |
| `internal/okf/efetiva.go` (novo) | tipo `SkillEfetiva` | 1 |
| `web/studio/src/skills/abas/fluxoDaSkill.ts`, `fluxoDaSkill.test.ts` | `novoId` aleatório | 1 |
| `internal/camadas/modelo.go`, `erros.go`, `trechos.go`, `comparar.go` (+ testes) (novos) | documento, trechos, comparação | 2 |
| `internal/acesso/acesso.go`, `acesso_test.go`; `internal/banco/migrations/20261104150000_permissoes_camadas.sql`, `atlas.sum`; `web/studio/src/acesso/eu.ts` | permissões novas | 3 |
| `internal/camadas/aplicar.go`, `validar.go` (+ testes) (novos) | decisões, obrigatório, prazo, validação | 4 |
| `internal/banco/ent/schema/skill_{camada,versao,atualizacao,aviso}.go` (novos), `internal/banco/ent/**` (gerado), migration nova + `atlas.sum` | esquema | 5 |
| `internal/harness/harness.go`, `tools.go`, `skills_efetivas_test.go` (novo); `internal/atende/servico.go`, `turno_bench_test.go` | turno lê a skill efetiva | 6 |
| `web/studio/src/camadas/contratos.ts`, `contratos.test.ts`, `camadaDaRota.ts` (novos) | contratos e hooks | 7 |
| `internal/camadas/postgres.go`, `postgres_test.go` (novos) | repositório | 8 |
| `internal/camadas/efetivas.go`, `efetivas_test.go` (novos) | composição em memória | 9 |
| `web/studio/src/app/router.tsx`; `src/camadas/{EditorDaCamadaPage,FluxoDaCamada,SeletorDeCamada,PublicarCamada}.tsx` (+ testes); `src/skills/SkillEditorPage.tsx`; `src/fluxo/painel/CamposDeFerramenta.tsx` | editor da camada | 10 |
| `internal/camadas/servico.go`, `publicar_base.go`, `servico_test.go` (novos) | serviço: camada e base | 11 |
| `web/studio/src/camadas/{ComparacaoPage,TrechoDaComparacao,MisturaDialog}.tsx`, `decisoes.ts` (+ testes); `src/app/router.tsx` | tela de comparação | 12 |
| `web/studio/src/camadas/{AvisoDeAtualizacoes,HistoricoDaCamada}.tsx` (+ testes); `src/camadas/EditorDaCamadaPage.tsx`; `src/skills/{SkillCard,SkillsPage}.tsx` | aviso e histórico | 13 |
| `web/studio/src/camadas/PublicarBaseDialog.tsx` (+ teste); `src/skills/SkillEditorPage.tsx` | publicar base | 14 |
| `web/studio/src/condominios/OrientacaoDaSkill.tsx`, `SkillDoCondominioDrawer.test.tsx` | orientação na camada | 15 |
| `internal/camadas/decidir.go`, `prazo.go`, `semente.go` (+ testes) (novos) | decidir, cascata, prazo, semente | 16 |
| `cmd/okf-admin/camadas.go`, `camadas_test.go` (novos); `testdata/contrato/camada*.json` | API da camada | 17 |
| `cmd/okf-admin/camadas_atualizacoes.go`, `camadas_atualizacoes_test.go` (novos); `testdata/contrato/{atualizacoes,atualizacao,publicar-previa}.json` | API de atualização e base | 18 |
| `cmd/okf-admin/main.go`, `config.go`, `config_test.go`, `rotas_acesso.go`, `rotas_acesso_test.go`, `README.md` | boot e rotas | 19 |
| `web/studio/src/contrato.test.ts` | contrato golden | 20 |
| `docs/superpowers/medicoes/2026-11-04-smoke-skills-base.md` (novo) | smoke e medição | 21 |

## Ondas e worktrees

Cada task roda num worktree próprio criado a partir de `feat/studio-react` **depois** que as dependências dela estão mergeadas lá. O worktree do harness nasce em `main`: o primeiro passo é `git switch -c skills-base/t<N> feat/studio-react`. Ao fechar, o controlador mergeia a branch da task em `feat/studio-react` antes da onda seguinte. Dentro de uma onda, os arquivos são disjuntos.

| Onda | Tasks (paralelas) | Depende de |
|---|---|---|
| 1 | 1, 2, 3 | pré-requisito |
| 2 | 4, 5, 6, 7 | 4←2; 5←3 (`atlas.sum`); 6←1; 7←3 |
| 3 | 8, 9, 10 | 8←2,4,5; 9←1,2; 10←1,3,7 |
| 4 | 11, 12, 13, 14, 15 | 11←4,8,9; 12←10; 13←10; 14←7; 15←7 |
| 5 | 16, 17 | 16←11; 17←11 |
| 6 | 18 | 16, 17 |
| 7 | 19, 20 | 19←6,18; 20←17,18,12,13 |
| 8 | 21 | tudo |

---

### Task 1: Identidade estável das etapas e textos no `caso.yaml` (okf + front)

**Files:**
- Modify: `internal/okf/fluxo.go` (`blocoDe`, `separarPrincipal`, `numerar`, `passoDe`)
- Modify: `internal/okf/skills.go` (extrair `TextosNoCaso` de `gravarTextosNoCaso`; `FixarIDs`)
- Create: `internal/okf/efetiva.go`
- Modify: `web/studio/src/skills/abas/fluxoDaSkill.ts`
- Test: `internal/okf/fluxo_test.go`, `internal/okf/skills_test.go`, `web/studio/src/skills/abas/fluxoDaSkill.test.ts`

**Interfaces:**
- Produces:
  - `id` no passo do `caso.yaml` (`- id: e-1a2b3c4d` + a chave do passo). A resposta (`output`) tem id fixo `responder`.
  - `numerar` mantém o `ID` que veio do arquivo ou do cliente; só dá `b<posição>` ao bloco sem id. `AplicarFluxo` grava `id` em todo passo.
  - `func TextosNoCaso(raw []byte, atual, t Textos) ([]byte, error)` (pura; `gravarTextosNoCaso` passa a chamá-la).
  - `func (c *Catalogo) FixarIDs(rel string) (bool, error)`: grava os ids posicionais nos passos que não têm; devolve se gravou. Skill sem fluxo: `false, nil`.
  - `type SkillEfetiva struct { Caso *Caso; Descricao, Principal, Orientacao string }` em `efetiva.go` (o que o turno usa no lugar dos arquivos).
  - Front: `novoId(blocos)` devolve `e-` + 8 hex de `crypto.randomUUID()`, refazendo se colidir.

- [ ] **Step 0: Conferir o pré-requisito**

Run: `git log --oneline feat/studio-react | head -20` e `ls cmd/okf-admin/testdata/contrato/ web/studio/src/test/contrato.ts docs/superpowers/medicoes/`
Expected: commits das Tasks 8, 9 e 10 da integração; fixtures golden e o smoke da integração presentes. Se faltar, parar e avisar o controlador.

- [ ] **Step 1: Escrever os testes que falham** (`fluxo_test.go`)
  - `TestFluxoMantemIDDoPassoAoReordenar`: YAML com dois passos `id: e-aaaa0001` e `id: e-bbbb0002`; `FluxoDeYAML` devolve esses ids; trocar a ordem, `AplicarFluxo`, ler de novo: ids iguais, ordem nova.
  - `TestFluxoSemIDRecebePosicionalEGravaNoSalvar`: passo sem `id` → `b1`; depois de `AplicarFluxo`, o YAML tem `id: b1`.
  - `TestFluxoAceitaIDEmPassoQueNaoEhRecurso`: `- id: e-1\n  confirm: "Confirma?"` não dá o erro "chave não cabe".
  - `TestRespostaTemIDResponder`.
  - `TestIdaEVoltaComIDsNaoMudaOArquivo`: para cada `okf/agentes/**/caso.yaml`, fixar ids, depois ler e aplicar sem mudança: bytes iguais (a regra de ida e volta do Studio passa a valer a partir do arquivo com ids).
  - `skills_test.go`: `TestTextosNoCasoTrocaSoOsCamposDeTexto` (passos e params intactos) e `TestFixarIDsGravaUmaVez` (segunda chamada devolve `false`).
  - `fluxoDaSkill.test.ts`: `it('createsRandomIdsThatNeverLookPositional')` (`/^e-[0-9a-f]{8}$/`, 50 ids distintos).

- [ ] **Step 2: Rodar e ver falhar**

Run: `eval "$GO" sh -c "'go test -p 1 -run \"TestFluxo|TestResposta|TestIdaEVolta|TestTextosNoCaso|TestFixarIDs\" ./internal/okf/'"` e o Vitest de `src/skills/abas/fluxoDaSkill.test.ts`
Expected: FAIL (ids posicionais, erro "chave id não cabe", funções inexistentes).

- [ ] **Step 3: Implementar.** `separarPrincipal` ignora `id` ao escolher a chave e o devolve à parte; `pedidos.ContratoDe` não muda (já ignora chaves desconhecidas; conferir com `go test ./internal/pedidos/`). `FixarIDs` usa a mesma trava `c.mu` e `gravarCaso`.

- [ ] **Step 4: Rodar e ver passar** (os mesmos comandos, depois Go tudo e front tudo).
Expected: PASS; `internal/pedidos` e `cmd/okf-admin` sem regressão.

- [ ] **Step 5: Commit**

```bash
git add internal/okf web/studio/src/skills/abas/fluxoDaSkill.ts web/studio/src/skills/abas/fluxoDaSkill.test.ts
```
```bash
git commit -m "feat: give flow steps a stable id that survives reordering"
```

---

### Task 2: Documento, trechos e comparação de três vias (núcleo puro)

**Files:**
- Create: `internal/camadas/modelo.go`, `internal/camadas/erros.go`, `internal/camadas/trechos.go`, `internal/camadas/comparar.go`
- Test: `internal/camadas/trechos_test.go`, `internal/camadas/comparar_test.go`

**Interfaces:**
- Consumes: `okf.Textos`, `okf.Fluxo`, `okf.Bloco`, `okf.Informacao`.
- Produces (`modelo.go`):

```go
type Tipo string // TipoBase "base", TipoAdministradora "administradora", TipoCondominio "condominio"
type Camada struct { Skill string `json:"skill"`; Tipo Tipo `json:"tipo"`; ID string `json:"id"` } // ID "" na base
type Documento struct {
	Textos      okf.Textos `json:"textos"`
	Fluxo       okf.Fluxo  `json:"fluxo"`
	Ferramentas []string   `json:"ferramentas"`
	Orientacao  string     `json:"orientacao"`
}
type TrechoID string
type TipoTrecho string // "texto" | "etapa" | "informacao" | "ordem" | "orientacao"
type Trecho struct { ID TrechoID; Tipo TipoTrecho; Rotulo, Conteudo string }
type Situacao string // SoBase "so_base", Conflito "conflito"
type Diferenca struct {
	Trecho      TrechoID   `json:"trecho"`
	Tipo        TipoTrecho `json:"tipo"`
	Rotulo      string     `json:"rotulo"`
	Situacao    Situacao   `json:"situacao"`
	Obrigatorio bool       `json:"obrigatorio"`
	Antes       *string    `json:"antes"`
	Depois      *string    `json:"depois"`
	Minha       *string    `json:"minha"`
}
type Cabeca struct { VersaoID int; Documento Documento }
type VersaoBase struct { Cabeca; CasoYAML []byte; EncerraEm time.Time }
type Retrato struct {
	Bases           map[string]VersaoBase        // skill → cabeça da base
	Administradoras map[string]map[string]Cabeca // skill → administradora → cabeça
	Condominios     map[string]map[string]Cabeca // skill → condomínio → cabeça
}
func MudouTextos(antes, depois Documento) bool
func MudouEtapas(antes, depois Documento) bool // fluxo ou orientação
```
- `erros.go`: `ErrVersaoDesatualizada`, `ErrNadaAPublicar`, `ErrVersaoNaoEncontrada`, `ErrAtualizacaoSubstituida`, `ErrCamadaNaoEncontrada`, `ErrMotivoObrigatorio`, `ErrDecisaoInvalida`, `ErrConflitoConcorrente`; `type ErrConflitoSemEscolha struct{ Trechos []TrechoID }`, `type ErrComposicaoInvalida struct{ Problemas []okf.ProblemaFluxo }`, `type ErrCamadasQuebram struct{ Camadas []CamadaQuebrada }`, `type CamadaQuebrada struct { Camada Camada; Problemas []okf.ProblemaFluxo }`.
- `func Trechos(d Documento) []Trecho` na ordem: textos, orientação, ordem, etapas na ordem do fluxo, informações. Conteúdo canônico: texto puro; JSON compacto (`json.Marshal`) de `Bloco` sem `ID`, de `Informacao` e da lista de ids. Rótulos em PT-BR: `Título`, `Resumo`, `Quando usar`, `Frases de exemplo`, `Palavras-chave`, `Orientação`, `Ordem das etapas`, `Etapa: <nome do tipo> <n>`, `Informação: <título ou nome>`. Lista vazia e campo vazio contam como trecho com conteúdo vazio (não ausente); etapa e informação inexistentes são ausentes (`nil`).
- `func Comparar(antes, depois, minha Documento) []Diferenca`: por trecho, com `A`, `D`, `M` (ausente = `nil`):

| Caso | Resultado |
|---|---|
| `A == D` | nada |
| `A == M`, `D != A` | `so_base` |
| `M == D` | nada (as duas mudaram igual) |
| senão | `conflito` (inclui apagar × alterar) |

Ordem do resultado: a de `Trechos(depois)`, depois os trechos que só existem em `minha` e `antes`. `Obrigatorio` sempre `false` aqui (a Task 4 marca).

- [ ] **Step 1: Escrever os testes que falham.** `comparar_test.go` em tabela com `documentoDeTeste()` (3 etapas: perguntar, confirmar, gravar; título "Agendar mudança"):
  - `TestCompararSoBaseMudouVemComoSoBase` (base troca o título; a pessoa não → 1 `so_base`, `Antes`/`Depois`/`Minha` preenchidos).
  - `TestCompararSoAPessoaMudouNaoApareca` (resultado vazio).
  - `TestCompararAsDuasMudaramDiferenteEhConflito`.
  - `TestCompararAsDuasMudaramIgualNaoApareca`.
  - `TestCompararApagarEAlterarEhConflito` (base apaga a etapa `e-2`, pessoa editou a pergunta dela → `conflito`, `Depois == nil`; e o espelho: pessoa apagou, base alterou).
  - `TestCompararReordenarSoMudaOTrechoDeOrdem` (mesmos blocos em outra ordem → só `fluxo:ordem`).
  - `TestCompararRenomearEtapaNaoMudaAIdentidade`.
  - `trechos_test.go`: `TestTrechosTemIDsUnicosEOrdemEstavel` e `TestMudouTextosEMudouEtapas` (tabela).

- [ ] **Step 2: Rodar e ver falhar.** Run: `eval "$GO" sh -c "'go test -run . ./internal/camadas/'"` Expected: FAIL (pacote sem implementação).

- [ ] **Step 3: Implementar** `modelo.go`, `erros.go`, `trechos.go`, `comparar.go`. Sem I/O, sem relógio.

- [ ] **Step 4: Rodar e ver passar** (o mesmo comando + `go vet ./internal/camadas/`). Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/camadas
```
```bash
git commit -m "feat: compare skill layers three ways by stable section"
```

---

### Task 3: Permissões das camadas (Go + migration de dados + front)

**Files:**
- Modify: `internal/acesso/acesso.go` (constantes, `todasAsPermissoes`, `PapeisIniciais`), `internal/acesso/acesso_test.go`
- Create: `internal/banco/migrations/20261104150000_permissoes_camadas.sql`; Modify: `internal/banco/migrations/atlas.sum`
- Modify: `web/studio/src/acesso/eu.ts` (`PERMISSOES`)

**Interfaces:**
- Produces: `acesso.SkillsEditarEtapas = "skills.editar_etapas"`, `acesso.SkillsDecidirAtualizacao = "skills.decidir_atualizacao"` (ambas em `gestor_administradora`), `acesso.SkillsPublicarBase = "skills.publicar_base"` (em `construtor_superlogica`), nessa ordem, logo depois de `SkillsExcluir`. Front: as três em `PERMISSOES`, mesma ordem.

- [ ] **Step 1: Teste que falha.** Em `acesso_test.go`, `TestGestorDecideEEditaEtapasNaCamada`: gestor de `adm-a` tem `SkillsEditarEtapas` e `SkillsDecidirAtualizacao` em `escopoCondominio("c-a1")` (condomínio de `adm-a`) e não tem em `c-b1`; não tem `SkillsPublicarBase` em lugar nenhum; construtor da Superlógica tem as três. O teste existente que confere a migration de papéis contra `PapeisIniciais()` deve falhar até a migration nova existir.
- [ ] **Step 2: Rodar e ver falhar.** Run: `eval "$GO" sh -c "'go test -p 1 ./internal/acesso/ ./internal/banco/'"` Expected: FAIL.
- [ ] **Step 3: Implementar** as constantes e a migration (`INSERT INTO papel_permissoes` com as três linhas; `-- +goose Down` apaga as três). Refazer o `atlas.sum` (`atlas migrate hash` pelo mesmo caminho que o `gerar-migration` usa, ou `go run ./internal/banco/cmd/gerar-migration` sem mudança de esquema, conforme o README).
- [ ] **Step 4: Rodar e ver passar** (Go tudo + front tudo). Expected: PASS.
- [ ] **Step 5: Commit**

```bash
git add internal/acesso internal/banco/migrations web/studio/src/acesso/eu.ts
```
```bash
git commit -m "feat: add permissions to edit steps, decide updates and publish base"
```

---

### Task 4: Aplicar decisões, obrigatório, prazo e validar o composto (núcleo puro)

**Files:**
- Create: `internal/camadas/aplicar.go`, `internal/camadas/validar.go`
- Test: `internal/camadas/aplicar_test.go`, `internal/camadas/validar_test.go`

**Interfaces:**
- Consumes: Task 2 (tudo), `okf.ValidarFluxo`.
- Produces:

```go
type Escolha string // Aceitar "aceitar", Manter "manter", Misturar "misturar"
type Decisao struct { Trecho TrechoID `json:"trecho"`; Escolha Escolha `json:"escolha"`; Mistura string `json:"mistura,omitempty"` }
func Aplicar(minha, depois Documento, difs []Diferenca, decisoes []Decisao) (Documento, error)
func AceitarPadrao(difs []Diferenca) []Decisao                         // aceitar em todo so_base; usado pelo prazo
func MarcarObrigatorios(difs []Diferenca, obrigatorios []TrechoID) []Diferenca
func AplicarObrigatorio(minha, depois Documento, obrigatorios []TrechoID) (Documento, []TrechoID) // devolve os trechos trocados onde minha != antes
func Validar(composto, base Documento) []okf.ProblemaFluxo
func TemErro(ps []okf.ProblemaFluxo) bool
```
- `Aplicar`: conflito sem decisão ⇒ `*ErrConflitoSemEscolha`; decisão para trecho fora de `difs` ou `Misturar` com `Mistura` que não faz parse no tipo do trecho ⇒ `ErrDecisaoInvalida` (com contexto). `so_base` sem decisão ⇒ manter (decisão 7). `Aceitar` com `Depois == nil` remove a etapa/informação. Ordem final das etapas: a lista do trecho `fluxo:ordem` escolhido, sem ids que não existem mais, mais as etapas que sobraram fora da lista na ordem em que aparecem em `depois` e depois em `minha`; `responder` sempre por último. `Ferramentas` do resultado = `depois.Ferramentas`.
- `Validar`: `okf.ValidarFluxo(composto.Fluxo)` mais `FERRAMENTA_FORA_DA_BASE` (erro, com `BlocoID`) para todo bloco `buscar_dados`/`gravar_no_sistema` cujo `Recurso` não está em `base.Ferramentas` nem em algum bloco da base, mais `FERRAMENTAS_DA_BASE` se `composto.Ferramentas` difere de `base.Ferramentas`, mais `VISIBILIDADE_DA_BASE` se a visibilidade difere.

- [ ] **Step 1: Testes que falham** (tabela em `aplicar_test.go`):
  - `TestAplicarAceitarTudoSemPersonalizacaoDevolveADepois`.
  - `TestAplicarManterTudoDevolveAMinha` (com conflitos decididos como `manter`).
  - `TestAplicarSoBaseSemDecisaoMantem`.
  - `TestAplicarConflitoSemDecisaoFalha` (`errors.As` em `*ErrConflitoSemEscolha` com o trecho).
  - `TestAplicarMisturaDeTexto` e `TestAplicarMisturaDeEtapa` (JSON de bloco editado entra com o mesmo id).
  - `TestAplicarMisturaInvalidaFalha`.
  - `TestAplicarAceitarRemocaoApaga` e `TestAplicarManterRemocaoGuardaAEtapa`.
  - `TestAplicarOrdemComEtapaNovaDaBaseEntraAntesDaResposta`.
  - `TestAceitarPadraoSoPegaSoBase`.
  - `TestAplicarObrigatorioTrocaMesmoPersonalizadoEDizQuais` (trecho editado pela pessoa vem em `trocados`; trecho não editado não vem).
  - Propriedade (`testing/quick` ou fuzz `FuzzAplicarManterEhIdentidade`): para documentos gerados, `Aplicar(m, d, Comparar(a, d, m), todos manter) == m`.
  - `validar_test.go`: `TestValidarRecusaFerramentaForaDaBase`, `TestValidarRecusaGravarSemConfirmarNoComposto`, `TestValidarRecusaFerramentasEVisibilidadeAlteradas`, `TestValidarAceitaEtapaQueSoEditaACom`.
- [ ] **Step 2: Rodar e ver falhar.** Run: `eval "$GO" sh -c "'go test -run \"TestAplicar|TestAceitar|TestValidar|Fuzz\" ./internal/camadas/'"` Expected: FAIL.
- [ ] **Step 3: Implementar.** Puro, sem I/O.
- [ ] **Step 4: Rodar e ver passar.** Expected: PASS.
- [ ] **Step 5: Commit**

```bash
git add internal/camadas/aplicar.go internal/camadas/aplicar_test.go internal/camadas/validar.go internal/camadas/validar_test.go
```
```bash
git commit -m "feat: apply layer decisions and validate the composed skill"
```

---

### Task 5: Esquema das camadas no Postgres (ent + migration)

**Files:**
- Create: `internal/banco/ent/schema/skill_camada.go`, `skill_versao.go`, `skill_atualizacao.go`, `skill_aviso.go`
- Modify (gerado): `internal/banco/ent/**`; Create: `internal/banco/migrations/<timestamp>_skill_camadas.sql`; Modify: `atlas.sum`
- Test: `internal/banco/banco_test.go` (o teste de migrations existente cobre)

**Interfaces:**
- Produces (tabelas; `json.RawMessage` para documentos, para o esquema não depender de `internal/camadas`):

| Tabela | Campos | Índices / FK |
|---|---|---|
| `skill_camadas` | `id`, `skill`, `tipo` (enum base/administradora/condominio), `administradora_id` (opcional, FK `administradoras` ON DELETE CASCADE), `condominio_id` (opcional, FK `condominios` ON DELETE CASCADE), `cabeca_id` (opcional), `acima_id` (opcional), `rascunho` (JSON opcional), `rascunho_versao` (string, padrão `""`) | únicos parciais: `(skill) WHERE tipo='base'`, `(skill, administradora_id) WHERE tipo='administradora'`, `(skill, condominio_id) WHERE tipo='condominio'` |
| `skill_versoes` | `id`, `camada_id` (FK), `numero`, `documento` (JSON), `caso_yaml` (bytes, opcional), `acima_id` (opcional), `atualizacao_id` (opcional), `origem` (enum da seção Contrato), `autor`, `resumo`, `motivo`, `obrigatorios` (JSON `[]string`), `encerra_em` (opcional), `criado_em` (default now) | único `(camada_id, numero)` |
| `skill_atualizacoes` | `id`, `camada_id` (FK), `de_id`, `para_id` (FK versões), `estado` (enum), `prazo_em`, `prazo_aplicado` (bool, padrão false), `decisoes` (JSON), `decidida_por`, `decidida_em` (opcional), `criado_em` | índice `(camada_id, estado)`; índice `(estado, prazo_em)` |
| `skill_avisos` | `id`, `camada_id` (FK), `versao_id` (FK), `motivo`, `trechos` (JSON `[]string` de rótulos), `encerra_em` (opcional), `ciente_por`, `ciente_em` (opcional), `criado_em` | índice `(camada_id, ciente_em)` |

`cabeca_id` e `acima_id` sem FK no ent (ciclo camada ↔ versão); a integridade é do repositório (Task 8).

- [ ] **Step 1:** escrever os 4 esquemas; `go generate ./internal/banco/ent`.
- [ ] **Step 2:** `eval "$GO" sh -c "'go run ./internal/banco/cmd/gerar-migration skill_camadas'"` Expected: um `.sql` novo em `internal/banco/migrations` e `atlas.sum` atualizado. Conferir no SQL os três índices parciais e os `ON DELETE CASCADE`.
- [ ] **Step 3: Rodar** `eval "$GO" sh -c "'go build ./... && go test -p 1 ./internal/banco/...'"` Expected: `ok` (migrations aplicam do zero e o teste de checks passa).
- [ ] **Step 4: Commit**

```bash
git add internal/banco
```
```bash
git commit -m "feat: add skill layer, version, update and notice tables"
```

---

### Task 6: O turno usa a skill efetiva (harness + atende)

**Files:**
- Modify: `internal/harness/harness.go` (`Config.Skills`, laço dos subagentes em `Build`, orientação), `internal/harness/tools.go` (`abrir_workflow` lê o `principal` efetivo)
- Create: `internal/harness/skills_efetivas_test.go`
- Modify: `internal/atende/servico.go` (`Servico.SkillsDe`, `montar`), `internal/atende/turno_bench_test.go`

**Interfaces:**
- Consumes: Task 1 `okf.SkillEfetiva`.
- Produces:
  - `harness.Config.Skills func(nome string) (okf.SkillEfetiva, bool)`: `nil` = arquivos, como hoje.
  - Em `Build`, na raiz, com `Skills != nil`: subagente sem efetiva é excluído (mesmo caminho de `excluidos` de hoje); com efetiva, o subagente usado é uma cópia rasa com `Caso` e `Descricao` trocados, e o `Agente` dele guarda `principal` (servido por `abrir_workflow` quando o arquivo pedido é `principal.md`).
  - Orientação do caso: `SkillEfetiva.Orientacao` quando não vazia, senão `cfg.Orientacoes[a.Nome]` (decisão 10).
  - `atende.Servico.SkillsDe func(administradora, condominio string, agora time.Time) func(string) (okf.SkillEfetiva, bool)`; `montar` faz `cfg.Skills = s.SkillsDe(condo.Administradora, condo.ID, time.Now())` quando não é nil.
  - Comentário `// ponytail: rotas.md do orquestrador continua o da base; personalizar o roteador pede um Roteador por condomínio` no ponto em que `Roteador` é montado (decisão 13).

- [ ] **Step 1: Testes que falham** (`skills_efetivas_test.go`, sobre o catálogo de `okf/` copiado para `t.TempDir()` como nos testes de harness atuais):
  - `TestBuildUsaOCasoEfetivo` (efetiva com `Caso.Name = "Mudança no Solar"`: o subagente montado tem esse caso e a projeção cita o nome).
  - `TestBuildExcluiSkillSemVersaoEfetiva`.
  - `TestAbrirWorkflowDevolveOPrincipalEfetivo`.
  - `TestOrientacaoEfetivaVenceADoMapeamento` e `TestSemOrientacaoEfetivaUsaADoMapeamento`.
  - `TestSkillsNilUsaOsArquivos` (regressão).
  - `turno_bench_test.go`: na tabela de `TestTurnoNoPostgresIdasAntesDaResposta`, rodar os três casos de novo com `svc.SkillsDe` ligado a uma função em memória que devolve a efetiva de `agendar-mudanca`; idas iguais (1, 1, 0).
- [ ] **Step 2: Rodar e ver falhar.** Run: `eval "$GO" sh -c "'go test -p 1 -run \"TestBuild|TestAbrirWorkflow|TestOrientacao|TestSkillsNil|TestTurnoNoPostgres\" ./internal/harness/ ./internal/atende/'"` Expected: FAIL (campos inexistentes).
- [ ] **Step 3: Implementar.** Nenhuma ida ao banco; `SkillsDe` é só consulta em memória.
- [ ] **Step 4: Rodar e ver passar** (+ race de `./internal/harness/ ./internal/atende/`). Expected: PASS, idas inalteradas.
- [ ] **Step 5: Commit**

```bash
git add internal/harness internal/atende
```
```bash
git commit -m "feat: let the turn use the composed skill instead of the files"
```

---

### Task 7: Contratos das camadas no front

**Files:**
- Create: `web/studio/src/camadas/contratos.ts`, `web/studio/src/camadas/contratos.test.ts`, `web/studio/src/camadas/camadaDaRota.ts`

**Interfaces:**
- Consumes: `apiGet`, `apiEnviar` (`@/shared/api/http`), `TextosSchema` (`@/skills/contratos`), `FluxoSchema` e `ProblemaSchema` (`@/fluxo/modelo/tipos`).
- Produces (seção Contrato da API, um schema por forma):
  - `DocumentoSchema`, `CamadaSchema`, `CamadaLidaSchema`, `RascunhoSalvoSchema`, `PublicadaSchema` (`{numero}`), `HistoricoSchema`, `DiferencaSchema`, `PreviaDaBaseSchema`, `PublicacaoBaseSchema` (`{numero, camadas}`), `AtualizacoesSchema`, `AtualizacaoSchema`, `DecididaSchema`, `CamadasQuebramSchema`.
  - `type TipoCamada = 'administradora' | 'condominio'`; `caminhoDaCamada(skill, tipo, id): string` (`/api/skills/${skill}/${tipo === 'administradora' ? 'administradoras' : 'condominios'}/${id}`).
  - Hooks: `useCamada(skill, tipo, id)`, `salvarRascunho(c, {versao, documento})`, `publicarCamada(c, {resumo})`, `useHistorico(c)`, `voltarVersao(c, numero)`, `usePreviaDaBase(skill)`, `publicarBase(skill, corpo)`, `useAtualizacoes()`, `useAtualizacao(id)`, `decidir(id, {cabeca, decisoes})`, `marcarCiente(id)`. Chaves de query: `['camada', skill, tipo, id]`, `['camada-historico', skill, tipo, id]`, `['atualizacoes']`, `['atualizacao', id]`, `['previa-base', skill]`.
  - `camadaDaRota.ts`: `export function camadaDaRota(params: {nome?: string; tipo?: string; id?: string}): Camada | null` (tipo só `administradora` ou `condominio`; o resto é `null`).

- [ ] **Step 1: Testes que falham** em `contratos.test.ts`: `parsesLayerExample`, `parsesDifferenceWithNullSides`, `rejectsUnknownSituation`, `buildsLayerPath`, `readsLayerFromRoute` e `rejectsUnknownLayerType` (com objetos escritos à mão a partir da seção Contrato; a Task 20 troca por fixture golden).
- [ ] **Step 2: Rodar e ver falhar.** Run: Vitest de `src/camadas/contratos.test.ts`. Expected: FAIL (módulo inexistente).
- [ ] **Step 3: Implementar.**
- [ ] **Step 4: Rodar e ver passar** (+ front tudo). Expected: PASS.
- [ ] **Step 5: Commit**

```bash
git add web/studio/src/camadas
```
```bash
git commit -m "feat: add studio contracts for skill layers and updates"
```

---

### Task 8: Repositório das camadas no Postgres

**Files:**
- Create: `internal/camadas/postgres.go`, `internal/camadas/postgres_test.go`

**Interfaces:**
- Consumes: Task 2 (tipos, erros), Task 4 (`Decisao`), Task 5 (ent).
- Produces:

```go
type Origem string // OrigemImportacao "importacao", OrigemInicial "inicial", OrigemPublicacao "publicacao", OrigemDecisao "decisao", OrigemObrigatorio "obrigatorio", OrigemPrazo "prazo", OrigemVolta "volta"
type EstadoAtualizacao string // Pendente, Aceita, Recusada, Parcial, Substituida
type Versao struct {
	ID, Numero, AcimaID, AtualizacaoID int
	Camada                Camada
	Documento             Documento
	CasoYAML              []byte
	Origem                Origem
	Autor, Resumo, Motivo string
	Obrigatorios          []TrechoID
	EncerraEm, CriadoEm   time.Time
}
type EstadoDaCamada struct { Camada Camada; Cabeca *Versao; AcimaID int; Rascunho *Documento; VersaoRascunho string }
type NovaVersao struct {
	Camada                Camada
	Documento             Documento
	CasoYAML              []byte
	AcimaID, AtualizacaoID int
	Origem                Origem
	Autor, Resumo, Motivo string
	Obrigatorios          []TrechoID
	EncerraEm             time.Time
	LimparRascunho        bool
}
type Atualizacao struct { ID int; Camada Camada; DeID, ParaID int; Estado EstadoAtualizacao; PrazoEm time.Time; PrazoAplicado bool; Decisoes []Decisao }
type NovoAviso struct { Camada Camada; VersaoID int; Motivo string; Trechos []string; EncerraEm time.Time }
type Aviso struct { ID int; NovoAviso; CriadoEm time.Time }

type Postgres struct{ /* *ent.Client */ }
func NovoPostgres(cli *ent.Client) *Postgres
func (p *Postgres) Repo() Repo                                              // leituras fora de transação
func (p *Postgres) EmTransacao(ctx context.Context, operacao string, f func(Repo) error) error // serializável, até 3 tentativas, ErrConflitoConcorrente no fim

type Repo struct{ /* *ent.Client */ }
func (r Repo) TemBase(ctx context.Context) (bool, error)
func (r Repo) Camada(ctx context.Context, c Camada) (EstadoDaCamada, error)       // camada sem linha: EstadoDaCamada{Camada: c}
func (r Repo) CamadasDaSkill(ctx context.Context, skill string) ([]EstadoDaCamada, error)
func (r Repo) SalvarRascunho(ctx context.Context, c Camada, versaoVista string, d Documento) (string, error) // ErrVersaoDesatualizada
func (r Repo) InserirVersao(ctx context.Context, nv NovaVersao) (Versao, error)  // cria a camada se falta; numero = último + 1; vira cabeça; acima_id da camada = nv.AcimaID
func (r Repo) Versao(ctx context.Context, id int) (Versao, error)
func (r Repo) VersaoPorNumero(ctx context.Context, c Camada, numero int) (Versao, error) // ErrVersaoNaoEncontrada
func (r Repo) Historico(ctx context.Context, c Camada, limite int) ([]Versao, error)       // mais nova primeiro
func (r Repo) CriarAtualizacao(ctx context.Context, c Camada, deID, paraID int, prazoEm time.Time) (int, error) // marca as abertas da camada como substituida
func (r Repo) Atualizacao(ctx context.Context, id int) (Atualizacao, error)
func (r Repo) AtualizacoesAbertas(ctx context.Context, limite int) ([]Atualizacao, error)   // pendente, recusada, parcial
func (r Repo) AtualizacoesVencidas(ctx context.Context, agora time.Time, limite int) ([]Atualizacao, error) // pendente|parcial, prazo_em <= agora, !prazo_aplicado
func (r Repo) RegistrarDecisao(ctx context.Context, id int, e EstadoAtualizacao, ds []Decisao, por string, em time.Time) error
func (r Repo) MarcarPrazoAplicado(ctx context.Context, id int) error
func (r Repo) CriarAviso(ctx context.Context, a NovoAviso) error
func (r Repo) AvisosAbertos(ctx context.Context, limite int) ([]Aviso, error)
func (r Repo) MarcarCiente(ctx context.Context, id int, por string, em time.Time) error
func (r Repo) LerRetrato(ctx context.Context) (Retrato, error)
```
Toda listagem tem limite (`limite` ≤ 500).

- [ ] **Step 1: Testes que falham** (Postgres real via `bancotest.Novo(t)` e `ent` aberto como em `internal/admin/postgres_test`):
  - `TestInserirVersaoNumeraECriaACamada`, `TestRascunhoComVersaoVelhaFalha`, `TestCriarAtualizacaoSubstituiAAnterior`, `TestAtualizacoesVencidasIgnoraJaAplicadas`, `TestLerRetratoTrazSoCabecas`, `TestRemoverCondominioApagaACamada` (cascade), `TestEmTransacaoDesfazTudoNoErro`.
- [ ] **Step 2: Rodar e ver falhar.** Run: `eval "$GO" sh -c "'go test -p 1 -run \"TestInserir|TestRascunho|TestCriarAtualizacao|TestAtualizacoesVencidas|TestLerRetrato|TestRemoverCondominio|TestEmTransacao\" ./internal/camadas/'"` Expected: FAIL.
- [ ] **Step 3: Implementar** com ent; `EmTransacao` copia o desenho de `admin.Postgres.alterar` (`internal/admin/postgres.go:52-138`), sem importar `admin`.
- [ ] **Step 4: Rodar e ver passar** (+ race de `./internal/camadas/`). Expected: PASS.
- [ ] **Step 5: Commit**

```bash
git add internal/camadas/postgres.go internal/camadas/postgres_test.go
```
```bash
git commit -m "feat: store skill layers, versions and updates in postgres"
```

---

### Task 9: Composição em memória para o turno

**Files:**
- Create: `internal/camadas/efetivas.go`, `internal/camadas/efetivas_test.go`

**Interfaces:**
- Consumes: Task 1 (`okf.TextosNoCaso`, `okf.AplicarFluxo`, `okf.ParseCaso`, `okf.GerarPrincipal`, `okf.SkillEfetiva`), Task 2 (`Retrato`, `Documento`).
- Produces:

```go
func Efetivar(casoYAML []byte, d Documento) (okf.SkillEfetiva, error) // pura
type Efetivas struct{ /* sync.RWMutex + mapas imutáveis depois de trocados */ }
func NovasEfetivas(log *slog.Logger) *Efetivas
func (e *Efetivas) Trocar(r Retrato)       // compõe cada versão distinta uma vez (por VersaoID) e troca tudo de uma vez
func (e *Efetivas) DoCondominio(administradora, condominio string, agora time.Time) func(string) (okf.SkillEfetiva, bool)
```
- `Efetivar`: skill sem fluxo (`casoYAML == nil`) devolve `Caso: nil`, `Descricao: d.Textos.QuandoUsar`, `Principal: ""`. Com fluxo: `TextosNoCaso` + `AplicarFluxo` + `ParseCaso`; `Principal = GerarPrincipal(d.Textos.Titulo, d.Textos.QuandoUsar, d.Fluxo.Blocos)`; `Orientacao = d.Orientacao`.
- `DoCondominio`: antes de qualquer `Trocar`, devolve `nil` (turno usa os arquivos). Depois, a função procura condomínio → administradora → base; skill com `EncerraEm` não zero e `!agora.Before(EncerraEm)` some. Sem I/O, sem alocação além do fechamento.
- `Trocar`: composição que falha numa camada loga `ERROR` (`operacao=camadas.compor`, `skill`, `camada`, `erro`) e usa a efetiva da camada de cima.

- [ ] **Step 1: Testes que falham:** `TestEfetivarAplicaTextosEFluxo`, `TestEfetivarSkillSemFluxo`, `TestDoCondominioPrefereCondominioDepoisAdministradoraDepoisBase`, `TestDoCondominioEscondeSkillEncerrada`, `TestDoCondominioAntesDeTrocarDevolveNil`, `TestMontarCaiParaACamadaDeCimaQuandoAComposicaoFalha` (documento com bloco de tipo inválido para `AplicarFluxo`), `TestTrocarCompoeCadaVersaoUmaVez` (contador no `Efetivar` injetado por campo não exportado), `TestDoCondominioEmParaleloComTrocar` (rodar com `-race`).
- [ ] **Step 2: Rodar e ver falhar.** Run: `eval "$GO" sh -c "'go test -run \"TestEfetivar|TestDoCondominio|TestMontar|TestTrocar\" ./internal/camadas/'"` Expected: FAIL.
- [ ] **Step 3: Implementar.**
- [ ] **Step 4: Rodar e ver passar** (+ race). Expected: PASS, sem `DATA RACE`.
- [ ] **Step 5: Commit**

```bash
git add internal/camadas/efetivas.go internal/camadas/efetivas_test.go
```
```bash
git commit -m "feat: precompose effective skills per condominio in memory"
```

---

### Task 10: Editor da camada (front)

**Files:**
- Modify: `web/studio/src/app/router.tsx` (rota `skills/:nome/:tipo/:id`, `tipo` ∈ `administradoras|condominios`)
- Create: `web/studio/src/camadas/EditorDaCamadaPage.tsx`, `FluxoDaCamada.tsx`, `SeletorDeCamada.tsx`, `PublicarCamada.tsx`
- Modify: `web/studio/src/skills/SkillEditorPage.tsx` (o `SeletorDeCamada` no cabeçalho), `web/studio/src/fluxo/painel/CamposDeFerramenta.tsx` (prop `permitidas?: readonly string[]`)
- Test: `web/studio/src/camadas/EditorDaCamadaPage.test.tsx`, `SeletorDeCamada.test.tsx`

**Interfaces:**
- Consumes: Task 7 (`useCamada`, `salvarRascunho`, `publicarCamada`, `camadaDaRota`), `QuandoUsarAba`, `FluxoCanvas`, `PainelLateralDoFluxo`, funções de `@/fluxo/modelo/edicao`, `useAutosave`, `SaveIndicator`, `useEu`, `pode`.
- Produces:
  - `EditorDaCamadaPage`: título "Personalizar · <título da skill>" e o nome da camada ("Sua administradora" ou o nome do condomínio); abas **Quando usar**, **Fluxo**, **Orientação** (só condomínio) e **Histórico** (a Task 13 preenche). Autosave do documento inteiro em `salvarRascunho` (1s, uma gravação por vez, como o editor de hoje). Botão **Publicar** abre `PublicarCamada` (resumo opcional) e fica desabilitado enquanto o autosave está `salvando` ou com erro. Textos editáveis só com `skills.editar_textos`; fluxo e orientação só com `skills.editar_etapas`.
  - `FluxoDaCamada({ documento, onChange, editavel, ferramentasDaBase, problemas })`: o canvas de hoje, sem a aba Ferramentas; `CamposDeFerramenta` recebe `permitidas={ferramentasDaBase}` e só oferece essas.
  - `SeletorDeCamada({ skill })`: menu "Versão: Superlógica | Sua administradora | <condomínio>"; navega para a rota da camada. Lista as camadas que o `eu` pode ler (administradora do `eu`; condomínios via o contrato de condomínios já existente).
  - Erro `FERRAMENTA_FORA_DA_BASE` mostra os problemas no bloco (mesmo realce de hoje).

- [ ] **Step 1: Testes que falham:**
  - `EditorDaCamadaPage.test.tsx`: `autosavesDraftOfTheLayer`, `disablesPublishWhileSaving`, `publishesWithSummary`, `offersOnlyBaseToolsInFlow`, `hidesOrientationTabForAdministradora`, `showsReadOnlyFlowWithoutEditStepsPermission`, `showsToolErrorOnTheBlock`.
  - `SeletorDeCamada.test.tsx`: `listsOwnAdministradoraAndCondominios`, `navigatesToLayerRoute`.
- [ ] **Step 2: Rodar e ver falhar.** Run: Vitest dos dois arquivos. Expected: FAIL.
- [ ] **Step 3: Implementar.** Cada componente < 200 linhas; cores só de `tokens`.
- [ ] **Step 4: Rodar e ver passar** (+ front tudo). Expected: PASS.
- [ ] **Step 5: Commit**

```bash
git add web/studio/src/app/router.tsx web/studio/src/camadas web/studio/src/skills/SkillEditorPage.tsx web/studio/src/fluxo/painel/CamposDeFerramenta.tsx
```
```bash
git commit -m "feat: edit and publish a skill layer in the studio"
```

---

### Task 11: Serviço: camada e publicação da base

**Files:**
- Create: `internal/camadas/servico.go`, `internal/camadas/publicar_base.go`, `internal/camadas/servico_test.go`

**Interfaces:**
- Consumes: Tasks 2, 4, 8, 9; `okf.Catalogo` (`LerTextos`, `LerFluxo`, `LerArquivo`, `Load`, `ResolveAgente`, `FixarIDs`).
- Produces:

```go
type Config struct {
	Postgres          *Postgres
	Catalogo          *okf.Catalogo
	Agente            string        // rel do concierge; skill = Agente + "/agentes/" + nome
	Efetivas          *Efetivas
	Prazo             time.Duration // padrão 14 dias
	Agora             func() time.Time
	Log               *slog.Logger
	Administradoras   func(ctx context.Context) ([]string, error)
	Condominios       func(ctx context.Context, administradora string) ([]string, error)
	DesligarSkillNova func(ctx context.Context, skill string) error
}
type Servico struct{ /* Config */ }
func NovoServico(c Config) *Servico
func (s *Servico) Recompor(ctx context.Context) error // LerRetrato + Efetivas.Trocar; chamado depois de toda escrita confirmada
type CamadaLida struct { Camada Camada; Documento Documento; Versao string; TemRascunho bool; Publicada *Versao; FerramentasDaBase []string; Problemas []okf.ProblemaFluxo }
func (s *Servico) LerCamada(ctx context.Context, c Camada) (CamadaLida, error)
func (s *Servico) SalvarRascunho(ctx context.Context, c Camada, versaoVista string, d Documento) (string, []okf.ProblemaFluxo, error)
func (s *Servico) PublicarCamada(ctx context.Context, c Camada, autor, resumo string) (Versao, error)
func (s *Servico) Historico(ctx context.Context, c Camada) ([]Versao, error)
func (s *Servico) Voltar(ctx context.Context, c Camada, numero int, autor string) (Versao, error)
type PublicacaoBase struct { Skill, Autor, Resumo, Motivo string; Obrigatorios []TrechoID; EncerraEm time.Time }
func (s *Servico) PreviaDaBase(ctx context.Context, skill string) (numero int, difs []Diferenca, err error)
func (s *Servico) PublicarBase(ctx context.Context, p PublicacaoBase) (Versao, int, error)
func (s *Servico) cascatear(ctx context.Context, r Repo, adm Versao) error // usado também pela Task 16
```
- `LerCamada`: documento = rascunho, senão cabeça, senão o documento de cima (cabeça da administradora para condomínio; cabeça da base). Sem base publicada: `ErrCamadaNaoEncontrada`.
- `SalvarRascunho`: `Validar(d, base)`; erro com código `FERRAMENTA_FORA_DA_BASE`, `FERRAMENTAS_DA_BASE` ou `VISIBILIDADE_DA_BASE` ⇒ `*ErrComposicaoInvalida` (nada gravado); outros problemas voltam como aviso e o rascunho grava.
- `PublicarCamada`: sem rascunho ⇒ `ErrNadaAPublicar`; `TemErro(Validar)` ⇒ `*ErrComposicaoInvalida`; versão `publicacao` com `AcimaID` da camada e `LimparRascunho`; camada da administradora ⇒ `cascatear` (atualização para cada camada de condomínio dela que existe, `de = acima` do condomínio, `para = versão nova`, `prazoEm = agora + Prazo`).
- `Voltar`: copia o documento da versão `numero` numa versão `volta`, depois de `Validar` contra a base atual.
- `PreviaDaBase`: `Comparar(cabeça da base, documento dos arquivos, cabeça da base)`.
- `PublicarBase`: antes da transação, `FixarIDs` e a leitura do documento dos arquivos (Textos, Fluxo, `Recursos` do agente, `caso.yaml` cru), sem I/O de arquivo dentro dela; `Motivo` vazio com obrigatórios ou `EncerraEm` ⇒ `ErrMotivoObrigatorio`; `TemErro(okf.ValidarFluxo)` ⇒ `*ErrComposicaoInvalida`. **Prévia:** para cada camada com cabeça, `AplicarObrigatorio(cabeça, nova, obrigatorios)` e `Validar`; alguma com erro ⇒ `*ErrCamadasQuebram` e nada é gravado. **Gravação:** versão da base; para cada administradora de `Administradoras(ctx)` sem camada e com base anterior, versão `inicial` = base anterior; atualização para cada camada de administradora; versão `obrigatorio` + `NovoAviso` (trechos trocados por rótulo, motivo, `EncerraEm`) em cada camada (administradora e condomínio) onde houve troca ou onde há `EncerraEm`. Primeira versão da skill depois da primeira subida (`TemBase` já era `true`) ⇒ `DesligarSkillNova`. Depois do commit: `Recompor` e log INFO `camadas.publicar_base` com `skill`, `numero`, `camadas` e `durationMs`.

- [ ] **Step 1: Testes que falham** (Postgres real + catálogo de `okf/` copiado; `Administradoras`/`Condominios` por fakes de função):
  - `TestPublicarBasePrendeAdministradoraSemCamadaNaVersaoAnterior`.
  - `TestPublicarBaseCriaAtualizacaoParaCadaAdministradora`.
  - `TestPublicarBaseComObrigatorioTrocaEmTodasAsCamadasEAvisaQuemMexeu`.
  - `TestPublicarBaseObrigatorioQueQuebraUmaCamadaNaoGravaNada` (`errors.As` em `*ErrCamadasQuebram` com a camada).
  - `TestPublicarBaseSemMotivoComObrigatorioFalha`.
  - `TestPublicarBaseSkillNovaChegaDesligada` e `TestPrimeiraSubidaNaoDesliga`.
  - `TestRascunhoComFerramentaForaDaBaseNaoGrava`.
  - `TestPublicarCamadaDaAdministradoraAvisaOsCondominiosComCamada`.
  - `TestPublicarSemRascunhoFalha`.
  - `TestVoltarCriaVersaoNovaComODocumentoAntigo` e `TestVoltarRecusaVersaoComFerramentaQueABaseTirou`.
  - `TestRecomporTrocaAsEfetivasDepoisDePublicar`.
- [ ] **Step 2: Rodar e ver falhar.** Run: `eval "$GO" sh -c "'go test -p 1 -run \"TestPublicar|TestPrimeiraSubida|TestRascunho|TestVoltar|TestRecompor\" ./internal/camadas/'"` Expected: FAIL.
- [ ] **Step 3: Implementar.** Decisão pura nas funções das Tasks 2 e 4; o serviço só lê, chama e grava.
- [ ] **Step 4: Rodar e ver passar** (+ race). Expected: PASS.
- [ ] **Step 5: Commit**

```bash
git add internal/camadas/servico.go internal/camadas/publicar_base.go internal/camadas/servico_test.go
```
```bash
git commit -m "feat: publish base and layer versions with mandatory sections"
```

---

### Task 12: Tela de comparação (front)

**Files:**
- Create: `web/studio/src/camadas/ComparacaoPage.tsx`, `TrechoDaComparacao.tsx`, `MisturaDialog.tsx`, `decisoes.ts`
- Modify: `web/studio/src/app/router.tsx` (rota `atualizacoes/:id`)
- Test: `web/studio/src/camadas/decisoes.test.ts`, `ComparacaoPage.test.tsx`

**Interfaces:**
- Consumes: Task 7 (`useAtualizacao`, `decidir`, `DiferencaSchema`), `BlocoPainel` (`@/fluxo/painel/BlocoPainel`) para misturar uma etapa.
- Produces:
  - `decisoes.ts` (puro): `type Escolhas = Record<string, { escolha: 'aceitar' | 'manter' | 'misturar'; mistura?: string }>`; `escolhasIniciais(difs)` (`so_base` → `aceitar`; conflito → nada); `aceitarTudo(difs)`; `recusarTudo(difs)` (`manter` em todos); `pendentes(difs, escolhas): string[]` (conflitos sem escolha); `paraEnvio(escolhas)`.
  - `ComparacaoPage`: cabeçalho com skill, camada, "Nova versão N: <resumo>" e "Chega sozinha em <data>" quando há `prazoEm`; uma linha por trecho com três colunas (**Como era**, **Como ficou**, **Sua versão**) e as escolhas; trecho obrigatório vem marcado e travado; ações **Aceitar tudo**, **Recusar tudo**, **Salvar decisão** (desabilitado com conflito pendente, dizendo quantos faltam). `422 FLUXO_INVALIDO` mostra os problemas no trecho; `409 VERSAO_DESATUALIZADA` mostra "Esta versão mudou. Recarregue para decidir de novo." Sucesso: snackbar "Atualização decidida" e volta para o editor da camada.
  - `TrechoDaComparacao`: texto lado a lado; etapa como cartão com o tipo e o texto principal do bloco; ordem como lista numerada de rótulos.
  - `MisturaDialog`: texto ⇒ campo de texto começando pela versão da pessoa; etapa ⇒ `BlocoPainel` com o bloco da pessoa; devolve o conteúdo canônico.

- [ ] **Step 1: Testes que falham:**
  - `decisoes.test.ts`: `acceptsBaseOnlyByDefault`, `leavesConflictsUndecided`, `rejectsAllWithKeep`, `countsPendingConflicts`.
  - `ComparacaoPage.test.tsx`: `showsThreeColumnsPerSection`, `blocksSaveWhileConflictPending`, `sendsMixForConflict`, `locksMandatorySection`, `showsServerProblemsOnSection`, `showsReloadOnStaleVersion`, `decidesTenSectionsWithAcceptAllInOneClick` (10 trechos `so_base`: um clique em Salvar decisão manda 10 `aceitar`; meta de 2 minutos).
- [ ] **Step 2: Rodar e ver falhar.** Expected: FAIL.
- [ ] **Step 3: Implementar.**
- [ ] **Step 4: Rodar e ver passar** (+ front tudo). Expected: PASS.
- [ ] **Step 5: Commit**

```bash
git add web/studio/src/camadas/ComparacaoPage.tsx web/studio/src/camadas/ComparacaoPage.test.tsx web/studio/src/camadas/TrechoDaComparacao.tsx web/studio/src/camadas/MisturaDialog.tsx web/studio/src/camadas/decisoes.ts web/studio/src/camadas/decisoes.test.ts web/studio/src/app/router.tsx
```
```bash
git commit -m "feat: compare and decide skill updates section by section"
```

---

### Task 13: Aviso de atualização e histórico (front)

**Files:**
- Create: `web/studio/src/camadas/AvisoDeAtualizacoes.tsx`, `HistoricoDaCamada.tsx`
- Modify: `web/studio/src/camadas/EditorDaCamadaPage.tsx` (aba Histórico e aviso no topo), `web/studio/src/skills/SkillCard.tsx` (selo), `web/studio/src/skills/SkillsPage.tsx` (passa as atualizações ao card)
- Test: `web/studio/src/camadas/AvisoDeAtualizacoes.test.tsx`, `HistoricoDaCamada.test.tsx`, `web/studio/src/skills/SkillsPage.test.tsx` (caso novo)

**Interfaces:**
- Consumes: Task 7 (`useAtualizacoes`, `marcarCiente`, `useHistorico`, `voltarVersao`).
- Produces:
  - `AvisoDeAtualizacoes({ camada })`: faixa "Essa skill tem atualizações" com o resumo e o botão **Revisar** (rota `atualizacoes/:id`); aviso de obrigatório: "A Superlógica trocou <rótulos>: <motivo>" (+ "Esta skill para de funcionar em <data>" quando `encerraEm`) com **Entendi** (`marcarCiente`).
  - `SkillCard`: prop `temAtualizacao: boolean`; selo "Atualização" quando há alguma aberta para a skill nas camadas do `eu`.
  - `HistoricoDaCamada({ camada })`: lista (número, data, autor, origem em PT-BR, resumo, motivo, "aceitou X, manteve Y, misturou Z"); **Voltar para esta versão** com confirmação "Voltar para a versão N? Uma versão nova é criada com o conteúdo dela." (exige `skills.editar_textos`).

- [ ] **Step 1: Testes que falham:** `showsUpdateBannerWithReviewLink`, `showsMandatoryNoticeWithReason`, `dismissesNoticeOnAcknowledge`, `showsEndDateWhenSkillIsRetired`, `listsVersionsWithDecisionCounts`, `restoresVersionAfterConfirm`, `hidesRestoreWithoutPermission`, `SkillsPage`: `marksCardWithPendingUpdate`.
- [ ] **Step 2: Rodar e ver falhar.** Expected: FAIL.
- [ ] **Step 3: Implementar.**
- [ ] **Step 4: Rodar e ver passar** (+ front tudo). Expected: PASS.
- [ ] **Step 5: Commit**

```bash
git add web/studio/src/camadas/AvisoDeAtualizacoes.tsx web/studio/src/camadas/AvisoDeAtualizacoes.test.tsx web/studio/src/camadas/HistoricoDaCamada.tsx web/studio/src/camadas/HistoricoDaCamada.test.tsx web/studio/src/camadas/EditorDaCamadaPage.tsx web/studio/src/skills/SkillCard.tsx web/studio/src/skills/SkillsPage.tsx web/studio/src/skills/SkillsPage.test.tsx
```
```bash
git commit -m "feat: show skill update notices and layer history"
```

---

### Task 14: Publicar a base (front)

**Files:**
- Create: `web/studio/src/camadas/PublicarBaseDialog.tsx`, `PublicarBaseDialog.test.tsx`
- Modify: `web/studio/src/skills/SkillEditorPage.tsx` (botão **Publicar versão** no cabeçalho, só com `skills.publicar_base`)

**Interfaces:**
- Consumes: Task 7 (`usePreviaDaBase`, `publicarBase`, `CamadasQuebramSchema`).
- Produces: `PublicarBaseDialog({ skill, aberto, onFechar })`: lista dos trechos que mudaram desde a última versão (prévia), cada um com a caixa **Obrigatória**; campo **Resumo**; campo **Motivo** (obrigatório se algum trecho é obrigatório ou se há data de encerramento); seção recolhida **Encerrar esta skill** com data (`<input type="date">`). Sem mudança e com versão anterior: "Nada mudou desde a versão N". `409 CAMADAS_QUEBRAM` lista "Administradora X / Condomínio Y: <problema>" e não fecha. Sucesso: snackbar "Versão N publicada para M camadas".

- [ ] **Step 1: Testes que falham:** `listsChangedSectionsFromPreview`, `requiresReasonWhenMandatory`, `requiresReasonWhenRetiring`, `showsBrokenLayersOnConflict`, `publishesAndReportsLayers`, `saysNothingChanged`, e em `SkillEditorPage.test.tsx` (já existente): `hidesPublishWithoutPermission`.
- [ ] **Step 2: Rodar e ver falhar.** Expected: FAIL.
- [ ] **Step 3: Implementar.**
- [ ] **Step 4: Rodar e ver passar** (+ front tudo). Expected: PASS.
- [ ] **Step 5: Commit**

```bash
git add web/studio/src/camadas/PublicarBaseDialog.tsx web/studio/src/camadas/PublicarBaseDialog.test.tsx web/studio/src/skills/SkillEditorPage.tsx web/studio/src/skills/SkillEditorPage.test.tsx
```
```bash
git commit -m "feat: publish a base skill version with mandatory sections"
```

---

### Task 15: Orientação do condomínio na camada (front)

**Files:**
- Modify: `web/studio/src/condominios/OrientacaoDaSkill.tsx`
- Test: `web/studio/src/condominios/SkillDoCondominioDrawer.test.tsx`

**Interfaces:**
- Consumes: Task 7 (`useCamada`, `salvarRascunho`, `publicarCamada`).
- Produces: `OrientacaoDaSkill` lê e grava `documento.orientacao` da camada do condomínio para a skill do `caso` (nome da pasta) (autosave no rascunho) e mostra **Publicar** quando `temRascunho`; o resto do documento vai intacto. Props não mudam (o `estado` deixa de ser lido para a orientação).

- [ ] **Step 1: Testes que falham:** `readsOrientationFromCondominioLayer`, `autosavesOrientationToDraft`, `publishesOrientation`, `keepsOtherSectionsWhenSavingOrientation`.
- [ ] **Step 2: Rodar e ver falhar.** Expected: FAIL.
- [ ] **Step 3: Implementar.**
- [ ] **Step 4: Rodar e ver passar** (+ front tudo). Expected: PASS.
- [ ] **Step 5: Commit**

```bash
git add web/studio/src/condominios/OrientacaoDaSkill.tsx web/studio/src/condominios/SkillDoCondominioDrawer.test.tsx
```
```bash
git commit -m "feat: keep per condominio guidance in the condominio layer"
```

---

### Task 16: Serviço: decidir, cascata, prazo e semente

**Files:**
- Create: `internal/camadas/decidir.go`, `internal/camadas/prazo.go`, `internal/camadas/semente.go`
- Test: `internal/camadas/decidir_test.go`, `internal/camadas/prazo_test.go`, `internal/camadas/semente_test.go`

**Interfaces:**
- Consumes: Task 11 (`Servico`, `cascatear`, `Recompor`), Tasks 4 e 8.
- Produces:

```go
type ResumoDeAtualizacao struct { ID int; Camada Camada; Estado EstadoAtualizacao; Para Versao; PrazoEm time.Time; Trechos, Conflitos int }
type AtualizacaoLida struct { ID int; Camada Camada; Estado EstadoAtualizacao; Cabeca int; Para Versao; PrazoEm time.Time; Diferencas []Diferenca }
func (s *Servico) Atualizacoes(ctx context.Context) ([]ResumoDeAtualizacao, []Aviso, error) // o handler filtra por escopo
func (s *Servico) Atualizacao(ctx context.Context, id int) (AtualizacaoLida, error)
func (s *Servico) CamadaDaAtualizacao(ctx context.Context, id int) (Camada, error)
func (s *Servico) CamadaDoAviso(ctx context.Context, id int) (Camada, error)
func (s *Servico) Decidir(ctx context.Context, id, cabecaVista int, ds []Decisao, autor string) (Versao, EstadoAtualizacao, error)
func (s *Servico) MarcarCiente(ctx context.Context, id int, autor string) error
func (s *Servico) VencerPrazos(ctx context.Context) (int, error)
func (s *Servico) RodarPrazos(ctx context.Context, intervalo time.Duration) // laço até ctx acabar; cada rodada com prazo de 1 min; erro vira log ERROR
func (s *Servico) SemearBase(ctx context.Context) (int, error)
func (s *Servico) ImportarOrientacoes(ctx context.Context, porCondominio map[string]map[string]string) (int, error) // condomínio → skill → texto
```
- `Atualizacao`: `Diferencas = MarcarObrigatorios(Comparar(de, para, cabeça), obrigatórios de para)`.
- `Decidir` (transação): estado `substituida` ⇒ `ErrAtualizacaoSubstituida`; cabeça atual ≠ `cabecaVista` ⇒ `ErrVersaoDesatualizada`; `Aplicar`; `TemErro(Validar(composto, base atual))` ⇒ `*ErrComposicaoInvalida`; versão `decisao` com `AcimaID = para`, `AtualizacaoID = id`; estado: todos `aceitar` ⇒ `aceita`, nenhum ⇒ `recusada`, misto ⇒ `parcial`; `RegistrarDecisao`; camada da administradora ⇒ `cascatear`. Log INFO `camadas.decidir` com contagens.
- `VencerPrazos`: para cada `AtualizacoesVencidas(agora, 200)`, numa transação por atualização: `AceitarPadrao` + `Aplicar` + `Validar`; inválido ⇒ log WARN e `MarcarPrazoAplicado` sem versão (a pendência fica, para a pessoa decidir); válido ⇒ versão `prazo` (autor `sistema`, `AcimaID` inalterado, porque os conflitos seguem abertos) + `MarcarPrazoAplicado` + `cascatear` se administradora. Devolve quantas aplicou.
- `SemearBase`: só se `!TemBase`; publica a v1 de cada skill do catálogo (`importacao`, autor `sistema`, sem `DesligarSkillNova`).
- `ImportarOrientacoes`: para cada par sem camada de condomínio, versão `importacao` = documento de cima + `Orientacao`.
- Todas chamam `Recompor` depois do commit.

- [ ] **Step 1: Testes que falham:**
  - `decidir_test.go`: `TestDecidirAceitarTudoAndaACamadaEAvisaOsCondominios`, `TestDecidirRecusarTudoGravaVersaoEMantemODocumento`, `TestRecusadaPodeSerRevistaAteAProximaVersao`, `TestDecidirComCabecaVelhaResponde409` (`ErrVersaoDesatualizada`), `TestDecidirSubstituidaFalha`, `TestDecidirRecusaMisturaComFerramentaNova`, `TestDecidirConflitoSemEscolhaFalha`, `TestCondominioSoVeOQueAAdministradoraAceitou` (a administradora recusa um trecho; a atualização do condomínio não traz esse trecho).
  - `prazo_test.go` (relógio por `Config.Agora`): `TestVencerPrazosAplicaSoOQueAAdministradoraNaoMexeu`, `TestVencerPrazosDeixaConflitoPendente`, `TestVencerPrazosDuasVezesGravaUmaVez`, `TestVencerPrazosCascateiaParaOsCondominios`, `TestAntesDoPrazoNadaChega`.
  - `semente_test.go`: `TestSemearBasePublicaTodasAsSkillsUmaVez`, `TestImportarOrientacoesEhIdempotente`.
- [ ] **Step 2: Rodar e ver falhar.** Run: `eval "$GO" sh -c "'go test -p 1 -run \"TestDecidir|TestRecusada|TestCondominioSo|TestVencerPrazos|TestAntesDoPrazo|TestSemearBase|TestImportarOrientacoes\" ./internal/camadas/'"` Expected: FAIL.
- [ ] **Step 3: Implementar.**
- [ ] **Step 4: Rodar e ver passar** (+ race). Expected: PASS.
- [ ] **Step 5: Commit**

```bash
git add internal/camadas/decidir.go internal/camadas/decidir_test.go internal/camadas/prazo.go internal/camadas/prazo_test.go internal/camadas/semente.go internal/camadas/semente_test.go
```
```bash
git commit -m "feat: decide updates with cascade, deadline and first seed"
```

---

### Task 17: API da camada (Go)

**Files:**
- Create: `cmd/okf-admin/camadas.go`, `cmd/okf-admin/camadas_test.go`
- Create: `cmd/okf-admin/testdata/contrato/camada.json`, `camada-historico.json`

**Interfaces:**
- Consumes: Task 11; `s.protegida`, `permitidoAqui`, `escopoAdministradora`, `escopoCondominio`, `lerCorpo`, `comPrazo`, `writeJSON`, `erroAPI`, `conferirContrato`, `acessoDe`.
- Produces:
  - `type apiCamadas struct { svc *camadas.Servico; log *log.Logger }`.
  - `func (s *servidor) rotasCamadas(mux *http.ServeMux, api apiCamadas)`: as 5 rotas × 2 tipos da seção Contrato, protegidas como na tabela. Não é chamada por `rotear` ainda (Task 19).
  - `func alvoDaCamada(r *http.Request) (acesso.Escopo, error)` (lê `{id}` e o tipo do padrão).
  - `func (api apiCamadas) responderErro(w http.ResponseWriter, err error)`: `ErrVersaoDesatualizada` 409, `ErrNadaAPublicar` 409, `ErrVersaoNaoEncontrada` 404, `ErrCamadaNaoEncontrada` 404, `ErrAtualizacaoSubstituida` 409, `ErrMotivoObrigatorio` 422, `*ErrComposicaoInvalida` 422 (`FERRAMENTA_FORA_DA_BASE` se algum problema tem esse código, senão `FLUXO_INVALIDO`), `*ErrConflitoSemEscolha` 422, `*ErrCamadasQuebram` 409, `ErrConflitoConcorrente` 503; o resto loga e responde 500 `ERRO_INTERNO`. Mensagens em PT-BR, segunda pessoa.
  - Autor das versões = `Acesso.UsuarioID` do contexto.

- [ ] **Step 1: Testes que falham** (`novoAmbienteCamadas(t)`: `novoAmbienteRotas(t)` + Postgres de `bancotest` + `camadas.NovoServico` com a base semeada; um mux só com `s.rotasCamadas` atrás de `s.autenticado`):
  - `TestCamadaGestorLeEditaEPublicaNaPropriaAdministradora`.
  - `TestCamadaGestorDeOutraAdministradoraRecebe403`.
  - `TestRascunhoComFerramentaForaDaBaseResponde422`.
  - `TestRascunhoMudandoFluxoSemEditarEtapasResponde403` (papel de teste só com `skills.editar_textos`).
  - `TestPublicarSemRascunhoResponde409`.
  - `TestHistoricoEVoltar`.
  - `TestContratoDaCamada` (`conferirContrato(t, "camada", …)` e `"camada-historico"`).
- [ ] **Step 2: Rodar e ver falhar.** Run: `eval "$GO" sh -c "'go test -p 1 -run \"TestCamada|TestRascunho|TestPublicarSem|TestHistorico|TestContratoDaCamada\" ./cmd/okf-admin/'"` Expected: FAIL.
- [ ] **Step 3: Implementar**; gerar as fixtures com `-args -atualizar-contrato` e conferir o JSON à mão contra a seção Contrato.
- [ ] **Step 4: Rodar e ver passar** (+ Go tudo). Expected: PASS.
- [ ] **Step 5: Commit**

```bash
git add cmd/okf-admin/camadas.go cmd/okf-admin/camadas_test.go cmd/okf-admin/testdata/contrato
```
```bash
git commit -m "feat: add layer draft, publish, history and restore api"
```

---

### Task 18: API de atualizações e da publicação da base (Go)

**Files:**
- Create: `cmd/okf-admin/camadas_atualizacoes.go`, `cmd/okf-admin/camadas_atualizacoes_test.go`
- Create: `cmd/okf-admin/testdata/contrato/atualizacoes.json`, `atualizacao.json`, `publicar-previa.json`

**Interfaces:**
- Consumes: Task 16, Task 17 (`apiCamadas`, `responderErro`, `novoAmbienteCamadas`).
- Produces: `func (s *servidor) rotasAtualizacoes(mux *http.ServeMux, api apiCamadas)` com `GET/POST /api/skills/{nome}/publicar`, `GET /api/atualizacoes`, `GET /api/atualizacoes/{id}`, `POST /api/atualizacoes/{id}/decisao`, `POST /api/avisos/{id}/ciente`; `func (s *servidor) alvoDaAtualizacao(api apiCamadas) alvoDe` e `alvoDoAviso` (consultam `CamadaDaAtualizacao`/`CamadaDoAviso`; id inexistente ⇒ 404). `GET /api/atualizacoes` filtra com `acesso.Permitido(a, SkillsLer, escopo da camada)`.

- [ ] **Step 1: Testes que falham:**
  - `TestListaDeAtualizacoesSoMostraAsDoEscopo` (gestor de `adm-a` não vê as de `adm-b`).
  - `TestDecidirPelaAPI` (201 `{numero, estado: "aceita"}`).
  - `TestDecidirComCabecaVelhaResponde409PelaAPI`.
  - `TestDecidirSemPermissaoResponde403` (operador).
  - `TestPublicarBaseSoComPublicarBase` (gestor 403; construtor 201).
  - `TestPublicarBaseQueQuebraCamadaResponde409ComAsCamadas`.
  - `TestCienteDoAviso` (204; segunda listagem sem o aviso).
  - `TestContratoDasAtualizacoes` (3 fixtures).
- [ ] **Step 2: Rodar e ver falhar.** Expected: FAIL.
- [ ] **Step 3: Implementar**; gerar e conferir as fixtures.
- [ ] **Step 4: Rodar e ver passar** (+ Go tudo). Expected: PASS.
- [ ] **Step 5: Commit**

```bash
git add cmd/okf-admin/camadas_atualizacoes.go cmd/okf-admin/camadas_atualizacoes_test.go cmd/okf-admin/testdata/contrato
```
```bash
git commit -m "feat: add update decision and base publish api"
```

---

### Task 19: Boot, rotas e configuração (Go)

**Files:**
- Modify: `cmd/okf-admin/main.go` (campo `servidor.camadas *camadas.Servico`, montagem, semente, ticker), `cmd/okf-admin/config.go`, `cmd/okf-admin/config_test.go`, `cmd/okf-admin/rotas_acesso.go` (`rotear`), `cmd/okf-admin/rotas_acesso_test.go`, `README.md`

**Interfaces:**
- Consumes: Tasks 6, 11, 16, 17, 18; `admin.Repositorio` (administradoras, condomínios, orientações por caso, `SalvarHabilitacao` com o condomínio padrão `admin.Padrao`).
- Produces:
  - `SKILLS_PRAZO_DIAS` (inteiro > 0, padrão 14; inválido ⇒ erro no boot) em `config.go`.
  - Boot, nesta ordem, depois do mapeamento: `camadas.NovoPostgres(cli)`, `camadas.NovasEfetivas(slog)`, `camadas.NovoServico(...)` com `DesligarSkillNova` gravando habilitação padrão desligada; `SemearBase`; `ImportarOrientacoes` (lido de `admin.Mapeamento`: só `PorCaso` de condomínio, não o padrão); `Recompor`; `s.atende.SkillsDe = efetivas.DoCondominio`; `go svc.RodarPrazos(ctx, time.Hour)` com o `ctx` do processo. Falha em qualquer passo antes de servir derruba o boot com erro de contexto (sem base composta, o turno cairia nos rascunhos).
  - `rotear`: `if s.camadas != nil { api := apiCamadas{svc: s.camadas, log: s.log}; s.rotasCamadas(mux, api); s.rotasAtualizacoes(mux, api) }`.
  - README: seção "Skills base e camadas" (base nos arquivos = rascunho, Publicar, camadas no Postgres, prazo e `SKILLS_PRAZO_DIAS`, permissões novas, limite de uma réplica para as efetivas, mesmo do `admin.Cache`).

- [ ] **Step 1: Testes que falham:**
  - `config_test.go`: `TestPrazoDasSkillsPadraoEh14Dias`, `TestPrazoDasSkillsInvalidoFalha`.
  - `rotas_acesso_test.go`: `TestRotearNaoEntraEmPanico` continua passando com `s.camadas` ligado; casos novos em `TestRotasExigemPermissao` para cada rota da seção Contrato (sem permissão ⇒ 403, com ⇒ status da tabela).
  - `TestBootSemeiaEUsaAsEfetivas` (ambiente de rotas com Postgres: depois da montagem, `s.atende.SkillsDe("adm-a", "c-a1", agora)("agendar-mudanca")` existe).
- [ ] **Step 2: Rodar e ver falhar.** Expected: FAIL.
- [ ] **Step 3: Implementar.**
- [ ] **Step 4: Rodar e ver passar:** Go tudo, race, e `internal/atende` `TestTurnoNoPostgresIdasAntesDaResposta`. Expected: PASS, idas inalteradas.
- [ ] **Step 5: Commit**

```bash
git add cmd/okf-admin README.md
```
```bash
git commit -m "feat: wire skill layers into boot, routes and the turn"
```

---

### Task 20: Contrato golden das camadas (front)

**Files:**
- Modify: `web/studio/src/contrato.test.ts`

**Interfaces:**
- Consumes: `lerContrato` (`src/test/contrato.ts`, da integração), fixtures das Tasks 17 e 18, schemas da Task 7.
- Produces: 5 linhas novas na tabela `parsesGoFixture`: `['camada', CamadaLidaSchema]`, `['camada-historico', HistoricoSchema]`, `['atualizacoes', AtualizacoesSchema]`, `['atualizacao', AtualizacaoSchema]`, `['publicar-previa', PreviaDaBaseSchema]`.

- [ ] **Step 1:** acrescentar as linhas. **Step 2:** Run: Vitest de `src/contrato.test.ts`. Expected: PASS; se um schema recusar uma fixture, o defeito é real: corrigir o lado errado (Go ou Zod) nesta task e dizer qual no commit.
- [ ] **Step 3: Commit**

```bash
git add web/studio/src/contrato.test.ts web/studio/src/camadas
```
```bash
git commit -m "test: pin layer api contracts to go fixtures"
```

---

### Task 21: Smoke ponta a ponta e medição

**Files:**
- Create: `docs/superpowers/medicoes/2026-11-04-smoke-skills-base.md`

- [ ] **Step 1:** subir com `./build.sh --run` (`AUTH_MODE=dev`) e seguir, registrando o que viu em cada item:
  1. Construtor muda a pergunta de uma etapa de `agendar-mudanca` e publica a v2.
  2. `gestor-exemplo` vê "Essa skill tem atualizações", abre a comparação e decide 10 trechos com **Aceitar tudo**: cronometrar (meta < 2 min).
  3. Gestor personaliza o título na administradora e publica; um condomínio dela personaliza a orientação e publica; uma conversa no Console daquele condomínio mostra o título e a orientação novos.
  4. Construtor publica a v3 mudando o mesmo título: conflito na administradora; escolher **Editar a mistura**.
  5. Construtor marca um trecho obrigatório com motivo: aparece em todas as camadas com o aviso e o motivo; a versão antiga fica no histórico.
  6. Rascunho na administradora tentando trocar a ferramenta de uma etapa: erro no bloco, nada salvo.
  7. Prazo: com `SKILLS_PRAZO_DIAS=1` e o relógio do banco adiantado (ou `prazo_em` ajustado à mão em dev), confirmar que os trechos não personalizados descem e os conflitos ficam.
  8. Voltar a uma versão anterior pelo histórico.
- [ ] **Step 2:** latência: rodar `TestTurnoNoPostgresIdasAntesDaResposta` e anotar as idas (1, 1, 0).
- [ ] **Step 3:** Go tudo, race e front tudo verdes; anotar no documento.
- [ ] **Step 4: Commit**

```bash
git add docs/superpowers/medicoes/2026-11-04-smoke-skills-base.md
```
```bash
git commit -m "docs: record skills base smoke and timings"
```

---

## Fora deste plano

- Liberar ferramenta por camada (spec: desenho próprio no futuro).
- Personalizar o `rotas.md` do orquestrador por condomínio (decisão 13).
- Trecho de texto por parágrafo (decisão 3).
- Mais de três camadas, skill própria da administradora, notificação fora do Studio (spec, Fora do escopo).
- Mais de uma réplica para as efetivas (mesmo limite do `admin.Cache`, README).
