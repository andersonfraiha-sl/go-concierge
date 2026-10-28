# go-concierge — Definições de partida

Projeto novo, em Go, do zero, com as ideias do `sl-concierge` (o concierge de condomínio
já construído em Go sobre Temporal, em `../sl-concierge`). Este arquivo registra o que
foi **definido pelo usuário antes de escrever código**, o que será **herdado** do projeto
anterior e o que ainda **precisa ser confirmado**. Nada aqui é implementação: é o
contrato de partida. Data: 2026-10-16.

---

## 1. Definições fixadas (não mudar sem conversar)

### 1.1 Linguagem e harness

- **Go**, projeto novo, módulo próprio (`go-concierge`), sem reaproveitar código do
  `sl-concierge` por cópia cega: as ideias e os formatos migram; o código é reescrito.
- **Harness: Eino** (CloudWeGo, open source, `github.com/cloudwego/eino` +
  `github.com/cloudwego/eino-ext`). O Eino entra com **dois papéis, e só dois**:
  1. **Chamar tools**: function calling do modelo, loop de agente (pensar → chamar tool →
     observar), com as tools do concierge registradas como tools do Eino.
  2. **Ler e interpretar agentes e subagentes especificados em OKF** (ver 1.2): o
     carregador transforma cada especificação OKF num agente do Eino (modelo + instrução +
     tools + subagentes), e o Eino executa.
- O Eino **não** é o dono da orquestração de negócio. Roteamento para caso de uso,
  interpretação do catálogo YAML, regras duras (POLICIES), sessão, pendências, handoff,
  orçamento e auditoria são **código nosso**, chamando o Eino onde há modelo e tool.
- Provedor de modelo pelo `eino-ext` (OpenAI-compatível cobre OpenRouter e Gemini; há
  também Claude e Gemini nativos). Decidir o provedor padrão em 4.3.

### 1.2 OKF: agentes e subagentes no formato de pasta (definido em 2026-10-16, reorganizado no mesmo dia)

**Reorganização aprovada (2026-10-16): dois mundos.** O que é **do agente** fica estático
na pasta e se edita no Admin; o que é **infraestrutura** vem do **Nexus em tempo de
execução** e o agente só declara o que precisa.

| Do agente (pasta) | Do Nexus (tempo de execução) |
|---|---|
| `agent.yaml`: nome, tipo, descrição, modelo, `recursos:` (nomes que declara usar) | **tools**: recursos `api`, `database`, `agent` da account |
| `persona.md`: traços e tom de quem fala com o morador; **só no orquestrador de primeiro nível** (o concierge). Subagentes produzem resultados, não falam com o morador: não têm persona nem a herdam; a voz é aplicada pelo orquestrador na entrega | **conteúdos e memórias**: recursos `knowledge` e `memory`, consultados, nunca copiados |
| `caso.yaml`: caso de uso determinístico (passos do catálogo); **única porta de escrita** | a lista vem de `GET /v1/accounts/{account}/resources` ou do instantâneo `okf/nexus/<account>.yaml` |
| `rules/`, `workflows/`, `routers/`, `knowledge/` (OKF), `assets/`, `evals/`, `agentes/` | o harness cruza a declaração do agente, o que a account oferece e o que a identidade do morador pode (privado só autenticado; escrita só pelo caso; bloqueios do condomínio) |

Saíram da pasta e da interface: `skills/`, `schemas/`, `mcps/`, `bash/`, `scripts/`,
`rag/`, `hooks/`, `data/` solto, os catálogos `bases/`, `tools/`, `apis/` e os campos
`fontes`, `apis`, `segredos` do `agent.yaml`. A interface ganhou **Recursos do Nexus**
(lista da account com interruptor "este agente usa"), **Persona** e **Caso de uso** na
ficha. O texto abaixo descreve a anatomia como ficou.

- Todo agente e subagente do concierge é uma **pasta** ("formato pastinha"), no mesmo
  desenho do AgentAsFolder do harness Python `adam` (`../adam`, o "projeto Python"), com
  o **conhecimento OKF** dentro dela. Nada de agente em código Go.
- **Anatomia da pasta** (`okf/agentes/<nome>/`): na raiz `agent.yaml` (nome, tipo
  `orquestrador | operador`, descricao, modelo, `recursos`), `persona.md` e `caso.yaml`;
  caixinhas `rules/` (output.md = contrato de saída; o resto é política), `workflows/`
  (principal.md + procedimentos), `routers/` (rotas.md + desambiguacao.md),
  `knowledge/`, `assets/`, `evals/` e `agentes/<sub>/` (subagentes, recursivo).
- **Conhecimento OKF** em `knowledge/`: `glossary.yaml` (termos, sinônimos, definição,
  labels; UMA definição, três consumidores: roteador, projeção, intenção) e documentos
  Markdown com frontmatter tipado `type: Domain | Vocabulary | Question | Condition |
  Policy | Procedure`, `id`, `title`, `version`, `status`, `description`, `tags`; um
  `Question` declara `answers:` (o espaço de respostas), `## Method` e `## Tie-breaking`.
  Formato vindo dos agentes do stattus4 no adam e dos bundles do `stt4-one`.
