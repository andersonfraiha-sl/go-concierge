<div align="center">

# go-concierge

**Concierge de condomínio no WhatsApp, em Go: o morador fala pelo ATENDE, o concierge responde com regra em código e auditoria.**

[![Go](https://img.shields.io/badge/Go-1.27-00ADD8?logo=go&logoColor=white)](go.mod)
[![Postgres](https://img.shields.io/badge/Postgres-18-4169E1?logo=postgresql&logoColor=white)](docker-compose.yml)
[![React](https://img.shields.io/badge/React-19-61DAFB?logo=react&logoColor=black)](web/studio/package.json)
[![pnpm](https://img.shields.io/badge/pnpm-12.16.1-F69220?logo=pnpm&logoColor=white)](web/studio/package.json)
[![Node](https://img.shields.io/badge/Node-26.13-339933?logo=node.js&logoColor=white)](web/studio/.nvmrc)
[![CI](https://github.com/andersonfraiha-sl/go-concierge/actions/workflows/ci.yml/badge.svg)](https://github.com/andersonfraiha-sl/go-concierge/actions/workflows/ci.yml)

**0 idas ao banco antes da resposta (contato quente)** &bull; **0,13 ms de ganchos por turno** &bull; **6 pontos de gancho** &bull; **601 testes Go + 417 Vitest**

</div>

---

## tltr

- Webhook do ATENDE entra, o turno roda em memória, a resposta sai, e só depois sessão e auditoria vão ao Postgres.
- Regra de segurança é gancho em Go, não frase no prompt. CPF e telefone saem mascarados antes de qualquer modelo ou log.
- Studio (React) edita skills, condomínios e regras, com RBAC por papel e escopo.
- Rodar: `cp .env.example .env && ./build.sh --run`, abre em `http://127.0.0.1:8090/studio/`.

## Sumário

[Problema e solução](#problema-e-solução) · [Arquitetura](#arquitetura) · [Um turno](#um-turno) · [Rodar local](#rodar-local) · [Testes e CI](#testes-e-ci) · [Estrutura de pastas](#estrutura-de-pastas) · [Acesso e papéis](#acesso-e-papéis) · [Ganchos](#ganchos) · [Logs e erros](#logs-e-erros) · [Endpoints](#endpoints) · [Legado](#legado) · [Docs](#docs)

## Problema e solução

**Problema.** Regra que protege morador e condomínio vivia espalhada: parte no prompt, parte em `if` solto. CPF ia inteiro pro modelo. Cada turno fazia 18 idas ao banco antes de responder.

**Insight.** Todo caminho que responde ou chama ferramenta atravessa as mesmas fronteiras: entrada, ferramenta, resposta. Regra mora nessas fronteiras, não em cada camada.

**Solução.** Um ponto único por fronteira (ganchos), um choke point de ferramenta (`invocar`), estado do contato em cache write-through, escrita adiada pra depois do envio.

| | Antes | go-concierge |
|---|---|---|
| **Regra "não informe dado a número não cadastrado"** | frase no prompt | gancho fixo em Go, não desliga |
| **CPF e telefone** | iam inteiros pro LLM e pro histórico | mascarados em `mensagem_recebida`, no log e na auditoria |
| **Regra por condomínio** ("reserva até 22h") | sem lugar | regra declarativa, editada no Studio, cache aquecido no boot |
| **Escrita no mundo** | qualquer tool | só por passo do `caso.yaml`, depois do `confirm` |
| **Idas ao banco antes da resposta** | 18 | 0 quente, 1 a frio |
| **Decisão de bloqueio** | some no log | linha em `ganchos_auditoria`, visível no Studio |

## Arquitetura

```mermaid
flowchart TB
    Morador(["Morador<br/>WhatsApp"]):::entry
    ATENDE["ATENDE<br/>protocolo e canal"]:::entry
    Studio(["Studio<br/>React 19 · web/studio"]):::entry

    subgraph GC ["go-concierge · cmd/okf-admin"]
        direction TB
        subgraph Borda ["Borda HTTP"]
            Webhook["POST /webhook/atende<br/>HMAC X-Atende-Signature"]:::entry
            API["/api · RBAC<br/>internal/acesso"]:::entry
        end
        subgraph Turno ["Turno · internal/atende"]
            direction TB
            GIn["ganchos<br/>mensagem_recebida"]:::hook
            Motor["motor do caso<br/>internal/pedidos"]:::core
            Harness["harness LLM<br/>internal/harness"]:::core
            Invocar["invocar<br/>antes e depois_da_ferramenta"]:::hook
            GOut["ganchos<br/>antes_de_responder"]:::hook
            GIn --> Motor
            GIn --> Harness
            Motor --> Invocar
            Harness --> Invocar
            Invocar --> GOut
        end
        subgraph Estado ["Estado em processo"]
            Caches["memória · pedidos · sessões<br/>mapeamento (write-through)"]:::store
            Regras["cache de regras<br/>internal/ganchos/regras"]:::store
        end
        Audit["auditoria<br/>internal/ganchos/auditoria"]:::core
        Copiloto["Copiloto<br/>internal/copiloto"]:::core
    end

    subgraph Externos ["Sistemas externos"]
        Gruvi["Gruvi BFF<br/>internal/gruvi"]:::ext
        Nexus["Nexus · MCP Gruvi BFF<br/>internal/nexus · internal/mcp"]:::ext
        LLM["LLM<br/>Gemini · OpenRouter · Luna"]:::ext
    end
    PG[("Postgres 18")]:::store

    Morador --> ATENDE
    ATENDE -- "webhook" --> Webhook
    Webhook --> GIn
    GOut -- "Sender.Enviar" --> ATENDE
    GOut -- "depois do envio" --> Audit
    Caches --> Turno
    Regras --> GIn
    Studio --> API
    API --> Copiloto
    Copiloto -- "copiloto_antes_de_aceitar" --> Regras

    Turno -. "identidade por telefone" .-> Gruvi
    Invocar -. "tools/call" .-> Nexus
    Harness -. "prompt e tools" .-> LLM
    Copiloto -. "propostas de edição" .-> LLM

    Caches -- "1 lote a frio" --> PG
    Audit -- "lote com sessão e carteira" --> PG
    API --> PG

    classDef entry fill:#111827,stroke:#f59e0b,stroke-width:3px,color:#f9fafb
    classDef core fill:#1f2937,stroke:#4b5563,color:#f9fafb
    classDef hook fill:#312e81,stroke:#818cf8,color:#eef2ff
    classDef store fill:#0f172a,stroke:#38bdf8,color:#e0f2fe
    classDef ext fill:#1e293b,stroke:#64748b,stroke-dasharray:4 3,color:#e2e8f0
    style GC fill:#0b1220,stroke:#334155,color:#e2e8f0
    style Turno fill:#151b2e,stroke:#6366f1,color:#e2e8f0
    style Borda fill:#151b2e,stroke:#f59e0b,color:#e2e8f0
    style Estado fill:#151b2e,stroke:#38bdf8,color:#e2e8f0
    style Externos fill:#0b1220,stroke:#64748b,color:#e2e8f0
```

| Peça | Onde | Papel |
|---|---|---|
| Turno | `internal/atende` | abre sessão, roda ganchos, escolhe motor ou harness, envia, grava depois |
| Motor do caso | `internal/pedidos` | passos do `caso.yaml`: coleta, valida, confirma, executa. Estado decidido em código |
| Harness | `internal/harness` | agente Eino da pasta OKF: router, planning ou react |
| `invocar` | `internal/harness/tools.go` | choke point de toda ferramenta: motor, planejador e `chamar_recurso` do LLM |
| Copiloto | `internal/copiloto` | propostas do modelo no Studio, filtradas por tela, permissão e ganchos |

## Um turno

Medido em [`medicoes/2026-11-03-turno.md`](docs/superpowers/medicoes/2026-11-03-turno.md) e [`medicoes/2026-11-03-ganchos.md`](docs/superpowers/medicoes/2026-11-03-ganchos.md): contato quente sem mudança de pedido faz **0 idas** ao Postgres antes do `Sender.Enviar`, também com 20 regras ativas e um bloqueio.

```mermaid
sequenceDiagram
    autonumber
    participant M as Morador
    participant A as ATENDE
    participant W as okf-admin
    participant T as Turno
    participant C as Caches em processo
    participant X as LLM e ferramentas
    participant P as Postgres

    M->>A: mensagem no WhatsApp
    A->>W: POST /webhook/atende com HMAC
    W-->>A: 202, processa em segundo plano
    W->>T: Processar
    T->>C: sessão, memória, pedidos, mapeamento, regras
    Note over T,P: contato quente, 0 idas ao banco até o envio
    T->>T: ganchos mensagem_recebida, mascara CPF e telefone
    T->>X: motor do caso ou harness, ferramentas via invocar
    T->>T: ganchos antes_de_responder
    T->>A: Sender.Enviar
    A->>M: resposta
    T->>P: 1 lote com sessão, carteira adiada e auditoria
    T--)P: extração de memória em segundo plano
```

| Cenário | Idas antes do envio |
|---|---|
| Contato quente, sem mudança de pedido | **0** |
| Contato frio (1 `pgx.Batch` com sessão, memória e pedidos) | 1 |
| Pedido aberto ou que muda de estado (durabilidade antes de citar o protocolo) | +1 |

<details>
<summary><strong>Limite conhecido: uma réplica</strong></summary>

Caches e locks por contato são em processo. Com duas réplicas nada corrompe o banco, mas uma não vê o que a outra escreveu:

- trocas simultâneas do mesmo telefone em réplicas diferentes podem perder um fato ou um pedido;
- o cache de sessões (`Sessoes`) e os de memória e pedidos não revalidam contra o banco;
- o cache do mapeamento só vê escrita do Admin desta réplica; `importar-json` com o servidor no ar só aparece depois de reiniciar;
- a sessão e as mudanças menores da carteira (valores, eventos) vão depois da resposta: se o processo cair nesse intervalo, elas se perdem.

Pra escalar: invalidação entre réplicas (LISTEN/NOTIFY ou versão com prazo curto) e locks por contato no banco.

</details>

## Rodar local

Pré-requisitos: Docker e Git Bash. Go e Node rodam em contêiner (`golang:1.27-trixie`, `node:26.13-trixie-slim`).

```bash
cp .env.example .env                   # AUTH_MODE=dev e DATABASE_URL já vêm prontos
docker compose up -d --wait postgres   # Postgres 18 em 127.0.0.1:5432
./build.sh --run                       # Studio compilado + okf-admin.exe em 127.0.0.1:8090
```

Abre em `http://127.0.0.1:8090/studio/`. Sem `DATABASE_URL` no ambiente nem no `.env`, o `--run` usa o Postgres do compose. Migrations rodam no boot com `AMBIENTE` vazio ou `dev` (`-migrar=false` desliga).

**Studio em dev** (hot reload, `/api` via proxy pra 8090):

```bash
npm install -g corepack@latest   # uma vez: o Node 26 não traz o corepack
corepack enable
cd web/studio
corepack pnpm install
corepack pnpm run dev            # http://localhost:5173/studio/
```

<details>
<summary><strong>Binário à mão, stack inteira em Docker e importação</strong></summary>

```bash
# binário à mão
DATABASE_URL='postgres://concierge:concierge@127.0.0.1:5432/concierge?sslmode=disable' \
  ./bin/okf-admin.exe -root ./okf -addr 127.0.0.1:8090 -open

# stack inteira: concierge (8090) + postgres + mcp na rede do concierge
docker compose up -d --build

# fora de dev o boot não semeia nada: leve os JSON da versão anterior antes do primeiro boot
./bin/okf-admin.exe importar-json -dir okf/var -env .env
```

Em dev, um banco sem mapeamento ganha uma administradora e um condomínio de exemplo (ligado ao `DEFAULT_ACCOUNT_ID`). Sem `DATABASE_URL` o boot falha dizendo como subir o Postgres. `/studio/` sem compilar responde 503 `STUDIO_NAO_COMPILADO`.

Em Linux, com `okf/` de outro usuário, libere o usuário 10001 do contêiner:

```bash
mkdir -p okf/var
setfacl -R -m u:10001:rwX okf
find okf -type d -exec setfacl -m d:u:10001:rwx {} +
```

</details>

<details>
<summary><strong>Env (.env.example)</strong></summary>

| Grupo | Variáveis |
|---|---|
| Banco | `DATABASE_URL` (obrigatória) |
| Acesso | `AUTH_MODE`, `AUTH_DEV_USUARIO`, `ADMINISTRADORA_ID`, `AMBIENTE` |
| Modelo | `LLM_MODE` (`gemini` \| `openrouter` \| `luna` \| `mock`), `GEMINI_API_KEY`, `OPENROUTER_API_KEY`, `LLM_LUNA_OPENAI_URL`, `LLM_LUNA_GATEWAY_TOKEN`, `LLM_LUNA_MODEL`, `MODEL_EXTRACT`, `MODEL_PLANNER`, `MODEL_COMPOSE`, `MODEL_CHEAP` |
| Gruvi | `GRUVI_BFF_URL`, `GRUVI_COMPANY_ID` |
| Ganchos | `GANCHOS_SEGREDO_EXPORTACAO` (HMAC da exportação; fora de dev é obrigatório) |
| Integração | `INTEGRACAO_CHAVE` (chave de serviço das rotas `/integracao/v1`; vazia desliga a rota) |
| ATENDE | `ATENDE_API_BASE_URL`, `ATENDE_API_KEY`, `ATENDE_API_SECRET`, `ATENDE_TENANT_ID`, `ATENDE_TENANT_TELEFONE`, `ATENDE_AGENTE`; legado: `ATENDE_WEBHOOK_TOKEN` (só emulador), `ATENDE_REPLY_URL`, `ATENDE_REPLY_TOKEN` |
| Recursos | `NEXUS_BASE_URL`, `NEXUS_TOKEN`, `DEFAULT_ACCOUNT_ID`, `MCP_URL`, `MCP_AUTH` (`b2c` \| `file` \| `bearer` \| `keycloak` \| `none`), `MCP_B2C_MOBILE`, `MCP_TOKEN_FILE` |
| Turno | `PEDIDOS=off` desliga o motor de pedidos, `ORQUESTRACAO_MODO` força `router` \| `planning` \| `react`, `DESLIGAMENTO_PRAZO` (padrão 30s) |

Regras de boot: o webhook real confere sempre o HMAC com `ATENDE_API_SECRET`, fora de `dev` o boot falha sem ele. Enquanto o token B2C é de um telefone só, atendimento e identidade aceitam só o `MCP_B2C_MOBILE` (outro número: 403). Com `AMBIENTE=dev` o webhook real troca o telefone de todo evento por esse, com WARN no boot.

</details>

## Testes e CI

```bash
./build.sh --test   # Studio (lint, typecheck, vitest) + gofmt, vet, go test -p 1, govulncheck
./build.sh          # tudo acima + build do Studio + binários okf-admin e okf-run (Windows e Linux)
```

`-p 1` porque `internal/banco` sobe Postgres por testcontainers: Docker precisa estar de pé.

| Checagem | Local | CI (`ci.yml`) |
|---|---|---|
| gofmt | `gofmt -l .` sem saída | sim |
| vet | `go vet ./...` | sim |
| Testes Go | `go test -p 1 -race ./...` | sim, com `-race` |
| golangci-lint v2.15.0 | `docker run --rm -v "$PWD:/src" -w /src golangci/golangci-lint:v2.15.0 golangci-lint run ./...` | sim |
| govulncheck v1.9.0 | `./build.sh --test` | não |
| Biome | `corepack pnpm run lint` | sim |
| Typecheck | `corepack pnpm run typecheck` | sim |
| Vitest | `corepack pnpm test` | sim |
| Build do Studio | `corepack pnpm run build` | sim |
| gitleaks v8.30.1 | | sim |

Comandos do Studio rodam em `web/studio`. O pre-commit (husky) aplica o Biome nos staged e roda a suíte quando o commit toca `web/studio`.

## Estrutura de pastas

| Pasta | O que tem |
|---|---|
| `cmd/okf-admin` | servidor HTTP: webhook, `/api`, Studio embutido, telas antigas, subcomandos `importar-json` e `saude` |
| `cmd/okf-run` | roda um agente pela linha de comando (harness Eino) |
| `internal/atende` | turno, sessão, envio ao ATENDE, emulador |
| `internal/pedidos` | motor do caso, carteira por contato, cache |
| `internal/harness` | agente Eino, tools nativas, `invocar`, modos de orquestração, Copiloto |
| `internal/ganchos` | eventos, fixos, regras, PII, auditoria |
| `internal/acesso` | papéis, permissões, escopos |
| `internal/memoria` | fatos por contato, cache |
| `internal/admin` | mapeamento de administradoras e condomínios, cache |
| `internal/okf` | pasta de agente: carga, herança, fronteira, projeção, conhecimento OKF |
| `internal/banco` | migrations, sqlc, ent, `Lote` (vários `:batch*` numa ida) |
| `internal/falha` | pilha de erro onde nasce |
| `internal/gruvi`, `nexus`, `mcp`, `responses` | clientes externos |
| `internal/copiloto` | quais propostas a pessoa pode aplicar na tela, sem side effect |
| `web/studio` | Studio em React + MUI, compilado em `dist` e embutido no binário |
| `web/okf` | Console e telas antigas, HTML e JS sem build |
| `okf/` | catálogo de exemplo: agentes, instantâneo do Nexus, `var/` |
| `mcp/` | servidor MCP do catálogo Gruvi, Node (legado) |
| `docs/superpowers` | specs, planos, medições, auditorias |

<details>
<summary><strong>Anatomia de um agente OKF</strong></summary>

```
okf/
├── agentes/<agente>/
│   ├── agent.yaml        nome, tipo (orquestrador | operador), modelo, recursos (nomes do Nexus)
│   ├── persona.md        tom com o morador; só no orquestrador de primeiro nível
│   ├── caso.yaml         caso de uso determinístico; a única porta de escrita
│   ├── rules/            output.md = contrato de saída; os demais são políticas
│   ├── workflows/        principal.md = o que ele faz
│   ├── routers/          rotas.md e desambiguacao.md
│   ├── knowledge/**      glossary.yaml e docs OKF tipados (herda)
│   ├── assets/**         templates citados como assets/<arquivo> (herda)
│   ├── evals/*.yaml      casos que provam o comportamento
│   └── agentes/<sub>/    subagentes, mesma anatomia
└── nexus/<account>.yaml  instantâneo dos recursos (usado sem NEXUS_BASE_URL)
```

- **Herança**: knowledge e assets do pai valem pros filhos, o mais próximo vence. O resto é de cada um.
- **Fronteira**: agente só referencia a própria pasta ou um ancestral.
- **Escrita**: recurso com `write: true` só entra por passo do `caso.yaml`, depois do `confirm`. Leitura o agente declara em `recursos` e chama com `chamar_recurso`.
- **Mock**: sem `NEXUS_BASE_URL`, `chamar_recurso` responde pela `amostra` do instantâneo; escrita sem amostra devolve `MOCK-000123`.

</details>

<details>
<summary><strong>Pedidos, memória e modos de orquestração</strong></summary>

**Pedidos.** Cada pedido vira um protocolo (`P-7K3Q9`) ligado a um caso de uso. Estado decidido em código pelos parâmetros validados, nunca inferido do texto:

```
coletando → pronto → aguardando_confirmacao → executando → concluido
         ↘ com_atendente ↗                               ↘ cancelado | falhou
```

O detector diz o que a mensagem contém, o motor percorre o `caso.yaml`, o restante vai ao orquestrador. Pedido de atendente gera resumo pro humano e passa a `com_atendente`. Tabelas `pedidos` e `pedido_eventos`.

**Memória.** O que o morador afirma sobre si vira fato com chave estável (`memoria_contatos`, `memoria_fatos`). O extrator usa `MODEL_EXTRACT` ou regras sem modelo; a memória entra no prompt da família inteira.

**Orquestração** (`agent.yaml → orquestracao`, editável no Studio em Avançado):

| Modo | Como decide |
|---|---|
| `router` (padrão) | uma chamada barata escolhe uma rota |
| `planning` | monta o TODO inteiro, pergunta tudo de uma vez |
| `react` | loop pensar, agir, observar, uma ação por passo |

Com os Pedidos ligados, o motor é a espinha dorsal e o orquestrador atende só o que não é pedido.

**okf-run** (CLI do harness):

```bash
./bin/okf-run.exe -root okf "posso fazer barulho depois das 22h?"
./bin/okf-run.exe -root okf --inspecionar   # projeção, roteador e tools, sem modelo
./bin/okf-run.exe -root okf -mock "..."     # modelo falso, sem rede
```

Gemini e OpenRouter sem chave caem em mock. Em `luna`, chave ausente é erro.

</details>

## Acesso e papéis

Toda rota `/api/*`, o emulador e o repasse `/mcp/` exigem usuário. Fora: webhook real (HMAC), `/healthz`, `/readyz` e estáticos. Sem usuário: 401. Sem permissão no alvo: 403.

| Papel | Herda de | Acrescenta |
|---|---|---|
| `operador_administradora` | | ver skills, conversas, pedidos e memória; atender conversas |
| `gestor_administradora` | operador | habilitar condomínios, editar textos, editar pedidos, esquecer memória, histórico dos ganchos |
| `construtor_superlogica` | gestor | editar fluxo, criar e excluir skills, ferramentas, condomínios, `studio.avancado`, editar ganchos |
| `admin_superlogica` | construtor | editar administradoras e usuários |

O papel vale no escopo atribuído (Superlógica, administradora ou condomínio). Atribuição num condomínio cai se ele mudar de administradora. Uma administradora pode receber concessão de `studio.avancado` pra um papel menor, só dentro dela.

| Env | Efeito |
|---|---|
| `AUTH_MODE=dev` | usuário vem de `X-Dev-Usuario` ou, sem ele, de `AUTH_DEV_USUARIO`. Boot recusa com `AMBIENTE` fora de vazio ou `dev` |
| `AUTH_MODE` vazio | API fechada, ainda não há login |
| `ADMINISTRADORA_ID` | fixa a implantação numa administradora; vazio atende todas. Memória e pedidos pedem `administradora` na query (400 `ADMINISTRADORA_OBRIGATORIA`) |

Usuários semeados, só válidos com `AUTH_MODE=dev`: `admin-superlogica`, `construtor-superlogica`, `gestor-exemplo` (`adm-exemplo`) e `gestor-avancado` (`adm-avancado`, com a concessão criada no boot de dev, nunca por migration). Outro usuário: `curl -H 'X-Dev-Usuario: <id>' ...`, ou o seletor "Ver como" no Studio.

## Ganchos

Um ponto único por fronteira do turno, o mesmo pra todas as camadas. Spec: [ganchos](docs/superpowers/specs/2026-11-03-ganchos-design.md).

| Evento | Onde dispara |
|---|---|
| `mensagem_recebida` | antes de qualquer camada; mascara CPF e telefone |
| `antes_da_ferramenta` | em `invocar` |
| `depois_da_ferramenta` | em `invocar`, antes do resultado voltar |
| `antes_de_responder` | logo antes do `Sender.Enviar` |
| `transbordo_ou_encerramento` | transbordo e encerramento |
| `copiloto_antes_de_aceitar` | por proposta do Copiloto |

- **Desfechos**: deixar, bloquear com motivo, ajustar, pedir confirmação.
- **Fixos** em Go (`internal/ganchos/fixos`): não desligam por API, banco, flag nem env. Conjunto sem eles é recusado no boot.
- **Regras** declarativas por condomínio (`internal/ganchos/regras`): sem regex nem script, só a Superlógica edita, cache aquecido no boot. Nunca afrouxam um fixo nem impedem o transbordo.
- **Auditoria**: toda decisão diferente de deixar vira linha em `ganchos_auditoria`, sem texto, params nem PII, gravada depois do envio.
- **Export e import** assinados com HMAC (`GANCHOS_SEGREDO_EXPORTACAO`).

## Logs e erros

| O quê | Como |
|---|---|
| Formato | JSON no stderr |
| PII | `pii.NovoHandler` troca CPF e telefone por `«cpf»` e `«tel»` em mensagem, atributos e pilha |
| Request id | cada requisição ganha `request_id`, devolvido em `X-Request-Id` |
| Pilha | `falha.ComPilha` na borda de I/O (Postgres, MCP, Gruvi, ATENDE, LLM). Erro esperado (sentinela, validação) não ganha |
| Uma linha por falha | o erro sobe embrulhado e loga uma vez na borda de cima: 5xx com `request_id`, `usuario_id`, `operacao`, `erro`, `chamador`, `stack` |
| Pânico | handler responde 500 `ERRO_INTERNO`; goroutine de fundo usa `falha.Recuperar`, ambos com a pilha |
| Fonte | todo ERROR traz `fonte` (arquivo:linha) |
| Studio | error boundaries em `web/studio/src/app/erro/LimiteDeErro.tsx`: `LimiteDeRota` por rota, `ErroDaRaiz` na raiz |

Servidor com prazos contra slowloris (cabeçalho 10s, leitura 60s, escrita 70s, ocioso 120s) e desligamento gracioso que espera turnos e trabalhos de fundo (`DESLIGAMENTO_PRAZO`).

## Endpoints

<details>
<summary><strong>Rotas do okf-admin</strong></summary>

| Grupo | Rotas |
|---|---|
| Público | `POST /webhook/atende` (HMAC), `GET /healthz`, `GET /readyz` |
| Serviço | `GET /integracao/v1/pacote`, `PUT /integracao/v1/mapeamento`, `GET /integracao/v1/memorias`, `GET\|DELETE /integracao/v1/memoria`, `GET /integracao/v1/pedidos` (chave de serviço, sem usuário) |
| Studio | `GET /studio/`, `GET /api/eu`, `GET\|POST /api/skills`, `GET\|DELETE /api/skills/{nome}`, `PUT /api/skills/{nome}/textos`, `GET\|PUT /api/skills/{nome}/fluxo`, `POST /api/skills/{nome}/duplicar` |
| Copiloto | `GET /api/copiloto/estado`, `POST /api/copiloto` |
| Agentes OKF | `GET\|POST /api/agentes`, `GET\|DELETE /api/agente`, `PUT /api/agente/config`, `POST /api/agente/recursos`, `POST /api/agentes/importar`, `GET /api/agentes/exportar`, `POST /api/agentes/mover` (410), `GET\|PUT\|DELETE /api/arquivo`, `GET /api/arquivo/modelo`, `GET /api/projecao`, `GET /api/anatomia` |
| Nexus | `GET /api/nexus/recursos`, `PUT\|DELETE /api/nexus/recurso` |
| Admin | `GET /api/admin/estado`, `POST\|PUT\|DELETE /api/admin/administradora`, `POST\|PUT\|DELETE /api/admin/condominio`, `PUT /api/admin/habilitacao`, `PUT /api/admin/orientacao`, `PUT /api/admin/ajustes`, `PUT /api/admin/payload`, `POST /api/admin/sugerir-payload`, `GET\|PUT /api/admin/orquestracao` |
| Conversas | `GET /api/atende/sessoes`, `GET\|DELETE /api/atende/sessao`, `GET /api/atende/saidas`, `POST /api/atende/transbordo`, `GET /api/atende/config`, `GET /api/atende/identidade` |
| Pedidos e memória | `GET /api/pedidos`, `GET /api/pedidos/exportar`, `POST /api/pedidos/acao`, `GET\|DELETE /api/memoria`, `GET /api/memorias` |
| Ganchos | `GET /api/ganchos/fixos`, `GET\|POST /api/ganchos/regras`, `GET\|PUT /api/ganchos/regras/{id}`, `GET /api/ganchos/regras/{id}/versoes/{versao}`, `GET /api/ganchos/regras/exportar`, `POST /api/ganchos/regras/importar`, `GET /api/ganchos/historico` |
| Emulador | `POST /emulador/atende/webhook`, `POST /emulador/atende/messages` |
| MCP | `GET\|POST\|DELETE\|OPTIONS /mcp/...` (repasse, `studio.avancado`) |

</details>

## Legado

Ficam no ar pra comparação, cada uma com link a partir do Studio.

| Tela | Rota | O que é |
|---|---|---|
| Admin | `/admin` | condomínios e matriz de casos de uso por administradora |
| Backdoor | `/backdoor`, `/backdoor/administradoras`, `/backdoor/payloads`, `/backdoor/orquestracao` | casos de uso, administradoras, payload de cada recurso por condomínio |
| Console | `/console` | emulador do ATENDE: conversa, identidade, rota, pedidos, memória, transbordo |

Console e telas antigas moram em `web/okf`, sem build; com `-web web/okf` são servidas do disco e editadas sem rebuild.

<details>
<summary><strong>mcp/: servidor MCP do catálogo Gruvi (Node, npm)</strong></summary>

Única parte do repo que usa npm. Transforma as OpenAPIs do BFF do Gruvi (`building-operation.json`, `mobile.json`) em tools `gruvi.bff.<módulo>.<operação>` em `/mcp/gruvi/bff`. O concierge consome via `internal/mcp`: `tools/list` vira recursos da account e `chamar_recurso` executa por `tools/call`.

```bash
cp mcp/.env.example mcp/.env
docker compose up -d --build                               # mcp na rede do concierge, porta 3337 não publicada
cd mcp && npm ci && npm run start:mcp:catalog-agent        # fora do Docker, pra usar o Explorer
```

- O MCP só escuta em loopback; de fora, use o repasse `http://127.0.0.1:8090/mcp/gruvi/bff` com `Authorization: Bearer <token B2C>`. O repasse não injeta o token do `.env`.
- `MCP_AUTH=b2c` (padrão): token pelo SMS de dev do Spaces, renovado pelo `refresh_token`, gravado em `MCP_TOKEN_FILE`. `file`, `bearer` e `keycloak` são as alternativas.
- **401 do MCP?** `MCP_B2C_MOBILE` sem acesso, ou o `concierge` recriado sem recriar o `mcp`. `docker compose up -d --build` recria os dois.

</details>

## Docs

| Tema | Spec | Plano | Prova |
|---|---|---|---|
| Fundação (Postgres, RBAC) | [spec](docs/superpowers/specs/2026-11-03-fundacao-design.md) | [plano](docs/superpowers/plans/2026-11-03-fundacao.md) | [turno](docs/superpowers/medicoes/2026-11-03-turno.md) |
| Ganchos | [spec](docs/superpowers/specs/2026-11-03-ganchos-design.md) | [plano](docs/superpowers/plans/2026-11-03-ganchos.md) | [medição](docs/superpowers/medicoes/2026-11-03-ganchos.md) |
| Studio | [spec](docs/superpowers/specs/2026-11-03-studio-design.md) | [plano](docs/superpowers/plans/2026-11-03-studio.md), [fase 2](docs/superpowers/plans/2026-11-03-studio-fase2.md), [na Fundação](docs/superpowers/plans/2026-11-04-studio-na-fundacao.md) | [smoke por papel](docs/superpowers/medicoes/2026-11-04-smoke-studio-na-fundacao.md) |
| Skills base | [spec](docs/superpowers/specs/2026-11-04-skills-base-design.md) | [plano](docs/superpowers/plans/2026-11-04-skills-base.md) | |
| Memória classificada | [spec](docs/superpowers/specs/2026-11-03-memoria-classificada-design.md) | | |

Integração com o app concierge do Nexus: [contrato do pacote](docs/integracao/nexus.md).

Princípios do projeto: [constituição](docs/superpowers/constitution.md). Definições de partida: [DEFINICOES.md](DEFINICOES.md). Regras de código do front: [web/studio/README.md](web/studio/README.md).
