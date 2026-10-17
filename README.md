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
concierge identifica o telefone no **Gruvi** (token client-credentials + BFF por
`filter[mobile-number]`), roda o agente de primeiro nível e responde ao protocolo. O
Console forja esse evento, manda a `POST /webhook/atende` e mostra a conversa no
telefone, a identidade (unidades do Gruvi), a rota com tokens e a caixa de saída.
Configuração no `.env`: `GRUVI_CLIENT_BASIC`, `GRUVI_COMPANY_ID` (e as URLs de staging
por padrão), `ATENDE_WEBHOOK_TOKEN`, `ATENDE_REPLY_URL` (envio ao ATENDE, contrato a
confirmar).

## A interface (okf-admin)

```bash
./build.sh            # gofmt + vet + test + bin/okf-admin.exe e bin/okf-admin (no contêiner, sem Go local)
./bin/okf-admin.exe -root ./okf -addr 127.0.0.1:8090 -open
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
quando o `agent.yaml` não diz. Sem chave, roda no modo mock.

## Estrutura

```
cmd/okf-admin      servidor HTTP da interface (SPA embutida)
cmd/okf-run        roda um agente pela linha de comando (harness Eino)
internal/okf       pasta de agente: carga, herança, fronteira, vínculos, conhecimento OKF, projeção, rotas, escrita, catálogo
internal/harness   adaptador Eino: modelo (Gemini | OpenRouter | mock), tools nativas, rota, trace e uso
web/okf            a interface (HTML, CSS, JS, sem dependências)
okf/               o catálogo de exemplo: concierge (orquestrador) + observador-sessao, regimento, faq
```

Próximas peças, na ordem das definições: harness Eino (tools + agentes OKF), emulador do
ATENDE, sessão com pendências, catálogo de casos de uso YAML, Admin da administradora.