- **Caso de uso** (`caso.yaml`): o formato do catálogo do sl-concierge (`id`, `name`,
  `visibility`, `params`, `steps` com `resource`, `choose`, `collect`, `form`,
  `confirm`, `compose`, `output`). O harness executa os passos; a escrita só acontece
  por um passo `resource` do caso, de preferência depois de um `confirm`.
- **Vínculos**: assets e conhecimento citados em crase nos textos (`assets/rodape.md`,
  `knowledge/glossary.yaml`), rotas (`→ rota \`nome\``) apontando para subagentes,
  recursos declarados (agent.yaml e passos do caso) conferidos contra o Nexus (existe na
  account? escrita fora do caso?), persona em vigor. Todos conferidos a cada leitura.
- **Duas regras duras** (do adam): **herança** (knowledge e assets do pai valem para os
  filhos, o mais próximo vence; persona, rules, workflows, routers, evals e caso não
  descem) e **fronteira** (um agente só referencia a própria pasta ou um ancestral;
  irmão, sobrinho e fora da árvore são proibidos).
- O código Go tem o **interpretador** (pacote `internal/okf`: carga, herança, fronteira,
  vínculos, conhecimento, projeção, rotas, escrita, catálogo) e a **interface de
  cadastro** (seção 7). O harness Eino receberá a **projeção** da pasta como instrução
  do agente e as caixinhas de capacidade como tools.
- Agentes já cadastrados em `okf/`: **concierge** (orquestrador) com os subagentes
  **observador-sessao** (1.6), **regimento** e **faq**.

### 1.3 Canal: emulação do ATENDE (contrato real recebido em 2026-10-17)

**O que o usuário instruiu.** O ATENDE manda um **evento por webhook** a cada mensagem
do morador (CloudEvents, `com.echo.atende.user.protocol.created`), com `tenant`,
`data.protocol.number` (o protocolo da conversa), `data.contact.{phone,name}`,
`data.channel` (WhatsApp, waba) e `data.message.content`. A identidade do morador vem
do **Gruvi**: `POST GRUVI_TOKEN_URL` com `Authorization: Basic <client>` e
`grant_type=client_credentials` → `access_token`; depois
`GET GRUVI_BFF_URL/building-operation/companies/{GRUVI_COMPANY_ID}?filter[mobile-number]={phone}`
com o Bearer → condomínio e unidades (`id_fastpass`, bloco, morador). O telefone do
contato do evento é o que vai no filtro.

**Implementado:** `internal/gruvi` (token com cache + identidade por telefone),
`internal/atende` (evento, sessão por protocolo com histórico, serviço que identifica,
roda o agente de primeiro nível pelo harness e responde pelo `Sender`), webhook
`POST /webhook/atende` (202 e processamento em segundo plano; `ATENDE_WEBHOOK_TOKEN`
opcional) e o **Console** (`/console`): o emulador forja o evento exatamente nesse
formato, manda ao webhook e mostra a conversa, a identidade, a rota e a caixa de saída.
Testado com o Gruvi de staging (telefone de exemplo → Vanderlei, unidade Jataí) e
Gemini.

**Em aberto:** como o concierge **responde** ao ATENDE (o endpoint real de envio ao
protocolo). Hoje a resposta vai por HTTP a `ATENDE_REPLY_URL` como
`POST {protocol, message}` (premissa); sem essa variável, ao **ATENDE emulado** do
próprio servidor (`/emulador/atende/messages`), que guarda o recebido e o Console marca
no balão com ✓✓. Também: eventos de
mensagens seguintes no mesmo protocolo (nome do evento), encerramento e transferência
ao atendente humano no contrato do ATENDE.

Texto anterior (referência D13, canal síncrono), mantido como histórico:

- O projeto **emula a chamada ao sistema ATENDE** da Superlógica. Os detalhes do
  emulador serão passados pelo usuário **em instruções futuras**; não inventar o
  comportamento do ATENDE além do que já é conhecido.
- O que já é conhecido (D13 do `sl-concierge`, `docs/ATENDE.md`), e vale como ponto de
  partida do contrato:
  - canal **síncrono**: uma requisição por mensagem do morador, resposta no corpo;
  - `POST /message` com `{tenantId, threadId, message, phone?, name?}`;
  - resposta `{aiMessage, exit, transferredToAttendant}`;
  - `tenantId` = condomínio (account); `threadId` = protocolo da conversa; `phone`
    identifica o morador (AUTH); sem `phone`, conversa pública;
  - `exit` = a sessão foi encerrada; `transferredToAttendant` = foi para humano;
  - `GET /health`, autenticação por Bearer.
- Como o canal é síncrono, **cada requisição do ATENDE é uma "vez do robô"**: o
  concierge processa até precisar do morador (pergunta), concluir ou transferir, e
  devolve tudo o que disse nessa vez. Não há Evolution/WhatsApp nesta fase (4.4).
- O emulador do ATENDE é uma peça do projeto (serviço ou módulo de teste) que faz o
  papel do ATENDE: abre protocolos, envia mensagens do morador, recebe `aiMessage`,
  reage a `exit` e a `transferredToAttendant`. Sua forma exata vem das instruções
  futuras.

### 1.4 Módulo Admin da administradora (redefinido em 2026-10-17)

