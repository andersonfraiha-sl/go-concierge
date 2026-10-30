# go-concierge

Concierge de condomínio em Go, do zero, com as ideias do `sl-concierge`. As definições de
partida (harness Eino, OKF, emulação do ATENDE, Admin da administradora, observador de
sessão) estão em [DEFINICOES.md](DEFINICOES.md).

O que existe hoje: o **cadastro de skills no formato de pasta** (OKF) com a interface de
administração, e o **harness Eino** que roda essas pastas.

## OKF: agentes em pasta, recursos no Nexus

Dois mundos. O que é **do agente** fica estático na pasta e se edita no Admin; o que é
**infraestrutura** (tools, conteúdos, memórias) vem do **Nexus em tempo de execução**, e
o agente só declara o que precisa.

```
okf/
├── agentes/<agente>/
│   ├── agent.yaml        nome, tipo (orquestrador | operador), descricao, modelo, recursos (nomes do Nexus)
│   ├── persona.md        traços e tom de quem fala com o morador; SÓ no orquestrador de primeiro nível
│   ├── caso.yaml         caso de uso determinístico (passos do catálogo); a única porta de ESCRITA
│   ├── rules/            output.md = contrato de saída; os outros .md são políticas
│   ├── workflows/        principal.md = o que ele faz; os demais são procedimentos
│   ├── routers/          rotas.md (critério → rota `nome`) e desambiguacao.md
│   ├── knowledge/**      conhecimento OKF: glossary.yaml e docs tipados (herda para os filhos)
│   ├── assets/**         templates e rodapés, citados como `assets/<arquivo>` (herda)
│   ├── evals/*.yaml      casos que provam o comportamento
│   └── agentes/<sub>/    subagentes, mesma anatomia
└── nexus/<account>.yaml  instantâneo dos recursos da account (usado sem NEXUS_BASE_URL)
```

**Conhecimento OKF** em `knowledge/`: `glossary.yaml` (termos, sinônimos, definição) e
documentos Markdown com frontmatter tipado `type: Domain | Vocabulary | Question |
Condition | Policy | Procedure`. Um `Question` declara `answers:`, `## Method` e
`## Tie-breaking`.

**Caso de uso** (`caso.yaml`): o formato do catálogo do sl-concierge (`params`, `steps`
com `resource`, `choose`, `collect`, `form`, `confirm`, `compose`, `output`). O harness
executa os passos; um recurso de escrita só é chamado por um passo do caso.

**Recursos do Nexus**: `agent.yaml → recursos` declara os de leitura; os passos do caso
declaram os de escrita. O harness cruza a declaração com o que a account oferece e o que
a identidade do morador pode (privado só autenticado; escrita só pelo caso).

Duas regras duras, herdadas do harness Python `adam`:

- **Herança**: knowledge e assets do pai valem para os filhos (o mais próximo vence).
  Persona (só do orquestrador de primeiro nível), rules, workflows, routers, evals e caso
  são de cada um.
- **Fronteira**: um agente só referencia a própria pasta ou um ancestral. Irmão, sobrinho
  e fora da árvore são proibidos.

## O Console: emulador do ATENDE

Em `/console`. O ATENDE manda um evento por webhook a cada mensagem do morador
(`com.echo.atende.user.protocol.created`, com protocolo, contato, canal e mensagem); o
concierge identifica o telefone no **Gruvi** (token B2C compartilhado com o MCP + BFF por
`filter[mobile-number]`), roda o agente de primeiro nível e responde ao protocolo. O
Console forja esse evento, manda a `POST /webhook/atende` e mostra a conversa no
telefone, a identidade (unidades do Gruvi), a rota com tokens e a caixa de saída.
Configuração no `.env`: `GRUVI_COMPANY_ID`, `GRUVI_BFF_URL`, `MCP_AUTH=b2c` e
`MCP_B2C_MOBILE`; opcionalmente `ATENDE_WEBHOOK_TOKEN`, `ATENDE_REPLY_URL` (envio ao ATENDE, contrato a
confirmar).

Enquanto o token B2C é gerado para um único telefone, o atendimento e a consulta de
identidade aceitam somente o telefone configurado em `MCP_B2C_MOBILE`.

