# Continuidade do Concierge v2

## Separação harness/core — implementação concluída em 24/11/2026

Branch `feat/separacao-harness-core`, base `c60bbea`. Mesmo processo Concierge, três apps preservados. Referência atual: [SEPARACAO-HARNESS-CORE.md](SEPARACAO-HARNESS-CORE.md). As seções seguintes são histórico das rodadas anteriores.

- `internal/harness/{atendimento,agentes,pedidos,memoria,ganchos,conteudo,configuracao,canais,sessoes,persistencia,identificacao}` contém comportamento e contratos. Teste de arquitetura verifica dependências transitivas, sem core/Ent/PostgreSQL.
- `internal/core` contém APIs, clientes, consultas, transações, administração, catálogo e infraestrutura de banco. `apps/concierge/cmd/concierge` passou a ser a entrada curta do executável. Um único `go.mod`, sem cópia das implementações.
- Runtime, estado administrativo, conteúdo, auditoria, catálogo, regras, retenção, entrega e CLI conectado usam as fronteiras HTTP. Credenciais efêmeras em loopback, prova administrativa assinada e recarga das permissões por operação. Operações que exigem resultado conhecido não repetem escritas incertas.
- Boot/migrations/seed/materialização inicial/importação de infraestrutura permanecem no core. Banco v2 em `internal/core/banco`; geradores Ent/sqlc e baseline reproduzidos sem mudança do schema. Makefile, scripts e CI acompanham os novos caminhos.
- Builds, vet, lint, arquitetura, geradores e testes focados aprovados. Smoke de processo com PostgreSQL descartável passou: boot, readiness, rotas administrativas, regras, configuração do CLI e encerramento por SIGTERM. CLI offline também passou com modelo mock. Evidências e seis falhas preexistentes da suíte global em [VALIDACAO-HARNESS-CORE.md](VALIDACAO-HARNESS-CORE.md); logs em `/tmp/concierge-harness-core/`. Não repetir checks aprovados por simples retomada.
- Bwrap ainda falha com `bwrap: loopback: Failed RTM_NEWADDR: Operation not permitted`. Duas sessões Terra 6.1 high foram tentadas; principal assumiu conforme autorização do usuário. Comandos elevados locais de leitura/patch/validação funcionam. Erros literais em `.temp/ERROS-FERRAMENTAS-HARNESS-CORE.md`.
- Nenhum restart, push ou deploy. As integrações reais e o banco da stack existente não foram usados nos testes.
- Próximo passo: combinar a aplicação local e o teste manual pelo Console. O processo ativo ainda usa a imagem anterior. O schema foi preservado e este refactor não requer reset de banco nem remoção de volumes.

## Separação das aplicações concluída — 23/11/2026