**Redefinição do usuário.** O condômino é o usuário final; ele fala com o próprio
condomínio ou com a administradora dele (nesse caso, a unidade e o condomínio vêm da
identificação pelo telefone). Há **um só concierge** (o orquestrador). Um **caso de uso**
é uma jornada completa numa trilha de atendimento (liberar visitante, reservar área
comum…), acessada pelo orquestrador; o caso de uso **define o agente** (workflows,
skills, assets, conhecimento) e pode ter subagentes. Cada condomínio injeta **contexto
dinâmico** nas skills e políticas desses agentes, mapeado pelos **recursos** do Nexus,
cada um com a assinatura do seu MCP.

Quem cria casos de uso, tools, MCPs e recursos do Nexus é a Superlógica: isso fica na
tela **Backdoor** (a antiga tela de skills). A tela **Admin** da administradora é só o
mapeamento, como um banco com RLS:

| Eixo | O que define | Regra sem mapeamento |
|---|---|---|
| condomínio × caso de uso | habilitado / desabilitado e papéis que podem usar | comum a todos (habilitado) |
| condomínio × recurso R | **payload** de definições (parâmetros padrão e regras locais) | o payload "Padrão (todos)" |

Em tempo de execução, o condomínio da conversa é resolvido pela company do Gruvi, pelo
tenant do ATENDE ou pela account; os casos desabilitados somem do roteador do concierge;
o payload de R é **remontado na chamada** (preenche os params que o agente não passou)
e entra no prompt como "Contexto do condomínio". Implementado em `internal/admin`
(JSON em `okf/var/admin/mapeamento.json`), `harness.Config{Casos, Payloads}`,
`internal/atende` e a tela Admin (`/`), com Backdoor em `/backdoor`. **A administradora
não vê recursos do Nexus** (técnico demais): o payload por condomínio fica no Backdoor,
em `/backdoor/payloads`; o Admin tem só Condomínios e a matriz de casos de uso.

**Administradoras (2026-10-17).** O Backdoor também cadastra as **administradoras**
(clientes da Superlógica) e diz a qual delas pertence cada condomínio
(`/backdoor/administradoras`; `admin.Administradora`, `Condominio.administradora_id`).
O Admin é de uma administradora: o servidor recorta o mapeamento pela administradora do
pedido (`?administradora=<id>`, `Mapeamento.Visao`) e ela só vê os seus condomínios e o
mapeamento deles; o padrão "*" é global, nosso. Hoje o seletor no topo do Admin emula
o login; no produto o id vem da sessão/RLS do Postgres. Remover uma administradora
exige antes mover os condomínios dela.

**Transbordo ao atendente humano (2026-10-17).** Qualquer agente da família pode chamar a
tool `transferir_atendente(motivo)`; o canal marca a sessão como transbordada, registra
o pedido na caixa de saída (no ATENDE real vira a chamada que tira o protocolo do bot) e
o concierge cala: as mensagens do morador ficam para o humano, que no Atendente assume,
responde pelo mesmo canal e devolve ao concierge (o histórico guarda o que o humano
disse). O atendente também pode assumir sem o concierge pedir. API:
`POST /api/atende/transbordo {acao: assumir | responder | devolver}`.

**Escrita pelo subagente do caso, depois do sim (2026-10-17).** Enquanto não existe o
executor determinístico do `caso.yaml`, a escrita roda pelo próprio subagente do caso
com uma guarda determinística no harness: `chamar_recurso` só aceita um recurso de
escrita se ele é passo do `caso.yaml` daquele agente, o morador está identificado, os
params obrigatórios estão presentes e a mensagem atual do morador é um sim explícito
(`harness.Confirmado`: começa com sim/ok/pode/confirmo… e não contém negação). Fora
disso o tool devolve erro mandando resumir e pedir "confirma? (sim/não)". Sem Nexus, o
mock devolve `{protocolo: MOCK-…}`.

**Identidade do Gruvi com vários condomínios (2026-10-17).** O BFF devolve `data` como
um condomínio (objeto com `condominium`, `units`, `legal_responsibles`) ou como lista de
condomínios; `gruvi.Identidade` guarda as unidades por condomínio (`Condominios`) e o
papel sobe para `sindico`/`proprietario` quando o telefone está em `legal_responsibles`.
Unidade sem `id_fastpass` usa o `id` do BFF. A identidade entra no prompt de toda troca
("# Morador identificado"): com um só condomínio e uma só unidade o concierge usa a
unidade sem perguntar; com mais de um, pergunta de qual se trata e registra o pedido
naquele condomínio e unidade. `unidades.listar` é resolvido pelo canal a partir da
identidade (`harness.Config.Locais`), sem depender do Nexus.

**Ficha do condomínio (2026-10-17).** Três camadas: o Backdoor é o universo (o
concierge com todos os casos de uso e recursos); o Admin liga e desliga **casos de uso**
por condomínio (a administradora não mexe em recursos); e a ficha de cada condomínio
guarda as particularidades em linguagem de administradora: **Orientações ao concierge**
(texto livre geral, que entra no prompt da família como "Contexto do condomínio", e por
caso de uso, que entra só na instrução daquele subagente: `admin.Orientacao`,
`harness.Config.Orientacoes`), **Ajustes** (formulário gerado das `definicoes` que cada
recurso declara no catálogo do Nexus, gravado no mesmo payload que o Backdoor vê em JSON)
e **Identificação** (ids de integração). Fica em aberto a sobreposição de regras e
conhecimento por condomínio (pasta de overlay no carregador OKF).

