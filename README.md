# go-concierge

Concierge de condomínio em Go, do zero, com as ideias do `sl-concierge`. As definições de
partida (harness Eino, OKF, emulação do ATENDE, Admin da administradora, observador de
sessão) estão em [DEFINICOES.md](DEFINICOES.md).

O que existe hoje é a primeira peça: o **cadastro de skills no formato de pasta** (OKF),
com a interface de administração.

## OKF: agentes em pasta

Um agente não é código: é uma pasta em `okf/agentes/<nome>/`. A raiz tem só o
`agent.yaml`; o resto são caixinhas, e `agentes/<sub>/` guarda os subagentes com a mesma
anatomia, recursivamente.

```
okf/
├── agentes/<agente>/
│   ├── agent.yaml        nome, tipo (orquestrador | operador), descricao, modelo, fontes, apis, segredos
│   ├── rules/            output.md = contrato de saída; os outros .md são políticas
│   ├── workflows/        principal.md = passo a passo; os demais são procedimentos
│   ├── routers/          rotas.md (critério → rota `nome`) e desambiguacao.md
│   ├── evals/*.yaml      casos que provam o comportamento
│   ├── hooks/*.yaml      automações que o runtime dispara
│   ├── skills/ schemas/ mcps/ bash/ scripts/   capacidades (herdam para os filhos)
│   ├── rag/**            documentos para busca por semelhança (não herda)
│   ├── data/**           dados indexados, lidos sob demanda (herda); data/okf/ = conhecimento OKF
│   ├── assets/**         estáticos (templates, rodapés), referenciados como `assets/<arquivo>`
│   └── agentes/<sub>/    subagentes
├── bases/<nome>/         conector completo (base.yaml + mcps/ schemas/ skills/ scripts/ bash/ data/)
├── tools/{scripts,mcps,assets,skills,schemas}/   recursos avulsos
└── apis/*.yaml           registro de APIs (vinculadas por referência em agent.yaml → apis)
```

**Conhecimento OKF**: dentro de `data/okf/` (ou `okf/` na raiz da pasta) ficam o
`glossary.yaml` (termos, sinônimos, definição) e documentos Markdown com frontmatter
tipado: `type: Domain | Vocabulary | Question | Condition | Policy | Procedure`. Um
`Question` declara `answers:` (o espaço de respostas), `## Method` e `## Tie-breaking`.

Duas regras duras, herdadas do harness Python `adam`:

- **Herança**: skills, schemas, mcps, bash, scripts, data e assets do pai valem para os
  filhos (o mais próximo vence). Rules, workflows, routers, hooks, evals e rag são de cada um.
- **Fronteira**: um agente só referencia a própria pasta ou os recursos próprios de um
  ancestral (`fontes: [../../data]`). Irmão, sobrinho e fora da árvore são proibidos.

## A interface (okf-admin)

```bash
./build.sh            # gofmt + vet + test + bin/okf-admin.exe e bin/okf-admin (no contêiner, sem Go local)
./bin/okf-admin.exe -root ./okf -addr 127.0.0.1:8090 -open
```

Três colunas:

- **Esquerda**: a árvore de agentes (arraste para mover, Shift+arraste para clonar; solte
  no título `agentes/` para levar à raiz) e o **catálogo** (bases, apis, assets, scripts,
  mcps, skills, schemas), cujos cartões se arrastam sobre um agente da árvore. Mover ou
  clonar acerta o `routers/rotas.md` dos dois pais.
- **Centro**: a ficha do agente: problemas encontrados, identidade (`agent.yaml`
  editável), as caixinhas com contagem próprio/herdado (clique no `+` para criar um arquivo
  com modelo), os **vínculos internos** (assets, dados, fontes, rotas, subagentes, apis,
  hooks) com o estado de cada um e a dica de correção, os subagentes, o conhecimento OKF
  (docs tipados e glossário) e o índice de dados que o prompt enxerga.
- **Direita**: os arquivos (editor, Ctrl+S salva), a **projeção** (o system prompt
  compilado da pasta, no princípio ICM) e o **roteador** (só o que decide a rota).

`+ novo agente` abre o assistente: onde (raiz ou dentro de um agente), nome, tipo,
descrição (o critério que o roteador do pai lê), caixinhas iniciais com arquivo-modelo,
fonte do pai, apis e segredos. `importar zip` instala uma pasta inteira.

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
| GET | `/api/catalogo` | bases, tools e apis |
| POST | `/api/recursos/aplicar` | `{tipo, nome, agente}`: copia para a caixinha e funde segredos/apis (idempotente) |

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