- Base anterior registrada, a pedido do usuário, no commit `efbcc2f`, na branch `feat/refactor-identidade-canais-db`; workspace estava limpo. A separação fica registrada na branch `feat/separacao-aplicacoes`, criada a partir dessa base. Sem push.
- Marco 1 concluído: Backdoor e Console com frontend/API próprios, Concierge com runtime e persistência atual. Código em `apps/{backdoor,console,concierge}`, preservando `cmd`, `internal` e `web`; domínio e `go.mod` continuam na raiz. Guia operacional: `SEPARACAO-APLICACOES.md`.
- Backdoor em `http://127.0.0.1:8091/studio/`, Console em `http://127.0.0.1:8092/console` e Concierge/API/webhook em 8090. As aplicações visuais consomem HTTP do núcleo, sem dependências de banco/runtime. O núcleo valida entregas emuladas antes de o Console armazená-las.
- Três imagens finais construídas e Composes raiz/individuais válidos. `build.sh` e documentação atualizados. `.env` local recebeu somente `CONCIERGE_GOOGLE_CREDENTIALS_FILE`, com o caminho absoluto da credencial já utilizada; demais configurações preservadas.
- Testes focados de infraestrutura HTTP, APIs visuais, arquitetura, autorização e envio passaram. `TestConsoleSeparadoProcessaConversa` passou com processo real do Console e LLM simulado. Typecheck/build do Studio, seis arquivos/90 testes, Biome dos arquivos alterados, `go vet` dos pacotes afetados e golangci-lint v2.15.0 (zero achados) passaram.
- Smoke das imagens visuais finais aprovado: páginas, assets, readiness e APIs de leitura responderam 200. Somente GET contra o núcleo já ativo; sem autenticação B2C, conversas LLM ou operações de negócio reais. Containers temporários removidos; IDs/inícios dos serviços preexistentes permaneceram iguais. Evidência: `/tmp/concierge-separacao-20261123/smoke-imagens.json`.
- Uma execução anterior do pacote principal inteiro revelou duas famílias de falhas de fixtures: `TestSessoesFicamNaImplantacao` e `TestE2EGanchosRegrasDeExemplo`. Ambas foram reproduzidas na base `efbcc2f` (`falhas-base.log`), sem relação com a separação. Expectativas de páginas extraídas foram adaptadas; testes HTTP finais passaram. Não executar novamente o pacote inteiro por retomada de contexto.
- Demais evidências em `/tmp/concierge-separacao-20261123/`: `build-docker-final.log`, `testes-http-encerramento.log`, `http-focado-final.log`, `studio-checks.log`, `studio-biome.log`, `vet-afetados.log` e `lint-afetados-final.log`. Não foram usados subagentes nem executadas as suítes globais Go/Studio nesta rodada.
- Stack existente não reiniciada, banco e volumes não alterados; rede `go-concierge_default` e volume `go-concierge_pgdata_v2` preservados. O usuário pode aplicar com `docker compose up -d --build` e testar pelas novas portas.
- Próximo marco, ainda sem implementação: API de dados consumida por todos, incluindo o próprio runtime. Confirmar com o usuário antes de iniciar futuras implementações; ele autorizou concluir esta rodada e reforçou economia de tokens, ausência de subagentes e testes proporcionais. Não repetir checks aprovados sem mudança relevante.

## Histórico anterior à separação

Os registros abaixo preservam os caminhos, comandos e limites das rodadas anteriores. Para estrutura, execução e pendências atuais, usar a seção acima e `SEPARACAO-APLICACOES.md`.

Branch: `feat/refactor-identidade-canais-db`
Base do refactor: `8ad19a2` (baseline v2). Implementação e validação de fechamento concluídas; consulte `git log` para os commits desta etapa. Sem push e sem substituir bancos ou volumes preexistentes.

## Estado

O backend usa a persistência v2, com empresa Nexus obrigatória, canal persistido e conteúdo global → empresa → condomínio. Canais e ativações são configurados por SQL nesta etapa; não foram criadas APIs/telas de gestão. Referências: `internal/banco/v2/RUNTIME.md` e `docs/modelagem/concierge-v2/PERSISTENCIA-PESSOAL.md`.

`ValidarRegistroSessao` exige igualdade entre empresa, telefone e condomínio do JSON e das colunas. Studio e Copiloto usam `Caso.ID`; caso desconhecido mantém sua chave original. A documentação principal foi atualizada; o golden mudou apenas o exemplo de condomínio de `cond-a` para `101`. Código que perdeu seus chamadores foi removido após comparação do lint com a base.

Preservar sempre: empresa é o recorte mínimo; canal estrito Alpha exclui Beta e NULL; snapshots parciais/vazios nunca autorizam apagar outro alvo; identificadores Nexus, Gruvi e account continuam distintos. Não remodelar prompts nem iniciar separação de frontends nesta etapa.

Validação de fechamento em 18/11/2026:

- Uma execução de `TZ=UTC go test -p 1 -race ./...`; quatro expectativas antigas falharam e passaram em retestes focados com `-race` após correção. Sem repetir a suíte completa.
- Baseline `-check`, `go vet ./...`, golangci-lint v2.15.0 e formatação passaram. Ajustes finais de canais/lote tiveram testes focados com `-race`; CLI teve build e smoke.
- Studio: Vitest completo com dois workers passou (70 arquivos, 1.160 testes); Biome dos arquivos alterados passou. Typecheck/build aprovados anteriormente foram reutilizados.
- Gitleaks v8.30.1 passou no histórico e nos arquivos candidatos ao commit. O wrapper passou em sintaxe e smoke sintético: saída limitada a 80 linhas e código de falha preservado.
- Relatório detalhado de comandos, resultados e logs: `/tmp/concierge-validacao-fechamento.md`.