**Casos de uso trazidos do sl-concierge (2026-10-17).** Os casos do catálogo YAML
(`catalog/usecases/_default` e `gruvi`) viraram subagentes do concierge, cada um com
`caso.yaml` (o YAML original), `workflows/principal.md` (a ordem narrada dos passos),
`rules/politicas.md` e `evals/`: listar-unidades, reserva-area-comum,
atualizacao-cadastral, agendar-mudanca, comunicar-obra, encomendas (Gruvi) e
codigo-retirada (Gruvi), além de faq, regimento e liberacao-visitante. As rotas estão em
`concierge/routers/rotas.md`; o instantâneo `okf/nexus/cond-001.yaml` ganhou as
descrições semânticas de `catalog/resources.yaml` e os recursos do BFF Gruvi
(`visitantes.motivos`, `encomendas.listar`, `encomendas.codigo`). O executor dos passos
do `caso.yaml` no harness continua em aberto: hoje o subagente segue o workflow narrado.

Texto original (para histórico):

- Existe um **módulo Admin** para a **administradora de condomínios** (não para o
  morador). Ele cadastra:
  1. **Casos de uso**: criar, editar, validar, habilitar e desabilitar os casos de uso
     (o catálogo), inclusive por condomínio;
  2. **Controle de acesso aos condomínios**: quais condomínios a administradora
     gerencia, quais usuários da administradora acessam quais condomínios, e quais
     casos de uso cada condomínio oferece a quais papéis (morador, proprietário,
     síndico, contato sem cadastro).
- Modelo mínimo de dados do Admin:
  - `Administradora` 1—N `Condomínio` (= account/tenant do ATENDE);
  - `UsuárioAdmin` N—N `Condomínio` (quem pode gerenciar o quê);
  - `CasoDeUso` (definição) × `Condomínio` → habilitado/desabilitado + papéis
    permitidos + sobrescritas por condomínio (o `catalog/usecases/<account>/` do
    `sl-concierge` vira registro, não pasta);
  - `Papel` do morador (vem do AUTH/Nexus) usado no filtro do catálogo.
- O Admin herda as telas e funções do Admin do `sl-concierge` (linha 2.9), agora com
  **multi-condomínio e permissão por usuário** como primeira classe.

### 1.5 Usuário principal e roteamento

- O **usuário principal é o condômino**. Ele **conversa espontaneamente** (linguagem
  natural, sem menu obrigatório) com o **orquestrador do concierge**.
- O orquestrador **roteia para o caso de uso equivalente** (Intention Finder + catálogo
  filtrado pelo condomínio e pelo papel do condômino).
- **Na impossibilidade de resolver a demanda, redireciona para um atendente humano**:
  pedido fora do catálogo sem agente que o atenda, confiança baixa repetida, erro
  técnico irrecuperável, orçamento estourado, ou pedido explícito do morador. No ATENDE
  isso é `transferredToAttendant: true`.
- O caminho padrão é determinístico (caso de uso do catálogo); o modelo decide e redige.
  Escrita (liberar, reservar, agendar, atualizar) **só por caso de uso catalogado**.

### 1.6 Sessão: agente observador e segmentador de assuntos

- **Cada sessão é controlada por um agente observador** (especificado em OKF), que
  acompanha a conversa de ponta a ponta e **segmenta os assuntos**.
- **Pendências**: no **início da conversa** o observador identifica as demandas do
  morador e abre uma **pendência por assunto** (ex.: "liberar visitante", "dúvida sobre
  piscina"). Cada pendência tem estado (`aberta` → `em_atendimento` → `resolvida` |
  `transferida` | `abandonada`), o caso de uso ou agente vinculado e o desfecho.
- Durante a sessão o observador: detecta assunto novo (abre pendência), mudança de
  assunto no meio de um fluxo (pausa/troca), pedido já atendido (fecha), e mantém o
  **histórico segmentado por assunto**, não só cronológico.
- **No fim da sessão** todas as pendências se encerram: resolvidas, transferidas ao
  humano ou abandonadas. A sessão só encerra (`exit`) quando não há pendência aberta e o
  observador decide que o morador obteve o que buscava (herda o critério de D12 do
  `sl-concierge`). O fechamento gera o **resumo da sessão** (pendências e desfechos),
  que alimenta auditoria, custo por caso e o protocolo do ATENDE.
- O observador **não executa** casos de uso nem chama recursos de escrita: ele observa,
  segmenta e decide sobre a sessão. É um agente de leitura, com tools de consulta ao
  estado da conversa e ao catálogo.

---

## 2. O que herdar do `sl-concierge` ("xupinhar")

Mapeamento do que existe hoje para o que o `go-concierge` deve ter. A origem está em
`../sl-concierge` (README.md e docs/ARQUITETURA.md são a fonte).