O webhook converte o evento do ATENDE para o contrato de entrada compartilhado
`internal/entrada.Mensagem`, recebido pelo processamento do concierge. Outro canal pode
adaptar sua mensagem para esse contrato; identidade por telefone, envio da resposta e
identificação única das conversas entre canais ainda precisam ser definidos para ele.

## Memória relacional por contato

Tudo o que o morador afirma sobre si ("só tenho um apartamento", "pode me chamar de
Van", "tenho um cachorro") vira um fato com chave estável na memória do telefone
(`internal/memoria`, JSON em `okf/var/memorias/<account>/<telefone>.json`). A cada
troca um extrator (modelo `MODEL_EXTRACT`, ou regras sem modelo) captura o que é
durável; em toda conversa seguinte a memória entra na instrução da família inteira
como "Memória sobre o morador: use sem perguntar de novo". O Console mostra e apaga os
fatos (`GET`/`DELETE /api/memoria?phone=`).

## Ficha do condomínio (Admin)

Cada condomínio tem uma ficha com o que o concierge faz nele (liga/desliga por caso de
uso), **Orientações ao concierge** (texto livre geral e por caso de uso, injetado no
prompt), **Ajustes** (formulário gerado das `definicoes` que os recursos declaram no
catálogo, gravado no payload do condomínio) e **Identificação**. `#/condo/*` é a ficha
do padrão, que vale para todos. API: `PUT /api/admin/orientacao`, `PUT /api/admin/ajustes`.

## Classes de recurso do Nexus

O catálogo da account (`okf/nexus/<account>.yaml` ou o Nexus ao vivo) organiza os
recursos em sete classes, pelo `kind`: **Leitura** (`api`/`database` que só leem),
**Escrita** (`api`/`database` com `write: true`), **Conhecimento** (`knowledge`),
**Memória** (`memory`), **Ação** (`action`: APIs que executam algo no mundo; `activity`,
uma atividade cadastrada no workspace da administradora, cai aqui também), **Agente**
(`agent`: executor do Nexus que recebe um pedido e devolve um resultado) e **Form**
(`form`: procedimento externo, chamada pública via formulário). `write` é
ortogonal à classe: qualquer recurso com `write: true` só entra por um passo do
`caso.yaml`, depois do `confirm`; os demais o agente declara em `agent.yaml → recursos`
e chama com `chamar_recurso`.

Cada recurso do instantâneo se edita no Backdoor (lápis no cartão ou "Novo recurso"):
nome, tipo, visibilidade, efeito (`write`), descrição, **conexão** (método, URL com
`{{param}}`, headers, corpo), **parâmetros de entrada** e **saída** (resumo e campos
com caminho no JSON). Fica em `conexao` e `saida` no YAML, documental: quem executa é o
Nexus/MCP. Com o Nexus ao vivo, a edição é recusada (o dono é o Nexus).

**Mock do concierge.** Sem `NEXUS_BASE_URL`, `chamar_recurso` responde pela `amostra` que
cada recurso traz no instantâneo (editável no lápis); escrita sem amostra devolve um
protocolo gerado (`MOCK-000123`) ecoando os params. `unidades.listar` vem da identidade
do Gruvi. Assim o emulador roda inteiro sem integração; depois, com o Nexus no ar, nada
muda no agente.

## Pacotes OKF v0.1

A caixinha `knowledge/` lê um pacote do [Open Knowledge Format](https://cloud.google.com/blog/products/data-analytics/how-the-open-knowledge-format-can-improve-data-sharing)
copiado inteiro: markdown com frontmatter (`type` obrigatório; `title`, `description`,
`resource`, `tags`, `timestamp` lidos e mostrados na ficha), conceitos ligados por links
relativos, `index.md` por pasta e `log.md`. O índice do prompt faz a progressive
disclosure: numa pasta com `index.md` só ele aparece, e o agente navega abrindo-o com
`ler_dado`; os links relativos de um documento voltam reescritos para o caminho
canônico (`knowledge/tables/foo.md`), que `ler_dado` aceita. `log.md` fica fora do índice
e da lista de docs, mas continua legível. Ver `internal/okf/bundle.go`.

## A interface (okf-admin)

```bash
./build.sh            # gofmt + vet + test + bin/okf-admin.exe e bin/okf-admin (no contêiner, sem Go local)
./bin/okf-admin.exe -root ./okf -addr 127.0.0.1:8090 -open
```

Três áreas na barra, nesta ordem: **Atendente** (`/console`, o emulador do ATENDE: a conversa do morador como o atendente a vê), **Admin** (`/`, a administradora: seus condomínios e a matriz de
casos de uso; o seletor no topo emula o login e o servidor recorta o mapeamento por
`?administradora=<id>`) e **Backdoor**
(`/backdoor`, nosso: casos de uso do concierge, `/backdoor/administradoras` com as
administradoras e os condomínios de cada uma, `/backdoor/payloads` com o payload de
cada recurso do Nexus por condomínio).

```bash
```

Três colunas:

- **Esquerda**: a árvore de agentes (arraste para mover, Shift+arraste para clonar; solte
  no título `agentes/` para levar à raiz) e os **recursos do Nexus** da account, por
  classe, com o interruptor "este agente usa" (grava `agent.yaml → recursos`). Mover ou
  clonar acerta o `routers/rotas.md` dos dois pais.
- **Centro**: a ficha do agente: problemas encontrados, identidade (`agent.yaml`),
  **persona** (própria ou herdada), **caso de uso** (os passos em linha do tempo),
  **recursos que usa** (com classe e estado), caixinhas com contagem próprio/herdado
  (clique no `+` para criar um arquivo com modelo), os **vínculos** (assets,
  conhecimento, rotas, subagentes, recursos, persona) com o estado de cada um e a dica de
  correção, os subagentes, o conhecimento OKF (docs tipados e glossário) e o índice que o
  prompt enxerga.
- **Direita**: os arquivos (editor, Ctrl+S salva), a **projeção** (o system prompt
  compilado da pasta, no princípio ICM) e o **roteador** (só o que decide a rota).

`+ novo agente` abre o assistente: onde (raiz ou dentro de um agente), nome, tipo,
descrição (o critério que o roteador do pai lê), `persona.md` e `caso.yaml` opcionais,
caixinhas iniciais com arquivo-modelo e os recursos do Nexus que declara. `importar zip`
instala uma pasta inteira.

### API

| método | rota | o que faz |
|---|---|---|
| GET | `/api/agentes` | árvore resumida (tipo, descrição, erros, avisos) |
| POST | `/api/agentes` | cria a pasta a partir do formulário (`NovoAgente`) e registra a rota no pai |
| GET | `/api/agente?agente=` | ficha completa: arquivos por caixinha (próprios e herdados), vínculos, conhecimento, dados, problemas |
| PUT | `/api/agente/config` | atualiza `agent.yaml` (nome, tipo, descricao, modelo, fontes, apis, segredos) |
| DELETE | `/api/agente?agente=` | exclui a pasta e tira a rota do pai |
| POST | `/api/agentes/mover` | `{origem, destino, clonar}` |
| POST | `/api/agentes/importar?caminho=` | zip no corpo |
| GET | `/api/agentes/exportar?agente=` | zip da pasta |
| GET/PUT/DELETE | `/api/arquivo` | um arquivo dentro de uma caixinha |
| GET | `/api/arquivo/modelo?caminho=` | conteúdo inicial para um arquivo novo |
| GET | `/api/projecao?agente=` | system prompt compilado e roteador |
| GET | `/api/nexus/recursos` | os recursos da account (Nexus ao vivo ou instantâneo) |
| POST | `/webhook/atende` | o evento do ATENDE; 202 e processamento em segundo plano |
| GET | `/api/atende/sessao?protocolo=` | a conversa do protocolo (turnos, identidade, rota) e a caixa de saída |
| GET | `/api/atende/identidade?phone=` | consulta a identidade no Gruvi |
| POST | `/api/agente/recursos` | `{agente, nome, ligar}`: declara ou tira um recurso em `agent.yaml → recursos` |

## O harness Eino (okf-run)

`internal/harness` transforma a pasta num agente Eino: a instrução é a projeção, as
tools são as nativas do formato (`ler_dado`, `abrir_workflow`, `consultar_schema`,
`chamar_api`, `invocar_agente`), o orquestrador roteia pelo `routers/` e o contexto
passa ao subagente (até 4 níveis). `cmd/okf-run` roda pela linha de comando:

```bash
cp .env.example .env            # LLM_MODE=gemini + GEMINI_API_KEY (ou openrouter + OPENROUTER_API_KEY)
./bin/okf-run.exe -root okf "posso fazer barulho depois das 22h?"
./bin/okf-run.exe -root okf --inspecionar    # projeção, roteador e tools, sem chamar modelo
./bin/okf-run.exe -root okf -mock "..."      # modelo falso, sem rede
```

O provedor vem do `.env` com os mesmos nomes do sl-concierge: `LLM_MODE` (`gemini` |
`openrouter` | `mock`), a chave correspondente e `MODEL_PLANNER` como modelo padrão
quando o `agent.yaml` não diz. Gemini e OpenRouter sem chave rodam no modo mock.

Para o gateway Luna, configure `LLM_MODE=luna`, `LLM_LUNA_OPENAI_URL`,
`LLM_LUNA_GATEWAY_TOKEN` e `LLM_LUNA_MODEL`. O cliente usa a API Responses e envia
os headers `x-company-id: internal` e `x-origem: workspace`. Deixe `MODEL_PLANNER`,
`MODEL_EXTRACT` e `MODEL_CHEAP` vazios para usar `LLM_LUNA_MODEL` em todas as tarefas,
ou configure modelos disponíveis no gateway. Um modelo definido no `agent.yaml` ou
passado por `-modelo` continua tendo precedência no agente principal. Em `luna`, a
chave ausente é erro; para executar sem rede, use `LLM_MODE=mock`.

## Pedidos: o registro de trabalho por contato

Cada pedido do morador vira um **Pedido** com protocolo próprio (`P-7K3Q9`), ligado a um
caso de uso e aos parâmetros do `caso.yaml` dele. O estado é decidido **em código**, pelos
parâmetros capturados e validados, nunca inferido do texto de uma resposta:

```
coletando → pronto → aguardando_confirmacao → executando → concluido
         ↘ com_atendente (pendência levada ao humano) ↗      ↘ cancelado | falhou
```

A cada mensagem (`internal/atende/pedidos.go`):

1. O **detector** (`harness.ExtratorPedidosLLM`, ou `pedidos.Regras` sem chave) diz só o que
   a mensagem contém: pedidos novos, valores para pedidos abertos, sim/não a uma confirmação,
   cancelamento, pedido de atendente e o "restante" (dúvida, conversa). O sim/não a uma
   confirmação pendente é decidido em código por cima do modelo.
2. O **motor** (`internal/pedidos/motor.go`) percorre os passos do `caso.yaml`: lista recursos,
   escolhe (uma opção só entra sozinha; `reuse: always` reaproveita a unidade de pedidos
   anteriores), coleta (valida data, número, telefone, opções, listas), confirma e executa
   pela mesma permissão e guarda de escrita do harness (`InvocarRecurso`), com o sim explícito.
   Perguntas repetidas contam as vezes e levam a razão da rejeição.
3. O **restante** vai ao orquestrador em modo router. A resposta final é redigida na persona
   a partir do que o motor devolveu, preservando protocolos, datas e as perguntas.
4. Se o morador pede **atendente** (ou o concierge transfere), cada pedido pendente ganha um
   **resumo** para o humano (o que quer, o que já foi capturado, o que falta e por quê, as
   últimas falas) e vai a `com_atendente`. Na aba Transbordo do Console o atendente preenche
   o que falta e clica "Confirmar e executar": o motor executa e o resultado vai ao morador.

Onde ver: aba **Pedidos** do Console (ao lado de Memória): protocolo, caso, estado,
parâmetros com origem, pergunta em aberto, confirmação, resultado e histórico. Os pedidos
ficam em `okf/var/pedidos/<account>/<telefone>.json` (`PEDIDOS_DIR`); `PEDIDOS=off` desliga
e a conversa volta a seguir só pelo orquestrador. API: `GET /api/pedidos?phone=`,
`POST /api/pedidos/acao` (`preencher`, `cancelar`). Casos com passo `form` (formulário
dinâmico) ficam registrados, mas seguem pelo subagente conversacional.

Com os Pedidos ligados, os modos planning e react abaixo deixam de mandar: o motor de
pedidos é a espinha dorsal, e o orquestrador atende só o que não é pedido.

## Modos de orquestração: router, planning e react (ART)

O orquestrador de primeiro nível decide **como** atende a mensagem. O padrão é o
`router` de sempre (uma rota por mensagem, `routers/rotas.md`). Os outros dois trazem
o *Automatic Reasoning and Tool-use*: o orquestrador raciocina sobre o pedido inteiro,
vincula cada parte a um caso de uso ou a um recurso de leitura e executa, guardando o
estado entre turnos.

| Modo | Como decide | Quando usar |
|---|---|---|
| `router` | uma chamada barata escolhe UMA rota; o subagente atende; a persona entrega | pedidos de um item (padrão) |
| `planning` | uma chamada monta o **TODO** inteiro (`itens` com `alvo` e `confianca`); os itens rodam em ordem; perguntas de escolha saem juntas, uma vez só; resposta consolidada no fim | mensagens com vários pedidos; menos idas e vindas |
| `react` | loop **pensar → agir → observar** com checklist: uma ação por passo (`caso`, `recurso`, `perguntar`, `responder`, `encerrar`), observação de cada ação no passo seguinte | pedidos abertos, em que o próximo passo depende do resultado do anterior |

Em `planning` e `react` o caso de uso roda **inline**: quando ele pergunta algo ao
morador ("Para qual data?"), o plano **pausa** (`pendente` = o item) e a sessão guarda o
plano; a próxima mensagem vai direto ao item pendente (`harness.Continuar`), sem
re-rotear. Vínculo com `confianca` abaixo de `bind_confidence` vira pergunta de
confirmação. Escrita nunca é vínculo direto: só pelo caso de uso, com o sim do morador
(`Confirmado`). O `observador-sessao` nunca é candidato.

```yaml
# okf/agentes/concierge/agent.yaml
orquestracao:
  modo: planning          # router | planning | react  (ORQUESTRACAO_MODO no ambiente vence)
  bind_confidence: 0.8    # abaixo disso o item vira pergunta ao morador
  max_itens: 5
  max_iteracoes: 6        # react: passos por turno; no limite, consolida o que há
```

Onde se controla: a guia **Orquestração** do Backdoor (http://127.0.0.1:8090/backdoor/orquestracao):
escolha o modo e os ajustes e salve; grava o bloco acima no `agent.yaml` do concierge e vale
a partir da próxima mensagem, sem reiniciar. `ORQUESTRACAO_MODO` no ambiente, quando definida,
força o modo por cima da guia (ela avisa). Teste no Console: a aba Rota mostra o plano.

No Console, a aba **Rota** mostra o modo, o plano (itens com status, alvo e resultado) e
os passos `plano`/`item`; o `Turno` carrega `plano` e a sessão guarda `plano` enquanto
houver item esperando o morador. Código: `internal/harness/plano.go`.

## Estrutura

```
cmd/okf-admin      servidor HTTP da interface (SPA embutida)
cmd/okf-run        roda um agente pela linha de comando (harness Eino)
internal/okf       pasta de agente: carga, herança, fronteira, vínculos, conhecimento OKF, projeção, rotas, escrita, catálogo
internal/harness   adaptador Eino: modelo (Gemini | OpenRouter | Luna | mock), tools nativas, rota, trace e uso
web/okf            a interface (HTML, CSS, JS, sem dependências)
okf/               o catálogo de exemplo: concierge (orquestrador) + observador-sessao, regimento, faq
```

Próximas peças, na ordem das definições: harness Eino (tools + agentes OKF), emulador do
ATENDE, sessão com pendências, catálogo de casos de uso YAML, Admin da administradora.

## Catálogo MCP oficial e Docker

O servidor MCP não publica porta (só escuta em loopback, no namespace de rede do
concierge). Para testar de fora, o concierge repassa `/mcp/...` a ele:
`http://127.0.0.1:8090/mcp/gruvi/bff` (Streamable HTTP; mande o seu
`Authorization: Bearer <token B2C>`, o repasse não injeta o token do `.env`). Sem token
o MCP responde 401. No MCP Inspector, transporte *Streamable HTTP* com essa URL e o
header de autorização.

O servidor MCP da Superlógica (entrega `mcp-catalog-agent`, vendorado em `mcp/`) transforma as
OpenAPIs do BFF do Gruvi, `building-operation.json` e `mobile.json`, em tools MCP com nome
`gruvi.bff.<módulo>.<operação>`, no endpoint `/mcp/gruvi/bff`. O concierge usa esse catálogo
como **fonte de recursos do Nexus** (`internal/mcp`): `tools/list` vira a lista de recursos da
account (classe `api`, privados, escrita pela anotação `readOnlyHint` ou pelo verbo do nome) e
`chamar_recurso` executa por `tools/call`. Um agente declara as tools que usa em `recursos:` do
`agent.yaml`, como qualquer recurso; o Backdoor marca cada uma com **MCP**.

O servidor exige um Bearer em cada requisição e repassa esse token ao BFF. Ele valida
apenas o formato do Bearer; o BFF é responsável pela autenticação e autorização.

- **`MCP_AUTH=b2c`** (padrão): o concierge obtém o token pelo fluxo SMS de dev do Spaces
  (`POST /auth/sms` com `validation=dev` → `session_id`; `POST /auth/token` com
  `grant_type=password`, `sms_code=999999`), renova pelo `refresh_token` (60 dias) 45 s antes
  de vencer e grava o token corrente em `MCP_TOKEN_FILE` (`okf/var/b2c.token`, ignorado pelo
  git). Configura-se só `MCP_B2C_MOBILE` (telefone de teste); URL, Basic do app, código e
  fingerprint têm padrão.
- `MCP_AUTH=file` lê o token corrente de `MCP_TOKEN_FILE` gravado por outro processo;
  `bearer` usa um token fixo (`MCP_BEARER_TOKEN`); `keycloak` pede um token de serviço por
  `client_credentials`, quando o servidor aceitar esse token.

O servidor só aceita o header `Host` em loopback; o cliente envia `127.0.0.1:3337` mesmo
chegando pelo nome do serviço. Para testar o MCP de fora pelo repasse `/mcp/gruvi/bff`, use
o token corrente de `okf/var/b2c.token` no `Authorization: Bearer` (ele troca a cada 5 min).

```bash
cp mcp/.env.example mcp/.env   # ou o .env da entrega, com as credenciais
docker compose up -d --build   # mcp (Node 24, porta 3337) + concierge (Go, porta 8090)
```

Em Linux, quando os arquivos `okf/` pertencem a outro usuário, prepare o acesso do
usuário 10001 do contêiner ao catálogo e aos arquivos criados posteriormente:

```bash
mkdir -p okf/var
setfacl -R -m u:10001:rwX okf
find okf -type d -exec setfacl -m d:u:10001:rwx {} +
```

Admin, Console e Backdoor em http://127.0.0.1:8090. O catálogo (`okf/`) é montado do repositório,
então o Admin edita os arquivos de verdade; `web/okf` também é montado (`-web`), então o front
se edita sem rebuild. O front não tem contêiner próprio: é o concierge que o serve. A POC do MCP
só escuta em loopback, por isso o `mcp` roda na rede do contêiner `concierge` e o Explorer
(porta 3337) não fica acessível do host; para usá-lo, rode o MCP local com
`cd mcp && npm ci && npm run start:mcp:catalog-agent`.

**401 do MCP?** O concierge registra "catálogo indisponível (não autorizado)" e segue só com o
Nexus ou o instantâneo. Causas: `MCP_B2C_MOBILE` sem acesso (o `/auth/token` falha), ou o
`concierge` recriado sem recriar o `mcp` (ele mora
no namespace de rede do concierge: `docker compose up -d --build` recria os dois).