Evidências anteriores preservadas:

- `/tmp/concierge-sessao-v2-tests.log`: pacote `internal/atende` passou.
- `/tmp/concierge-studio-canonicos.log`: 3 arquivos e 49 testes passaram, incluindo `src/copiloto/aplicar.test.ts`.
- `/tmp/concierge-golden-v2.log`: `TestContratosDoStudio -args -atualizar-contrato` passou.
- `/tmp/concierge-studio-tests-final.log` e `/tmp/concierge-http-root-final.log` são anteriores às correções correspondentes e contêm falhas antigas; não representam o estado atual.

## Testes econômicos

Na raiz, `make` e `make test-focado` executam por padrão `go test ./internal/atende -run '^TestV2'`. Ajuste pacote e regex:

`make test-focado PKG=./internal/atende TEST='^TestV2SessaoAutorizaEscopoEValidaDados$'`

Para uma integração isolada, selecione explicitamente pacote e teste:

`make test-integracao INTEGRACAO_PKG=./internal/atende INTEGRACAO_TEST='^TestV2SessaoAutorizaEscopoEValidaDados$'`

A suíte completa é opt-in por `make test-completo`. Os comandos Go executam com `TZ=UTC`, gravam o log completo em `/tmp` (ou `LOG_DIR=...`) e preservam o código de saída. Em sucesso exibem apenas o resumo; em falha exibem até 80 linhas de diagnóstico e informam o caminho do log. Pacotes contendo `...` são recusados nos modos focado e integração. Nenhum alvo adiciona `-race`. Para a regressão de orientação do Studio: `cd web/studio && pnpm exec vitest run --maxWorkers=2 src/copiloto/aplicar.test.ts`.

## Próxima rodada

- Retomar pelos commits e por este handoff; não repetir discovery nem checks aprovados sem mudança relevante.
- O Compose local aponta `DATABASE_URL` e `POSTGRES_DB` para `concierge_v2` e usa `pgdata_v2`, deixando o volume legado `pgdata` intacto. `BOOTSTRAP_DEV_ATENDE` vem de `.env` e assume `true`; a seed em dev ativa casos apenas quando a empresa ainda não tem canais nem configurações de casos. Exige `NEXUS_COMPANY_ID` explícito e as configurações ATENDE listadas em `internal/banco/v2/RUNTIME.md`.
- A seed mantém `condo_id = NULL`, roles vazios e credenciais em `channels`; não deduz empresa/condomínio ERP de Gruvi ou account. Reinícios não recriam ativações removidas nem sobrescrevem dados gerenciados por SQL. O reset local `docker compose down -v` é manual e opcional; não foi executado.
- Validação deste marco: `go test -count=1 ./cmd/okf-admin -run '^TestBootstrapAtendeDev'` e `docker compose config --quiet` passaram, incluindo `BOOTSTRAP_DEV_ATENDE=false`. O teste usou somente o Postgres descartável `bancotest`; log em `/tmp/concierge-bootstrap-dev-tests.log`.
- O Compose prepara o banco v2 dedicado e a configuração DEV inicial; a gestão posterior continua por SQL conforme `RUNTIME.md`. O banco legado permanece preservado.
- Definir com o usuário o próximo marco. APIs administrativas, integração visual do plugin e separação de frontends continuam etapas futuras.

## Console local — 19/11/2026