| # | Funcionalidade | Como é hoje (`sl-concierge`) | No `go-concierge` |
|---|---|---|---|
| 2.1 | **Catálogo de casos de uso em YAML** | `catalog/usecases/_default/*.yaml` + `<account>/`; passos `resource`, `choose`, `collect`, `form`, `confirm`, `compose`; `output` em template Go; `visibility`, `fallback`, `keywords`, `examples`, `params` | Mesmo formato de passos e de params (manter compatibilidade para importar os YAML atuais). Armazenado e versionado pelo Admin; habilitação por condomínio em banco |
| 2.2 | **Tipos de campo e validação** | `text`, `textarea`, `number`, `date`, `phone`, `select`, `checkbox`, `list`; `required`, `placeholder`, `help`, `options`; datas em linguagem natural ("sábado", "dia 15") | Igual, no mesmo pacote de formulários (validação única para chat, formulário e API) |
| 2.3 | **Slots e reaproveitamento** | `key` e `reuse` (always, pending, never); dado informado numa conversa preenche outros fluxos; campo devolvido por API preenche parâmetro | Igual, guardado na sessão |
| 2.4 | **Mini-formulários determinísticos** | `form.threshold`; formulário derivado do YAML sem LLM; link assinado para canais sem formulário; prefill do que já foi dito | Igual; no ATENDE sai como link assinado (`/f/<thread>/<id>`) |
| 2.5 | **Recursos com classes** | Nexus: `api`, `database`, `memory`, `knowledge`, `agent`; `write: true`; `metadata.md`; descrição semântica editável (`catalog/resources.yaml`) | Toda chamada a recurso vira **tool do Eino**. Classes e descrição semântica mantidas. Cliente do Nexus + mock + ponte Gruvi como referência de contrato |
| 2.6 | **Intention Finder** | `intent.min_confidence`, alternativas, saudação/desconhecido, fallback FAQ, mudança de assunto (`switch.threshold`, `min_words`), confirmação sim/não e escolha em texto livre pelo decisor | Orquestrador OKF + decisor. A interface `Decider` (heurística, LLM, Open-JEV) continua, com Open-JEV como opção futura (4.5) |
| 2.7 | **Modos de orquestração** | `router`, `react`, `planning` (D7), com `bind_confidence`, checklist/TODO publicado, casos de uso inline | **Router** como padrão. Os modos agênticos passam a ser o **loop de tools do Eino** com agentes OKF; o TODO/checklist vira as **pendências do observador** (1.6). Não reimplementar três máquinas de estado |
| 2.8 | **Conhecimento como contexto** (D10) | `knowledge` consultado antes do raciocínio, `top_k`, escopo público/privado pela identidade, nunca ação | Igual |
| 2.9 | **Admin** | Visão geral, casos de uso (editor YAML com validação e "testar uma frase"), recursos (semântica, testar, bloquear), concierge (persona e frases), orquestrador (config), assistente de fluxos com LLM, custo por caso, auditoria com correção que vira exemplo | Tudo isso, mais o que está em 1.4 (administradora, condomínios, usuários, permissões). Config deixa de ser arquivo no repo: vira registro por condomínio com padrão global |
| 2.10 | **POLICIES** | `catalog/POLICIES.md`: regras `[código]` (escrita só por caso de uso, privacidade, limite de passos, conhecimento é contexto, bloqueados) e `[prompt]` (sem invenção, fora de escopo, tom) | Pacote `policy` em Go com as regras duras; texto do POLICIES em todo prompt; editável no Admin, por condomínio com padrão global |
| 2.11 | **Persona e frases** (D11) | `catalog/PERSONA.md` e `catalog/messages.yaml` (templates com variáveis, padrão de fábrica) | Igual; por condomínio |
| 2.12 | **Contexto dinâmico** | Data e hora no fuso do condomínio em toda chamada ao modelo, na mensagem do usuário para preservar cache | Igual; fuso por condomínio |
| 2.13 | **Atendente humano** (D9) | `takeover`, `message`, `release`; fluxo pausado sem expirar; ao devolver, decide `continue`, `answer` ou `drop` com a sessão do atendente | Igual; no ATENDE o handoff é `transferredToAttendant` e o retorno vem pelas instruções do emulador |
| 2.14 | **Encerramento por decisão** (D12) | Silêncio após a última fala + decisor diz "obteve o que buscava" com confiança mínima; despedida editável; no ATENDE, avaliado na mesma vez | Passa a ser responsabilidade do **observador** (1.6), com a mesma regra |
| 2.15 | **Orçamento e ledger** | `budget.soft_usd` (rebaixa modelo) e `hard_usd` (para e encaminha); ledger por chamada (tarefa, modelo, tokens, custo, latência); custo por caso e por etapa; tabela de preços | Igual; ledger em banco, por condomínio, por sessão e por pendência |
| 2.16 | **Auditoria** | Toda decisão de intenção e de troca de assunto, com confiança, alternativas e rota; marcar certo/errado; correção vira exemplo no YAML | Igual, mais as decisões do observador (abrir/fechar pendência) |
| 2.17 | **Identidade (AUTH)** | Telefone → pessoa, account, papel, unidades; contato sem cadastro vira identidade pública na account padrão | Igual; account vem do `tenantId` do ATENDE |
| 2.18 | **Mídia** | Áudio transcrito, foto descrita (inclusive texto visível), documento anexado; store compartilhado, só referência trafega | Manter o desenho; implementar quando o emulador do ATENDE disser como a mídia chega |
| 2.19 | **Agentes utilitários locais** | `catalog/agents/<nome>/AGENT.md` + `knowledge/*.md`; mesma interface de recurso; ferramentas só de leitura; um agente não chama outro | Viram **agentes OKF** (1.2). Diferença: OKF permite subagentes |
| 2.20 | **Idempotência** | Chave `workflow/run:passo` em toda chamada de escrita | Chave `sessão:pendência:passo`, estável entre retentativas |
| 2.21 | **Console de teste** | Simulador de WhatsApp, personas, bastidores (linha do tempo, decisões, recursos, plano, custo), modo debug com estado cru, destacar conversa, aba do atendente | Console equivalente falando com o **emulador do ATENDE**; bastidores mostram as pendências do observador |
| 2.22 | **Testes** | Unitários (workflows) + smoke de ponta a ponta em Python (`scripts/smoke_test.py`, 66+ checagens) | Unitários em Go; smoke de ponta a ponta contra o emulador (reescrever as mesmas checagens) |
| 2.23 | **Nexus / Gruvi** | Cliente genérico (`invoke`, metadata, identity); mock com personas; ponte para o BFF Gruvi | Mesmo contrato; mock e ponte vêm junto |