- O `.env` local usa empresa Nexus explícita, `MCP_AUTH=none` e `NEXUS_BASE_URL` vazia. O contato é público, sem morador/unidades simulados como autenticados. O Console preenche telefone de teste somente em DEV e oculta a identificação externa desativada.
- O usuário fixo `admin-superlogica` foi cadastrado idempotentemente no banco v2 ativo, sem reset; `garantirAdminDev` agora o prepara no boot exclusivamente DEV com autenticação DEV. Dois testes focados passaram.
- Configuração e webhook do Console usam o escopo da empresa quando a implantação é fixa; implantação global mantém a exigência global. Identidade v2 autoriza empresa e valida canal; legado mantém o alvo global. Sessenta testes focados passaram, incluindo rejeição de empresa externa, e o JavaScript passou na checagem de sintaxe.
- Imagem `concierge` reconstruída; somente `concierge` e `mcp` recriados. Postgres e volumes preservados. `/readyz`, `/api/eu` e `/api/atende/config` retornaram 200. Logs: `/tmp/luna-console-local-build.log` e `/tmp/luna-console-local-up.log`.
- A API local de recursos confirmou `faq.buscar` no snapshot `cond-001`; Console servido com `console.js?v=9` identifica o instantâneo local. Nenhuma conversa LLM ou chamada real B2C/Nexus foi executada nessa validação; o teste manual da resposta continua com o usuário.

## Luna e envio do emulador — 19/11/2026

- Protocolo `098404709013`, empresa `admin112698`: pergunta sobre horário da piscina falhou no Vertex com `invalid_grant / Invalid JWT Signature`; o envio da resposta de erro recebeu 403 na rota local do emulador. O usuário confirmou que o acesso ao Vertex foi retirado.
- `.env` local: `LLM_MODE=luna`, com o valor Vertex anterior preservado comentado. Token já configurado em `LLM_LUNA_GATEWAY_TOKEN`; modelo `LLM_LUNA_MODEL=gpt-6-luna`; URL do gateway já configurada. `MODEL_*` estão vazios, permitindo usar o modelo Luna. Não copiar credenciais para este documento.
- Correção implementada por Luna 6 com contexto isolado em `cmd/okf-admin/atende.go` e `rotas_acesso_test.go`: `/emulador/atende/messages` usa o alvo do Console; na implantação fixa valida empresa, sessão emulada e protocolo antes de registrar resposta. Mantém permissão StudioAvançado e o acesso global quando não há empresa fixa.
- Testes focados aprovados pelo subagente: `go test ./cmd/okf-admin -run 'TestAtendeMensagemEmuladaNoEscopoDaImplantacaoFixa|TestAtendeConfigNoEscopoDaImplantacaoFixa|TestRotasExigemPermissao' -count=1`. Também passou `git diff --no-ext-diff --check`. Não repetir suíte geral por retomada de contexto.
- Aplicação local concluída: build somente de `concierge` (frontend em cache), seguido de `docker compose up -d --no-deps --force-recreate concierge mcp`, sem recriar Postgres ou volumes. Logs `/tmp/concierge-luna-emulador-build.log` e `/tmp/concierge-luna-emulador-up.log`; ambos os comandos encerraram com código 0.
- Verificação após aplicação: `/readyz`, `/api/eu` e `/api/atende/config` retornaram 200; provedor ativo `luna`, modelo do container `gpt-6-luna`, token presente sem exposição, resposta emulada ativa e container saudável. Não foi gerada nova conversa LLM para validar o token; próximo passo é o teste manual pelo usuário no Console.
- Ao retomar em nova sessão, ler este handoff e consultar apenas evidências pertinentes. Gestão/plugin/separação de frontends permanecem fora deste ajuste; alterações atuais ainda sem commit.

## Teste manual do Console aprovado — 19/11/2026