Casos de uso a importar de saída: liberação de visitante, listar unidades, reserva de
área comum, atualização cadastral, agendar mudança, comunicar obra, FAQ público, FAQ
privado; e os da account Gruvi (liberação com motivos, encomendas, código de retirada).

---

## 3. Arquitetura proposta (alto nível, para validar)

```
ATENDE (emulado) ──POST /message──▶ api/atende ──▶ Sessão (observador OKF)
                                                   │  pendências por assunto
                                                   ▼
                                              Orquestrador (OKF)
                                   ┌───────────────┼──────────────────┐
                                   ▼               ▼                  ▼
                            Caso de uso YAML   Agente/subagente    Humano
                            (interpretador)    OKF via Eino        (transferredToAttendant)
                                   │               │
                                   └──── tools do Eino ────▶ Nexus / mock / Gruvi
                                                   │
                       policy · ledger · audit · forms · persona · knowledge
```

Pacotes Go previstos (nomes provisórios):

| Pacote | Responsabilidade |
|---|---|
| `cmd/concierge` | API do canal (`/message`, `/health`), API do Admin, console de teste |
| `cmd/atende-emulator` | Emulador do ATENDE (forma a definir pelas instruções futuras) |
| `cmd/nexus-mock`, `cmd/nexus-gruvi` | Herdados: mock com personas e ponte Gruvi |
| `internal/okf` | Parser, validação e carregador de agentes/subagentes OKF → agentes Eino |
| `internal/harness` | Adaptador do Eino: modelo, registro de tools, execução do loop, usage/custo |
| `internal/session` | Sessão, pendências, observador, encerramento, resumo |
| `internal/orchestrator` | Intention Finder, roteamento, handoff humano |
| `internal/usecase` | Interpretador dos passos do YAML (resource/choose/collect/form/confirm/compose) |
| `internal/catalog`, `internal/forms`, `internal/policy`, `internal/persona`, `internal/knowledge` | Herdados do `sl-concierge` em conceito |
| `internal/admin` | Administradoras, condomínios, usuários, permissões, habilitação de casos |
| `internal/nexus`, `internal/decider`, `internal/ledger`, `internal/audit`, `internal/media` | Herdados em conceito |
| `internal/store` | Persistência (ver 4.6) |

Fluxo de uma vez do robô (uma requisição do ATENDE):

1. API recebe `{tenantId, threadId, message}`; resolve identidade (AUTH) e condomínio.
2. Carrega ou abre a sessão; o **observador** lê a mensagem e atualiza as pendências
   (abre, fecha, troca de assunto).
3. Se há fluxo ativo esperando resposta, a mensagem vai para ele; senão o
   **orquestrador** roteia (caso de uso, agente OKF, conhecimento, ou humano).
4. O caso de uso ou o agente roda até **precisar do morador, concluir ou transferir**.
   Toda chamada a recurso passa pelo `policy` e pelo Eino como tool; tudo vai ao ledger.
5. O observador decide `exit` (sem pendência aberta e morador satisfeito) e
   `transferredToAttendant`; a API devolve `{aiMessage, exit, transferredToAttendant}`.

---

## 4. Decisões (aprovadas pelo usuário em 2026-10-16, como recomendado)

Todas as recomendações abaixo valem como decisão. A 4.1 foi fechada no mesmo dia: OKF é o
formato de pasta do AgentAsFolder com o conhecimento tipado dentro (seção 1.2).

| # | Pergunta | Decisão |
|---|---|---|
| 4.1 | **O que é exatamente o OKF?** | **Fechado:** pasta por agente (AgentAsFolder do `adam`) + conhecimento OKF tipado em `data/okf/` (seção 1.2). Subagentes em `agentes/<sub>/`; assets em `assets/` referenciados por caminho |
| 4.2 | **Temporal: fica ou sai?** O `sl-concierge` usa um workflow por conversa + filhos. Com canal síncrono (vez do robô) e sessão em banco, o Temporal custou outbox, sinal de `turn` e espera no gateway | **Sair** nesta fase: sessão persistida em banco, execução síncrona dentro da requisição do ATENDE, com timeout. Reavaliar se surgirem esperas longas (aprovação de terceiros, agendamentos) |
| 4.3 | **Provedor de modelo padrão** | OpenAI-compatível do `eino-ext` apontando para OpenRouter (cobre Gemini, Claude e baratos), com `mock` para desenvolvimento sem chave |
| 4.4 | **Evolution/WhatsApp** | Fora desta fase. A interface de canal fica abstrata para receber a Evolution depois |
| 4.5 | **Open-JEV como decisor** | Manter a interface `Decider` com heurística e LLM; Open-JEV entra quando houver servidor |
| 4.6 | **Persistência** | Postgres (sessões, pendências, Admin, ledger, auditoria); SQLite para desenvolvimento local |
| 4.7 | **Catálogo: arquivo ou banco?** | Banco como fonte, com **importação e exportação em YAML** no mesmo formato de hoje (para migrar os casos atuais e versionar) |
| 4.8 | **Papéis do Admin** | `admin_administradora` (tudo nos condomínios dela), `gestor_condominio` (um ou mais condomínios), `atendente` (assume conversas, só leitura no Admin) |

---

## 5. Fora de escopo desta fase

- Evolution API, QR Code, WhatsApp real.
- WhatsApp Flows ou formulários nativos: o formulário segue como link assinado.
- ~~Reimplementar os três modos (router/ReAct/planning) como no `sl-concierge`.~~ Revisto em 2026-10-28 (ver 1.9): os modos vieram, mas como **estratégia do orquestrador** em cima do mesmo harness (casos de uso inline, plano guardado pela sessão), não como três máquinas de estado.
- Open-JEV hospedado.
- Faturamento por condomínio (o ledger prepara, não cobra).

### 1.7 Persona é só do orquestrador (2026-10-16)

- `persona.md` existe apenas no agente de **primeiro nível** (o concierge). Subagentes
  não têm persona, não a herdam e a interface nem mostra o bloco para eles.
- Consequência para o harness (feita em 2026-10-17): a resposta ao morador é **entregue
  pelo orquestrador**. Depois do desvio de rota, o orquestrador de primeiro nível faz
  uma chamada de redação com a persona, as políticas e os assets (texto, nunca caminho)
  sobre o resultado do subagente, sem JSON nem termos internos, e sem cumprimentar de
  novo quando a conversa já começou. Passo `entrega` no trace. O `observador-sessao`
  nunca é rota: saudação e despedida ficam no `self` do concierge.

### 1.7b O agente é o caso de uso; a ficha mostra o workflow (2026-10-17)

- "Caso de uso" não é um atributo do agente: o agente **é** o caso de uso na essência.
  A ficha do Backdoor não tem mais um card "Caso de uso"; tem o **Workflow**: a ordem
  do acontecimento em `workflows/principal.md` (passos numerados citando tools e
  arquivos, como no AgentAsFolder do adam), os **procedimentos** (demais workflows, com
  etapas `00_/10_`, abertos por `abrir_workflow`) e, quando existe, os passos
  determinísticos de `caso.yaml` como detalhe (o que o harness executa com confirmação
  e escrita).

### 1.8 Memória relacional por contato (2026-10-17)