- Fluxo executado no Chrome via Playwright em `http://127.0.0.1:8090/console`, às 15h30 BRT, sobre `1d11bb5` com as alterações locais descritas acima. Nenhuma mudança de código nem novo build nesta rodada.
- Canal `c0bac16c-2a96-5862-b517-d354e1c73b71`, empresa `admin112698`, contato DEV público `5511999990001`, protocolo novo `113018782941`. Identificação externa permaneceu desativada.
- Pergunta enviada pela interface: “Qual é o horário de funcionamento da piscina?”. O modelo `gpt-6-luna` consultou `knowledge/faq.md` e respondeu: “Olá! Sou o concierge do condomínio. A piscina funciona todos os dias, das 8h às 22h. Crianças de até 12 anos devem estar acompanhadas.” Foram quatro chamadas ao modelo, com 6.361 tokens (6.234 de entrada e 127 de saída).
- Webhook emulado retornou 202; o envio da resposta em `POST /emulador/atende/messages` retornou 200, sem o 403 anterior. Sessão finalizada com dois turnos, uma saída enviada e uma mensagem recebida pelo emulador; a interface exibiu a confirmação de entrega. Sem erros JavaScript ou respostas HTTP de erro nas requisições locais observadas pelo navegador.
- A conversa foi reaberta pela lista após recarregar a página, mantendo resposta e confirmação. Nenhuma conversa preexistente foi apagada; Postgres e volumes foram preservados.
- Alcance: resposta com FAQ do instantâneo local e entrega emulada aprovadas. O backend registrou catálogo MCP indisponível por autenticação (401); esta rodada não valida ferramentas MCP autenticadas nem chamadas reais B2C/Nexus/ATENDE.
- Evidências em `/tmp/concierge-console-manual-20261119/`: `resultado.json` (sessão e entrega), `console.png` (captura após reabrir a conversa), `backend.log` (202/200 e indisponibilidade MCP), `validar.cjs` e `capturar.cjs` (roteiros; ambos encerraram com código 0). Nenhuma suíte geral foi repetida.
- Pendência anterior do teste manual Luna/emulador encerrada. Alterações continuam sem commit; próximos marcos de gestão/plugin/separação de frontends precisam de escopo próprio.

## B2C preparado para novo teste — 19/11/2026

- A pedido do usuário, identificação externa reativada no `.env` local com `MCP_AUTH=b2c`; `MCP_B2C_MOBILE` já estava atualizado para `5556999999049` e foi preservado. Luna, empresa Gruvi, endpoints e credenciais foram mantidos.
- Configuração efetiva do Compose validada: modo B2C, telefone novo, MCP/Gruvi preenchidos e `LLM_MODE=luna`. Nenhuma chamada de autenticação ou consulta externa foi executada; geração do token e identificação ainda precisam do teste real.
- Os serviços não foram reiniciados nesta rodada: o usuário fará a aplicação e o teste. Usar `docker compose up -d --no-deps --force-recreate concierge mcp` para carregar o `.env` novo; `docker compose restart` mantém o ambiente antigo. Postgres e volumes não precisam ser recriados.
- Após aplicar, recarregar `/console` e iniciar nova conversa: o telefone vem de `telefone_b2c`, e o botão “Identificar no Gruvi” deve ficar disponível. Conversas antigas mantêm seus contatos; não reutilizar a sessão pública anterior para validar o número novo.

## B2C e unidades confirmados — protocolo 117026465090

- Usuário aplicou a configuração e testou. Consulta somente de leitura confirmou `MCP_AUTH=b2c`, telefone `5556999999049` e `LLM_MODE=luna` no container ativo. Nenhuma nova autenticação foi provocada pela investigação.
- Sessão da empresa Nexus `admin112698`: `authenticated=true`, nome Luiz Leite, papel morador, empresa Gruvi `13719` / FC Guto Com Portaria, sem erro de identidade. Uma unidade retornada: 42, ID Fastpass `323`, ID interno `15c91bb4-673c-416c-a9f5-d23d47e5b221`; o campo bloco veio com espaço em branco.
- Token presente em `okf/var/b2c.token`, com emissão JWT em 19/11/2026 às 17h00min39s BRT e expiração às 17h05min39s BRT. Só existência/datas foram inspecionadas, sem expor token. Logs não registram diretamente `/auth/sms` e `/auth/token`; identidade autenticada comprova sucesso da consulta BFF com bearer, e o arquivo confirma emissão de token no fluxo ativo. O provedor reutiliza o token e renova quando necessário.
- Pedido `P-SXU4S3WZ` avançou para `aguardando_confirmacao`, para churrasqueira em 21/11/2026; não houve confirmação nem criação de reserva nesta investigação. Detalhe de apresentação observado: a confirmação mostra o ID interno da unidade em vez do rótulo 42. Não corrigido nesta rodada.
- Log local consultado: `/tmp/concierge-protocolo-117026465090-backend.log`; nenhuma alteração de código ou reinício nesta validação.