- Entre o concierge e **cada telefone** que entra em contato existe uma memória comum a
  todos os agentes: fatos duráveis que a pessoa afirmou sobre si ("só tenho um
  apartamento", "pode me chamar de Van", "tenho um cachorro"). Guardados por
  `account + telefone`, com chave estável (`unidades`, `nome_preferido`, `pets`…),
  texto, protocolo de origem e data; a mesma chave substitui o fato anterior.
- **Captura**: a cada troca, um extrator lê a mensagem do morador e a resposta e
  devolve só o que é durável (modelo barato `MODEL_EXTRACT`; sem modelo, regras por
  padrão). Nada pontual, nada dito pelo concierge, nada incerto.
- **Uso**: em toda conversa seguinte do telefone, a memória entra na instrução de toda
  a família como "Memória sobre o morador: use sem perguntar de novo". Um caso de uso
  que precise da unidade, por exemplo, não pergunta se a memória diz que há uma só.
- **Onde**: `internal/memoria` (`Store` em JSON por contato em `okf/var/memorias/`,
  trocável por Postgres ou pelo recurso `memory` do Nexus), extração no harness,
  leitura/gravação no serviço do ATENDE, visualização e esquecimento no Console
  (`GET/DELETE /api/memoria?phone=`).

## 6. Estado e próximos passos

Feito em 2026-10-16: módulo Go `go-concierge`, pacote `internal/okf` com testes, a
interface `okf-admin` (seção 7) e o catálogo de exemplo em `okf/`. Falta: as instruções
do emulador do ATENDE (1.3); depois, na ordem, o harness Eino (tools + projeção dos
agentes OKF), a sessão com pendências, o catálogo de casos de uso YAML e o Admin da
administradora (1.4), que passa a incluir esta interface como a seção "Skills".

## 7. Interface: Admin e Console (okf-admin)

A barra tem só **Admin** (o cadastro de skills abaixo) e **Console** (o emulador do
ATENDE, seção 1.3).

`cmd/okf-admin` serve `web/okf` (embutida) e fala com `internal/okf`. Rodar:
`./build.sh` e `./bin/okf-admin.exe -root ./okf -addr 127.0.0.1:8090 -open`.

- **Árvore de agentes** com arrastar e soltar: mover (Shift = clonar), soltar no título
  `agentes/` leva à raiz; mover, clonar, criar e excluir sincronizam o `routers/rotas.md`
  dos dois pais (a rota nova entra antes do `self`; sem `routers/` nada é inventado).
- **Recursos do Nexus** (lateral): a lista da account por classe (leitura, conhecimento,
  memória, agentes utilitários, escrita), com interruptor "este agente usa" que grava
  `agent.yaml → recursos`; escrita não tem interruptor (entra pelo `caso.yaml`). Fonte:
  Nexus ao vivo (`NEXUS_BASE_URL`) ou instantâneo `okf/nexus/<account>.yaml`.
- **Ficha do agente**: problemas (erro/aviso com solução), identidade (`agent.yaml`),
  **persona** (própria ou herdada, editável), **caso de uso** (passos em linha do tempo,
  com a escrita marcada), **recursos que usa** (declarados, com classe e estado),
  caixinhas com contagem próprio/herdado e criação de arquivo com modelo, **vínculos**
  com estado ou o problema e a dica de fronteira, subagentes, conhecimento OKF (docs por
  tipo, glossário com sinônimos, inclusive herdados) e o índice que o prompt vê.
- **Arquivos** (editor, Ctrl+S), **projeção** (system prompt compilado, princípio ICM) e
  **roteador** (só o que decide a rota).
- **Assistente de novo agente** (ou subagente): onde, nome, tipo, descrição como
  classificador, `persona.md` e `caso.yaml` opcionais, caixinhas iniciais com
  arquivo-modelo, recursos do Nexus que declara. **Importar/exportar zip** da pasta.
- API JSON documentada no README; é a mesma que o Admin da administradora usará.

## Catálogo MCP oficial (2026-10-28)

O catálogo de recursos do Nexus pode vir do servidor MCP oficial (entrega `mcp-catalog-agent`,
em `mcp/`): cada tool `gruvi.bff.<módulo>.<operação>` é um recurso de classe `api`, privado;
o harness executa por `tools/call` (`internal/mcp`). Autenticação com Keycloak: `request_bearer`
(token B2C repassado ao BFF) hoje; `client_credentials` quando o servidor aceitar token de
serviço. Precedência das fontes: MCP, depois Nexus ao vivo, depois o instantâneo. Empacotado
em `docker compose` (serviços `mcp` e `concierge`).

### 1.9 Modos de orquestração ART no harness (2026-10-28)

- Pedido: "traga os mecanismos de ReAct e Planning (Automatic Reasoning and Tool-use)".
- `router` segue o padrão. `planning` e `react` são configurados em `orquestracao:` do
  `agent.yaml` do orquestrador (ou `ORQUESTRACAO_MODO`), e só valem no primeiro nível.
- O que muda em relação ao `sl-concierge`: não há workflows nem três máquinas de
  estado. O plano (`harness.Plano`: itens, pendente, observações) é um valor guardado
  pela sessão do ATENDE enquanto um item espera o morador; `harness.Continuar` retoma.
  Os casos de uso rodam inline, pelo mesmo `run` do subagente; recursos pela mesma
  guarda de `chamar_recurso`. O modelo propõe vínculos; o harness confere nome,
  confiança (`bind_confidence`) e escrita (nunca direta).
- Trace: passos `modo`, `plano`, `item`; o Console mostra o TODO/checklist na aba Rota.
- Controle: guia **Orquestração** do Backdoor (`/backdoor/orquestracao`, API `/api/admin/orquestracao`),
  que grava `orquestracao:` no `agent.yaml` do concierge; o ambiente (`ORQUESTRACAO_MODO`) só força.

### 1.10 Token B2C do MCP obtido e renovado pelo concierge (2026-10-28)

- O servidor MCP da entrega exige em cada requisição o token B2C de morador (Keycloak
  `persons`), que dura 5 minutos, e o pinava na subida. Pedido do usuário: o fluxo SMS de
  dev do Spaces (`/auth/sms` com `validation=dev` → `session_id`; `/auth/token` com
  `grant_type=password`, `sms_code=999999`) para obter o token.
- Decisão: `MCP_AUTH=b2c` no concierge (`internal/mcp/b2c.go`): obtém pelo SMS, renova pelo
  `refresh_token` (60 dias) 45 s antes de vencer e grava o token corrente em `okf/var/b2c.token`.
  O servidor MCP ganhou `B2C_TOKEN_FILE` (uma função a mais, opt-in) e lê esse arquivo a cada
  verificação. Nenhum token colado em `.env`. A renovação é sob demanda (na próxima chamada
  ao MCP), não por timer.
- O concierge repassa `/mcp/...` ao servidor (porta 8090) sem injetar token: quem testa de
  fora usa o token corrente do arquivo.

### 1.11 Pedidos: registro de trabalho por contato (2026-10-28)

- Problema: no planning, o status "feito" era inferido do texto da resposta do caso de uso;
  casos que só tinham perguntado apareciam concluídos. Pedido do usuário: um mecanismo mais
  seguro, com pedidos acumulados entre sessões, protocolo por pedido, execução só com os
  parâmetros obrigatórios respondidos, e pendências levadas ao atendente com resumo.
- Decisões (confirmadas pelo usuário): execução **determinística pelos passos do caso.yaml**
  (não pelo subagente conversacional) e os Pedidos como **espinha dorsal** da orquestração
  (planning/react deixam de mandar; o roteador atende só o "restante").
- Estado em código: `internal/pedidos` (modelo, contrato do caso.yaml, motor, extrator
  Regras, resumo); extração por LLM em `harness/pedidos.go`; pipeline em
  `atende/pedidos.go`; guia Pedidos e formulário do atendente no Console.
