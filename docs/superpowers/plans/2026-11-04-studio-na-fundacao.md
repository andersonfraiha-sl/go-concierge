# Studio na Fundação Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Levar o Studio (`feat/studio-react`) para cima da Fundação (`origin/main`, PR #2): merge sem quebra, todas as rotas do Studio atrás do acesso por papel e escopo, `/api/eu` e Copiloto lendo o `Acesso` real em vez do cabeçalho `X-Studio-Papel`, front falando `X-Dev-Usuario`, e contrato Go ↔ Zod preso por fixtures golden.

**Architecture:** O merge fica com a versão da main em tudo que é persistência e acesso (`admin.Repositorio` no Postgres, `admin.Cache` no turno, `rotear` com `protegida`/`protegidaEmAlgum`) e soma o que é do Studio (`rotasStudio`, `rotasCopiloto`, `GET /admin`, `casoDeUso.resumo/icone`, handlers do `legado.go`). O núcleo puro de acesso (`internal/acesso`) ganha `PermissoesEmAlgum` e `PapelPrincipal`, e o `Acesso` passa a trazer `Nome`, lido na mesma consulta que já carrega o usuário. `/api/eu` e o Copiloto montam a identidade a partir do `Acesso` do contexto; o cabeçalho `X-Studio-Papel` deixa de existir nos dois lados. O contrato vira arquivo: o teste Go grava a resposta real em `cmd/okf-admin/testdata/contrato/*.json` e o Vitest faz parse com o schema Zod.

**Tech Stack:** Go 1.27 (net/http, ent, pgx v5, goose, testcontainers), Postgres 18, React 19.3, MUI 9.5, TanStack Query 5, React Router 8, Zod 4, Vitest 5 + Testing Library, Biome 2.6.

**Spec:** `docs/superpowers/specs/2026-11-03-studio-design.md` (Studio) e `C:\Users\felip\Desktop\Superlogica\go-concierge\docs\superpowers\specs\2026-11-03-fundacao-design.md` (Fundação, na main) · Base do desenho: `docs/superpowers/pesquisas/2026-11-04-studio-na-fundacao.md` (seção 6 = as 10 tarefas; seções 1 a 4 = detalhes; esta numeração de tasks é a mesma) · Constituição: `docs/superpowers/constitution.md` · Plano anterior (estilo): `docs/superpowers/plans/2026-11-03-studio-fase2.md`

## Decisões tomadas neste plano (divergem ou completam a pesquisa)

1. **`legado.go` sai inteiro no merge, e os corpos dele vencem.** A pesquisa viu só o panic de rota duplicada. Há também método duplicado: `anatomia`, `criar`, `excluir` e `mover` existem em `main:cmd/okf-admin/main.go:425-575` e em `studio:cmd/okf-admin/legado.go`. Os da main não compilam contra o `okf` do Studio (`s.cat.ExcluirAgente` e `s.cat.Mover` não existem mais). Fica o corpo do `legado.go` (movido para `main.go` no lugar dos da main), a main registra as rotas protegidas, e `rotasLegado` deixa de existir. Backdoor e Admin continuam servidos.
2. **`mover` responde 410.** Consequência do item 1: o caso "mover agente" de `TestRotasExigemPermissao` passa de `quer: 400` para `quer: 410`.
3. **Na Task 1 o Copiloto só troca `Ler()` por `Ler(r.Context())`.** `visaoPara` depende do `Acesso` no contexto, que só existe quando a rota é protegida (Task 4). Trocar o recorte antes zeraria o retrato e quebraria os testes do Copiloto. `recorteDoUsuario` + `visaoPara` entram na Task 4.
4. **`euDev` sai na Task 4, não na 3.** Enquanto o Copiloto o usa, apagar quebra o build. A Task 3 troca só o handler de `/api/eu`.
5. **Usuários de dev: migration para usuários e atribuições, boot de dev para a concessão.** `concessoes.administradora_id` tem FK para `administradoras` (`main:internal/banco/migrations/20261103154500_acesso.sql:13`), e a concessão vale para todos os gestores da administradora. Então o "gestor com Avançado" precisa de uma administradora própria, `adm-avancado`, e ela não pode nascer numa migration que também roda em produção. A migration semeia `construtor-superlogica`, `gestor-exemplo` e `gestor-avancado`, e um `semearAcessoDev` só em `AMBIENTE=dev` + `AUTH_MODE=dev` cria `adm-avancado` e a concessão.
6. **`resetQueries()` em vez de `clear()` na troca de usuário.** `clear()` tira as queries do cache, mas os observadores montados não buscam de novo até o próximo render. `resetQueries()` volta tudo ao estado inicial, sem os dados do usuário anterior, e busca de novo as ativas. Atende o objetivo da pesquisa (nada do usuário anterior na tela).
7. **Usuário sem papel recebe 200 em `/api/eu`**, com `permissoes: []`, `papel: ""` e `administradoraId: null`. Ele está autenticado, então 401 seria falso. Sem permissão nenhuma, o front não mostra nada editável.

## Global Constraints

- **Back-end é Go nativo.** Nada de Node no servidor. O MCP (`mcp/`) continua como está.
- **Front com pnpm via corepack.** O Node 26 não traz mais o corepack embutido. Instalar com `npm install -g corepack@latest`, depois `corepack enable && corepack install`, que lê o `packageManager: pnpm@12.16.1` do `web/studio/package.json`. Nunca `npm install -g pnpm`.
- **Node 26 LTS** (`node:26.13-trixie-slim`, `engines.node: ">=26.13 <27"`).
- **Testes que tocam banco com `-p 1`** (`go test -p 1 ./...`). O testcontainers sobe um Postgres por pacote.
- **`-race` roda em Docker, na `golang:1.27-trixie`, com o socket do Docker montado** (CGO e gcc vêm na imagem trixie).
- **Nada antigo é apagado:** o Admin (`web/okf/index.html` + `admin.js`, servido em `GET /admin`), o Backdoor (`/backdoor*`) e `mcp/` ficam. Isto revoga, só para este plano, a Task 31 do plano da fase 2.
- **Front:** componentes com menos de 200 linhas, nenhuma cor hexadecimal fora de `src/theme/tokens.ts`, Biome (`pnpm run lint`) verde, TypeScript strict, sem `any`, sem `as` fora de teste, sem `!`, Zod em toda borda de API.
- **Go:** erro com contexto e sem "failed to", nada engolido, nenhum estado global mutável, toda chamada externa com `context` e prazo. Handler fino, regra pura no pacote de domínio.
- **Commits:** conventional sem ticket (`feat: …`, `fix: …`, `test: …`, `chore: …`, `docs: …`), descrição minúscula, até 72 caracteres, `git commit` num comando Bash próprio, sem heredoc, **sem trailer de atribuição** e sem mencionar IA. Um commit por task, no mínimo, assim que ela fecha.
- **Latência:** um turno quente (contato já visto, sem mudança de pedido) faz **0 idas ao banco antes da resposta**. O guarda é `TestTurnoNoPostgresIdasAntesDaResposta` (`internal/atende/turno_bench_test.go`, caso "contato quente, pedido sem mudança" = 0). Nenhuma task pode pôr `s.admin` (Postgres direto) no caminho do turno. O turno lê de `s.atende.Mapeamento = admin.NovoCache(adminPG)`.
- **Erros da API:** novos handlers do Studio respondem `erroAPI{Codigo, Mensagem}` (`{codigo, mensagem}`); o middleware da main responde `{codigo, error}`. O `http.ts` lê os dois.
- Skills de quem implementa: `code-quality` + `go` (tasks Go), `code-quality` + `typescript` + `react` (tasks de front), e `api-design` em toda task que cria ou protege rota. `code-quality` prevalece sobre precedente local em código novo.
- Nome de teste em verbo na 3ª pessoa, sem "should" (Go: `TestXxx` descritivo; Vitest: `it('createsWithPost')`).

### Comandos (rodar da raiz do worktree, em Git Bash)

```bash
DOCKER_DO_HOST="-v /var/run/docker.sock:/var/run/docker.sock -e TESTCONTAINERS_HOST_OVERRIDE=host.docker.internal -e TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock --add-host=host.docker.internal:host-gateway"
GO="MSYS_NO_PATHCONV=1 docker run --rm $DOCKER_DO_HOST -v $PWD:/src -w /src -v sl-concierge-gomod:/go/pkg/mod -v sl-concierge-gocache:/root/.cache/go-build golang:1.27-trixie"
STUDIO="MSYS_NO_PATHCONV=1 docker run --rm -e HUSKY=0 -v $PWD:/src -v /src/web/studio/node_modules -w /src/web/studio node:26.13-trixie-slim"
PNPM="npm install -g corepack@latest >/dev/null && corepack enable && corepack install && pnpm install --frozen-lockfile"
```

- Go, um teste: `eval "$GO" sh -c "'go test -p 1 -run <Regex> ./cmd/okf-admin/'"`
- Go, tudo: `eval "$GO" sh -c "'gofmt -l . && go vet ./... && go test -p 1 ./...'"` (esperado: `gofmt -l` sem saída e todos os pacotes `ok`)
- Race: `eval "$GO" sh -c "'CGO_ENABLED=1 go test -race -p 1 ./cmd/okf-admin/ ./internal/acesso/ ./internal/atende/'"` (esperado: `ok` e nenhum `WARNING: DATA RACE`)
- Vitest, um arquivo: `eval "$STUDIO" sh -c "'$PNPM && pnpm exec vitest run <arquivo>'"`
- Front, tudo: `eval "$STUDIO" sh -c "'$PNPM && pnpm run lint && pnpm run typecheck && pnpm test'"`
- Depois da Task 1, `./build.sh --test` faz os dois "tudo" de uma vez.

## Review Focus

1. **Spoofing pelo `X-Studio-Papel`.** Um gestor de A que manda `X-Studio-Papel: construtor` (ou qualquer valor) continua sendo gestor de A: o retrato do Copiloto só traz condomínios de A, as operações oferecidas ao modelo são as do gestor, e `/api/eu` ignora o cabeçalho. → Task 4 `TestCopilotoIgnoraCabecalhoDePapel`; Task 3 `TestEuIgnoraCabecalhoDePapel`; Task 5 `stopsSendingStudioRoleHeader`.
2. **Panic no boot por rota duplicada.** Qualquer padrão registrado duas vezes no `ServeMux` derruba o processo antes de servir. → Task 1 `TestRotearNaoEntraEmPanico` (monta `rotear` com e sem MCP).
3. **`GET /` redireciona.** `/` responde 302 para `/studio/`. O Admin antigo continua público em `GET /admin` e os arquivos estáticos continuam em `GET /` (ex.: `/admin.js`). → Task 1, casos novos em `TestRotasAutenticacao`.
4. **401 sem tela e sem dados velhos.** `/api/eu` com 401 mostra "Não reconhecemos seu usuário" e não fica tentando de novo. Na troca de usuário, nada do anterior aparece. → Task 8 `showsUnauthenticatedScreen`, `showsUnauthenticatedScreenInLayout`, `doesNotRetryOnUnauthenticated`, `dropsPreviousUserDataOnSwitch`.
5. **409 legível.** Criar condomínio ou administradora com id que já existe (`409 JA_EXISTE`) mostra uma frase em PT-BR no rodapé. `/api/eu` para usuário com duas administradoras responde `409 ADMINISTRADORA_AMBIGUA` em vez de escolher uma por ele. → Task 6 `showsConflictMessageOnDuplicateCondominio`, `showsConflictMessageOnDuplicateAdministradora`; Task 3 caso `gestor-ab` em `TestEu`.

---

## Mapa de arquivos

| Arquivo | Responsabilidade | Tasks |
|---|---|---|
| `cmd/okf-admin/main.go` | struct `servidor` (main + `studio`, `copiloto`, `limiteCopiloto`), boot, `rotas()`; recebe os corpos de `anatomia/criar/excluir/mover` | 1, 5 |
| `cmd/okf-admin/legado.go` | apagado (corpos vão para `main.go`) | 1 |
| `cmd/okf-admin/rotas_acesso.go` | `rotear` chama `rotasStudio` e `rotasCopiloto` | 1 |
| `cmd/okf-admin/admin.go` | registro da main + `GET /admin`; `casoDeUso.Resumo/Icone`; GET de orquestração em algum escopo | 1, 7 |
| `cmd/okf-admin/studio.go` | rotas de skills protegidas; `/api/eu` pelo `Acesso` | 2, 3 |
| `cmd/okf-admin/copiloto.go`, `copiloto_test.go` | Copiloto no `Acesso`, recorte, limite por `UsuarioID` | 1, 4 |
| `cmd/okf-admin/autenticacao.go` | `usuariosSemeadosDev` vira conjunto | 5 |
| `cmd/okf-admin/acesso_dev.go` (novo) | `semearAcessoDev` | 5 |
| `cmd/okf-admin/rotas_acesso_test.go` | casos novos nas tabelas; fake com `Nome` e concessões | 1, 2, 3, 4, 5, 7 |
| `cmd/okf-admin/contrato_test.go` (novo) | `conferirContrato` + `TestContratosDoStudio` | 3, 9 |
| `cmd/okf-admin/testdata/contrato/*.json` (novos) | respostas golden | 3, 9 |
| `internal/acesso/acesso.go`, `acesso_test.go` | `Acesso.Nome`, `PermissoesEmAlgum`, `PapelPrincipal` | 3 |
| `internal/acesso/postgres.go`, `postgres_test.go` | `Nome` no `Carregar`; `Conceder` | 3, 5 |
| `internal/banco/migrations/20261104120000_usuarios_dev_studio.sql` (novo) + `atlas.sum` | usuários de dev do Studio | 5 |
| `build.sh`, `README.md` | união dos dois lados; seção Acesso | 1 |
| `web/studio/src/shared/api/http.ts`, `http.test.ts` | `X-Dev-Usuario` no lugar de `X-Studio-Papel` | 5 |
| `web/studio/src/acesso/DevRoleSwitch.tsx`, `.test.tsx`; `app/Topbar.tsx` | troca de usuário de dev; `resetQueries` | 5, 8 |
| `web/studio/src/acesso/ErroAcesso.tsx`, `eu.test.tsx`; `shared/api/http.ts`; `main.tsx` | tela de 401, sem retry em 401 | 8 |
| `web/studio/src/condominios/contratos.ts`, `NovoCondominio.tsx`, `AdministradorasAba.tsx` (+ testes) | criar com POST, 409 legível | 6 |
| `web/studio/src/test/contrato.ts`, `src/contrato.test.ts` (novos) | ler fixture golden e fazer parse | 9 |
| `docs/superpowers/medicoes/2026-11-04-smoke-studio-na-fundacao.md` (novo) | checklist do smoke por papel | 10 |

## Ondas e worktrees

Cada task roda num worktree próprio, criado a partir de `feat/studio-react` **depois** que as dependências dela já estão mergeadas lá. O worktree nasce em `main`: o primeiro passo é `git switch -c <branch-da-task> feat/studio-react` (ou `git merge --ff-only feat/studio-react`). Ao fechar, o controlador mergeia a branch da task em `feat/studio-react` (fast-forward ou merge simples) antes de abrir a próxima onda.

| Onda | Tasks | Observação |
|---|---|---|
| 1 | 1 | sozinha |
| 2 | 2 → 7 em sequência; 3 e 6 em paralelo com elas | 2, 3 e 7 mexem em `rotas_acesso_test.go`. Ordem de integração: 2, 7, 3, 6; quem integra depois rebaseia, e o conflito é de tabela (trivial) |
| 3 | 4 e 5 em paralelo | ambas dependem da 3; 4 e 5 mexem em `rotas_acesso_test.go` (5 só em `TestUsuarioSemeadoSoValeEmModoDev`) |
| 4 | 8 (depois da 5) e 9 (depois da 3 e da 4) em paralelo | |
| 5 | 10 | smoke ponta a ponta |

---

### Task 1: Merge de `origin/main` em `feat/studio-react` (Go + build)

**Files:**
- Modify: `cmd/okf-admin/main.go` (struct `servidor`, literal, boot do mapeamento, `rotas()`, handlers `anatomia/criar/excluir/mover`)
- Delete: `cmd/okf-admin/legado.go`
- Modify: `cmd/okf-admin/rotas_acesso.go:28-41` (`rotear`)
- Modify: `cmd/okf-admin/admin.go` (registro de rotas e `casoDeUso`)
- Modify: `cmd/okf-admin/copiloto.go:243`, `cmd/okf-admin/copiloto_test.go:42-60`
- Modify: `cmd/okf-admin/rotas_acesso_test.go` (`TestRotasAutenticacao`, caso "mover agente"), `build.sh`, `README.md`
- Test: `cmd/okf-admin/rotas_acesso_test.go` (`TestRotearNaoEntraEmPanico`)

**Interfaces:**
- Consumes: main — `admin.Repositorio`, `admin.NovaEmMemoria()`, `admin.NovoCache`, `s.protegida`, `s.protegidaEmAlgum`, `alvoSuperlogica`, `novoAmbienteRotas`.
- Produces: `func (s *servidor) rotear(mcp http.Handler) http.Handler` registrando, nesta ordem, `s.rotas`, `s.rotasAtende`, `s.rotasAdmin`, `s.rotasStudio`, `s.rotasCopiloto`, MCP. Campos `servidor.studio fs.FS`, `servidor.copiloto func(context.Context, harness.PedidoAoCopiloto) (harness.RespostaDoModelo, error)`, `servidor.limiteCopiloto *limiteDoCopiloto`. `rotasStudio` e `rotasCopiloto` ainda com `mux.HandleFunc` sem proteção (Tasks 2, 3 e 4 protegem).

- [ ] **Step 1: Criar a branch da task e rodar o merge**

```bash
git switch -c merge/studio-na-fundacao feat/studio-react
git fetch origin
git merge --no-ff --no-commit origin/main
```
Esperado: `CONFLICT` em exatamente `README.md`, `build.sh`, `cmd/okf-admin/admin.go`, `cmd/okf-admin/main.go`.

- [ ] **Step 2: Resolver os 4 conflitos como na pesquisa §4**
  - `main.go`: fica a main inteira (struct, literal, boot com `semearMapeamento`, `admin.NovoCache`, `s.atende.Mapeamento = mapeamento`, `rotas()` com `protegida`). Somar os 3 campos do Studio e o `NovoCopiloto`/`novoLimiteDoCopiloto` do boot do Studio. O seed `s.admin.Ler()` do Studio (studio:main.go:167-175) sai. Não sobra `mux.HandleFunc` das rotas do Studio em `main.go`.
  - `main.go`, rotas: somar `mux.Handle("GET /{$}", http.RedirectHandler("/studio/", http.StatusFound))` logo depois de `padraoEstaticos`.
  - `main.go`, handlers: apagar os corpos da main de `anatomia`, `criar`, `excluir`, `mover` e colar no lugar os de `legado.go` (o comentário do `mover` vai junto). Apagar `legado.go`.
  - `admin.go`: fica o registro da main; somar `mux.HandleFunc("GET /admin", s.telaAntiga("index.html"))` e manter `telaAntiga`. `casoDeUso` mantém os campos `Resumo` (`json:"resumo"`) e `Icone` (`json:"icone"`) do Studio. Usar `escrever` da main; `responderMapeamento` sai.
  - `build.sh`: união. Imagem Go `golang:1.27-trixie` (Studio), `DOCKER_DO_HOST` e `-p 1` (main), `docker compose up -d --wait postgres` + `DATABASE_URL` no `--run` (main), o passo do Studio antes do Go nos três modos (Studio). Trocar `PNPM` pelo da seção Comandos (corepack). Atualizar o cabeçalho de comentário dizendo as duas coisas.
  - `README.md`: união das seções de rodar e de áreas; acrescentar a seção "Acesso" com `AUTH_MODE=dev`, `X-Dev-Usuario`, `AUTH_DEV_USUARIO` e a lista de usuários semeados (por enquanto só `admin-superlogica`).

- [ ] **Step 3: Fazer compilar**
  - `rotear` (rotas_acesso.go): acrescentar `s.rotasStudio(mux)` e `s.rotasCopiloto(mux)` depois de `s.rotasAdmin(mux)`.
  - `copiloto.go:243`: `mapa, err := s.admin.Ler(r.Context())`. O recorte continua `mapa.Visao(quem.Administradora)` (decisão 3).
  - `copiloto_test.go:42-60`: `s.admin = admin.NovaEmMemoria()` e as chamadas `SalvarAdministradora(t.Context(), a)`, `SalvarCondominio(t.Context(), c)`. `SalvarOrientacao` passa a `SalvarOrientacao(t.Context(), admin.Alvo{Condominio: "solar", Dona: "adm-outra"}, admin.Orientacao{Geral: "segredo do Solar"})`.
  - Qualquer outro `admin.NewStore` em teste do Studio: trocar por `admin.NovaEmMemoria()`.

Run: `eval "$GO" sh -c "'go build ./... && go vet ./...'"`
Expected: sem saída.

- [ ] **Step 4: Escrever o teste do panic e atualizar os casos da main**

Em `rotas_acesso_test.go`:

```go
func TestRotearNaoEntraEmPanico(t *testing.T) {
	for _, nome := range []string{"sem mcp", "com mcp"} {
		t.Run(nome, func(t *testing.T) {
			amb := novoAmbienteRotas(t)
			var mcp http.Handler
			if nome == "com mcp" {
				mcp = http.NotFoundHandler()
			}
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("rotear entrou em panic: %v", r)
				}
			}()
			amb.s.rotear(mcp)
		})
	}
}
```

Em `TestRotasAutenticacao`, trocar o caso `"página do Admin é pública"` por:

```go
{nome: "raiz redireciona para o Studio", url: "/", status: http.StatusFound},
{nome: "Admin antigo é público", url: "/admin", status: http.StatusOK},
{nome: "arquivo do Admin é público", url: "/admin.js", status: http.StatusOK},
{nome: "Studio é público", url: "/studio/", status: http.StatusOK},
```
e, depois do laço, conferir `Location`:

```go
t.Run("raiz aponta para /studio/", func(t *testing.T) {
	if got := amb.pedir(t, "", "GET", "/", "").Header().Get("Location"); got != "/studio/" {
		t.Fatalf("Location = %q, quero /studio/", got)
	}
})
```
`novoAmbienteRotas` passa `studio: fstest.MapFS{"index.html": {Data: []byte("<html>studio</html>")}}`, o mesmo `fs.FS` falso de `studio_web_test.go:17` (o `dist` não existe no contêiner de teste Go). Em `TestRotasExigemPermissao`, o caso `"mover agente"` passa a `quer: 410` (decisão 2).

- [ ] **Step 5: Rodar tudo**

Run: `eval "$GO" sh -c "'gofmt -l . && go vet ./... && go test -p 1 ./...'"` e o race da seção Comandos.
Expected: todos `ok`, incluindo `TestRotearNaoEntraEmPanico`, `TestRotasAutenticacao`, `TestRotasExigemPermissao`, os testes do Copiloto e `internal/atende` `TestTurnoNoPostgresIdasAntesDaResposta` (latência intacta).

Run: front, tudo.
Expected: lint, typecheck e testes verdes (o front não muda nesta task).

- [ ] **Step 6: Commit do merge**

```bash
git add -A
```
```bash
git commit -m "chore: merge fundacao into studio"
```

---

### Task 2: Proteger as 8 rotas de skills (Go)

**Files:**
- Modify: `cmd/okf-admin/studio.go` (`rotasStudio`)
- Test: `cmd/okf-admin/rotas_acesso_test.go` (`TestRotasExigemPermissao`)

**Interfaces:**
- Consumes: `s.protegida(p, alvo, h)`, `s.protegidaEmAlgum(p, h)`, `alvoSuperlogica`, constantes `acesso.Skills*`.
- Produces: rotas de `/api/skills*` protegidas. `GET /api/eu` fica com a Task 3; `GET /studio/` e `GET /studio` seguem públicos.

| Rota | Proteção |
|---|---|
| `GET /api/skills`, `GET /api/skills/{nome}`, `GET /api/skills/{nome}/fluxo` | `protegidaEmAlgum(acesso.SkillsLer, …)` |
| `POST /api/skills`, `POST /api/skills/{nome}/duplicar` | `protegida(acesso.SkillsCriar, alvoSuperlogica, …)` |
| `PUT /api/skills/{nome}/textos` | `protegida(acesso.SkillsEditarTextos, alvoSuperlogica, …)` |
| `PUT /api/skills/{nome}/fluxo` | `protegida(acesso.SkillsEditarFluxo, alvoSuperlogica, …)` |
| `DELETE /api/skills/{nome}` | `protegida(acesso.SkillsExcluir, alvoSuperlogica, …)` |

- [ ] **Step 1: Escrever os casos que falham**

Acrescentar à tabela de `TestRotasExigemPermissao`, num bloco `// skills do Studio`:

```go
{nome: "listar skills", metodo: "GET", url: "/api/skills", sem: "sem-papel", com: "operador-a", quer: 200},
{nome: "textos da skill", metodo: "GET", url: "/api/skills/agendar-mudanca", sem: "sem-papel", com: "operador-a", quer: 200},
{nome: "fluxo da skill", metodo: "GET", url: "/api/skills/agendar-mudanca/fluxo", sem: "sem-papel", com: "operador-a", quer: 200},
{nome: "criar skill", metodo: "POST", url: "/api/skills", corpo: `{}`, sem: "construtor-a", com: "admin", quer: 400},
{nome: "duplicar skill", metodo: "POST", url: "/api/skills/nao-existe/duplicar", corpo: `{}`, sem: "construtor-a", com: "admin", quer: 404},
{nome: "salvar textos", metodo: "PUT", url: "/api/skills/nao-existe/textos", corpo: `{}`, sem: "construtor-a", com: "admin", quer: 404},
{nome: "salvar fluxo", metodo: "PUT", url: "/api/skills/nao-existe/fluxo", corpo: `{}`, sem: "construtor-a", com: "admin", quer: 404},
{nome: "excluir skill", metodo: "DELETE", url: "/api/skills/nao-existe", sem: "construtor-a", com: "admin", quer: 404},
```
`construtor-a` tem as permissões só em `adm-a`, não o escopo da Superlógica, e por isso recebe 403 nas escritas do catálogo.

- [ ] **Step 2: Rodar e ver falhar**

Run: `eval "$GO" sh -c "'go test -p 1 -run TestRotasExigemPermissao ./cmd/okf-admin/'"`
Expected: FAIL em `sem permissão: status 200` (ou 400/404) nos 8 casos novos.

- [ ] **Step 3: Registrar com proteção** em `rotasStudio`, como na tabela acima, trocando `mux.HandleFunc` por `mux.Handle`. O handler não muda.

- [ ] **Step 4: Rodar e ver passar**

Run: o mesmo do Step 2, depois Go tudo.
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/okf-admin/studio.go cmd/okf-admin/rotas_acesso_test.go
```
```bash
git commit -m "feat: protect studio skill routes by permission"
```

---

### Task 3: `GET /api/eu` a partir do `Acesso` (Go)

**Files:**
- Modify: `internal/acesso/acesso.go`, `internal/acesso/acesso_test.go`
- Modify: `internal/acesso/postgres.go:33-65`, `internal/acesso/postgres_test.go`
- Modify: `cmd/okf-admin/studio.go` (`studioEu`, registro de `/api/eu`; `euDev` fica até a Task 4)
- Modify: `cmd/okf-admin/studio_test.go` (sair os casos de `/api/eu` com `X-Studio-Papel` em `TestStudioAPI`)
- Modify: `cmd/okf-admin/rotas_acesso_test.go` (`acessosFake` preenche `Nome`)
- Create: `cmd/okf-admin/eu_test.go`, `cmd/okf-admin/contrato_test.go`, `cmd/okf-admin/testdata/contrato/eu.json`

**Interfaces:**
- Consumes: `acessoDe(ctx)`, `administradorasVisiveis(a, adms)`, `s.admin.Ler(ctx)`, `s.authMode`, `authModeDev`.
- Produces:
  - `acesso.Acesso.Nome string`, preenchido por `Postgres.Carregar` com `u.Nome` (a consulta do usuário já existe; nenhuma ida a mais).
  - `func PermissoesEmAlgum(a Acesso) []Permissao` em `internal/acesso/acesso.go`: cada permissão de `todasAsPermissoes` (slice não exportado, na ordem das constantes) para a qual `PermitidoEmAlgum` é verdadeiro. Nunca nil.
  - `func PapelPrincipal(a Acesso) string`: o `PapelID` da atribuição vigente cuja cadeia de herança é a mais longa; empate fica com a primeira; sem atribuição vigente, `""`.
  - `func montarEu(a acesso.Acesso, adms []admin.Administradora, dev bool) (eu, error)` em `studio.go`. `errAdministradoraAmbigua` (sentinela) quando o usuário não é da Superlógica e vê mais de uma administradora.
  - `func conferirContrato(t *testing.T, nome string, corpo []byte)` em `contrato_test.go`, com a flag `-atualizar-contrato` (`flag.Bool`). Sem a flag, compara o JSON indentado (`json.Indent`, 2 espaços, `\n` no fim) com `testdata/contrato/<nome>.json` e falha mostrando os dois. Com a flag, grava. A Task 9 reutiliza.

Regras de `montarEu` (pesquisa §3, gap 1):

| Situação | `administradoraId` | Resposta |
|---|---|---|
| `Permitido(a, SkillsLer, superlogica)` | `null` | 200 |
| vê exatamente 1 administradora | o id dela | 200 |
| vê 0 (sem papel) | `null` | 200, `permissoes: []`, `papel: ""` |
| vê 2 ou mais | n/a | 409 `{codigo: "ADMINISTRADORA_AMBIGUA", mensagem: "Seu usuário está em mais de uma administradora. Peça para a Superlógica ajustar seu acesso."}` |

`dev` = `s.authMode == authModeDev`. `s.admin.Ler(ctx)` só é chamado quando o usuário não é da Superlógica (checagem barata primeiro).

- [ ] **Step 1: Testes do núcleo puro (falham)** em `internal/acesso/acesso_test.go`:

```go
func TestPermissoesEmAlgum(t *testing.T) {
	casos := []struct {
		nome   string
		ats    []Atribuicao
		conc   []Concessao
		tem    []Permissao
		naoTem []Permissao
	}{
		{"operador", naAdm("operador_administradora", "adm-a"), nil, []Permissao{SkillsLer}, []Permissao{CondominiosHabilitar, StudioAvancado}},
		{"gestor sem concessão", naAdm("gestor_administradora", "adm-a"), nil, []Permissao{SkillsEditarTextos}, []Permissao{StudioAvancado}},
		{"gestor com concessão", naAdm("gestor_administradora", "adm-a"), []Concessao{{AdministradoraID: "adm-a", PapelID: "gestor_administradora", Permissao: StudioAvancado}}, []Permissao{StudioAvancado}, nil},
		{"sem papel", nil, nil, nil, []Permissao{SkillsLer}},
	}
	// para cada caso: got := PermissoesEmAlgum(acessoCom(c.ats, c.conc)); got != nil;
	// slices.Contains(got, p) para todo p de tem; !slices.Contains(got, p) para todo p de naoTem
}

func TestPapelPrincipal(t *testing.T) {
	// admin_superlogica na superlogica + operador em adm-a → "admin_superlogica"
	// só gestor em adm-a → "gestor_administradora"
	// sem atribuição → ""
	// atribuição de condomínio cuja dona mudou (não vigente) é ignorada → ""
}
```
`naAdm` e `acessoCom` são helpers do teste (papéis de `PapeisIniciais()`); se o arquivo já tiver equivalentes, usar os de lá.

Em `internal/acesso/postgres_test.go`: `TestCarregarTrazNome` grava usuário `u-nome` com nome `"Ana"` e atribuição, e confere `a.Nome == "Ana"`.

- [ ] **Step 2: Rodar e ver falhar**

Run: `eval "$GO" sh -c "'go test -p 1 -run \"TestPermissoesEmAlgum|TestPapelPrincipal|TestCarregarTrazNome\" ./internal/acesso/'"`
Expected: FAIL de compilação (`undefined: PermissoesEmAlgum`, `a.Nome undefined`).

- [ ] **Step 3: Implementar** `Acesso.Nome`, `todasAsPermissoes`, `PermissoesEmAlgum`, `PapelPrincipal` e `Nome: u.Nome` no `Carregar`.

- [ ] **Step 4: Rodar e ver passar** (mesmo comando). Expected: PASS.

- [ ] **Step 5: Teste da rota (falha)** em `cmd/okf-admin/eu_test.go`. `acessosFake.Carregar` passa a pôr `Nome: "Nome de " + usuarioID`.

```go
func TestEu(t *testing.T) {
	amb := novoAmbienteRotas(t)
	casos := []struct {
		usuario          string
		status           int
		papel            string
		administradoraID *string
		tem, naoTem      []string
	}{
		{usuario: "admin", status: 200, papel: "admin_superlogica", administradoraID: nil, tem: []string{"skills.criar", "studio.avancado", "administradoras.editar"}},
		{usuario: "construtor-a", status: 200, papel: "construtor_superlogica", administradoraID: ptr("adm-a"), tem: []string{"skills.editar_fluxo", "studio.avancado"}},
		{usuario: "gestor-a", status: 200, papel: "gestor_administradora", administradoraID: ptr("adm-a"), tem: []string{"condominios.habilitar"}, naoTem: []string{"studio.avancado", "skills.criar"}},
		{usuario: "sem-papel", status: 200, papel: "", administradoraID: nil, naoTem: []string{"skills.ler"}},
		{usuario: "gestor-ab", status: 409},
	}
	// para cada caso: w := amb.pedir(t, c.usuario, "GET", "/api/eu", "")
	// status == c.status; em 409, corpo contém "ADMINISTRADORA_AMBIGUA";
	// em 200: decodificar em eu; ID == c.usuario; Nome == "Nome de "+c.usuario; Papel == c.papel;
	// AdministradoraID igual a c.administradoraID (nil vs valor); Dev == true;
	// Permissoes contém tem e não contém naoTem; Permissoes != nil (JSON "[]", nunca "null")
}

func TestEuSemUsuarioDa401(t *testing.T) {
	// amb.pedir(t, "", "GET", "/api/eu", "") → 401 e corpo com "NAO_AUTENTICADO"
}

func TestEuIgnoraCabecalhoDePapel(t *testing.T) {
	// GET /api/eu com X-Dev-Usuario: gestor-a e X-Studio-Papel: construtor →
	// Papel == "gestor_administradora", AdministradoraID == "adm-a", sem "skills.criar"
}
```
O "gestor com concessão" de `/api/eu` fica coberto por `TestPermissoesEmAlgum` (o fake não carrega concessões até a Task 7).

Em `contrato_test.go`:

```go
func TestContratosDoStudio(t *testing.T) {
	amb := novoAmbienteRotas(t)
	casos := []struct{ nome, usuario, metodo, url, corpo string }{
		{nome: "eu", usuario: "gestor-a", metodo: "GET", url: "/api/eu"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			w := amb.pedir(t, c.usuario, c.metodo, c.url, c.corpo)
			if w.Code != http.StatusOK {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
			conferirContrato(t, c.nome, w.Body.Bytes())
		})
	}
}
```

Em `studio_test.go`, tirar os dois casos de `/api/eu` (linhas ~303 e ~313) de `TestStudioAPI`. `TestEuDev` fica até a Task 4.

- [ ] **Step 6: Rodar e ver falhar**

Run: `eval "$GO" sh -c "'go test -p 1 -run \"TestEu|TestContratosDoStudio\" ./cmd/okf-admin/'"`
Expected: FAIL (`/api/eu` ainda responde pelo `euDev`; `papel` é `admin_superlogica` com id `dev-superlogica`).

- [ ] **Step 7: Implementar** `montarEu`, o novo `studioEu` e `mux.Handle("GET /api/eu", s.autenticado(http.HandlerFunc(s.studioEu)))`. Erro de `s.admin.Ler` → `s.responderErroInterno(w, r, "ler administradoras do usuário", err)`.

- [ ] **Step 8: Gerar a fixture e rodar tudo**

Run: `eval "$GO" sh -c "'go test -p 1 -run TestContratosDoStudio ./cmd/okf-admin/ -args -atualizar-contrato'"`, depois Go tudo.
Expected: `testdata/contrato/eu.json` criado com `"id": "gestor-a"`, `"papel": "gestor_administradora"`, `"administradoraId": "adm-a"`; tudo PASS sem a flag.

- [ ] **Step 9: Commit**

```bash
git add internal/acesso cmd/okf-admin/studio.go cmd/okf-admin/studio_test.go cmd/okf-admin/rotas_acesso_test.go cmd/okf-admin/eu_test.go cmd/okf-admin/contrato_test.go cmd/okf-admin/testdata/contrato/eu.json
```
```bash
git commit -m "feat: answer /api/eu from the loaded access"
```

---

### Task 4: Copiloto no `Acesso` (Go)

**Files:**
- Modify: `cmd/okf-admin/copiloto.go` (`rotasCopiloto`, `copilotoPerguntar`, `conversarComCopiloto`, `quemDoCopiloto`, `retratoParaCopiloto`, `contextoDaTela`, `arquivoParaCopiloto`)
- Modify: `cmd/okf-admin/studio.go` (apagar `euDev`), `cmd/okf-admin/studio_test.go` (apagar `TestEuDev`)
- Modify: `cmd/okf-admin/copiloto_test.go`, `cmd/okf-admin/rotas_acesso_test.go`

**Interfaces:**
- Consumes: Task 3 `acesso.PermissoesEmAlgum`; main `recorteDoUsuario(r, adms)`, `visaoPara(r, m, recorte)`, `acessoDe`.
- Produces:
  - `func quemDoCopiloto(a acesso.Acesso, recorte string) copiloto.Quem`, pura: `Permissoes` = `PermissoesEmAlgum(a)` como `[]string`; `Administradora` = `""` se `Permitido(a, SkillsLer, superlogica)`, senão `recorte`.
  - `retratoParaCopiloto(r *http.Request, tela copiloto.Contexto, quem copiloto.Quem) (retrato, error)`: `s.admin.Ler(r.Context())` → `recorteDoUsuario` → `visaoPara`. Erro de recorte (`erroDeAlvo`) responde pelo `responderErroDeAlvo`.
  - `contextoDaTela` e `arquivoParaCopiloto` recebem `copiloto.Quem` em vez de `eu`.
  - Rotas: `GET /api/copiloto/estado` e `POST /api/copiloto` com `protegidaEmAlgum(acesso.SkillsLer, …)`.
  - Limite: `s.limiteCopiloto.tirarFicha(a.UsuarioID)`. Log: `registrarConversa(a.UsuarioID, …)`, campo `usuario_id`. Nenhuma leitura de `X-Studio-Papel` no Go.

- [ ] **Step 1: Ajustar a base de teste do Copiloto.** `servidorDoCopiloto` passa a preencher `s.acessos = acessosFake{atribuicoes: usuariosDoCopiloto(), mapeamento: s.admin}`, `s.autenticador = autenticadorDev{}`, `s.authMode = authModeDev`, com:

```go
func usuariosDoCopiloto() map[string][]acesso.Atribuicao {
	return map[string][]acesso.Atribuicao{
		"construtor": {{PapelID: "construtor_superlogica", Escopo: acesso.Escopo{Tipo: acesso.TipoSuperlogica}}},
		"gestor":     naAdministradora("gestor_administradora", "adm-exemplo"),
	}
}
```
`handlerDoCopiloto(s)` passa a devolver `s.rotear(nil)`. `perguntar` e o helper da linha ~452 mandam `X-Dev-Usuario: <usuario>` no lugar de `X-Studio-Papel`. Os testes existentes trocam o argumento `"construtor"`/`"gestor"` pelo id equivalente (os nomes acima foram escolhidos para isso).

- [ ] **Step 2: Escrever os testes novos (falham)**

```go
func TestCopilotoIgnoraCabecalhoDePapel(t *testing.T) {
	falso := &copilotoFalso{}
	s := servidorDoCopiloto(t, falso)
	h := handlerDoCopiloto(s)
	corpo := corpoJSON(t, map[string]any{"mensagem": "Resuma", "historico": []any{}, "contexto": copiloto.Contexto{Tela: copiloto.TelaCondominios}})
	w := chamar(t, h, pedido{metodo: "POST", caminho: "/api/copiloto", corpo: texto(corpo),
		cabecalhos: map[string]string{"X-Dev-Usuario": "gestor", "X-Studio-Papel": "construtor"}})
	// w.Code == 200
	// strings.Contains(falso.recebeu.Contexto, "Vila Verde") e !strings.Contains(..., "Solar")
	// nenhuma operação de falso.recebeu.Operacoes é exclusiva do construtor
	//   (comparar com operacoesPermitidas(copiloto.TelaCondominios, quemDoCopiloto(<acesso do gestor>, "adm-exemplo")))
}

func TestQuemDoCopiloto(t *testing.T) {
	// tabela: construtor na superlogica, recorte "" → Administradora "", Permissoes contém "skills.editar_fluxo"
	//         construtor na superlogica, recorte "adm-exemplo" → Administradora "" (Superlógica não vira gestor)
	//         gestor em adm-exemplo, recorte "adm-exemplo" → Administradora "adm-exemplo", sem "skills.editar_fluxo"
}

func TestCopilotoLimitaPorUsuarioDoAcesso(t *testing.T) {
	// limite com 1 ficha (novoLimiteDoCopiloto com relógio parado, como TestCopilotoLimitaChamadasPorUsuario):
	// 2º pedido de "gestor" → 429; 1º pedido de "construtor" logo depois → 200
	// (mesmo papel enviado em X-Studio-Papel nos três: a chave é o UsuarioID)
}
```
`TestQuemDoCopiloto` substitui o atual (que recebia `eu`).

Em `TestRotasExigemPermissao`:

```go
{nome: "estado do copiloto", metodo: "GET", url: "/api/copiloto/estado", sem: "sem-papel", com: "operador-a", quer: 200},
{nome: "perguntar ao copiloto", metodo: "POST", url: "/api/copiloto", corpo: `{}`, sem: "sem-papel", com: "operador-a", quer: 503},
```
(`novoAmbienteRotas` não liga o modelo; 503 `COPILOTO_INDISPONIVEL` prova que passou do acesso.)

- [ ] **Step 3: Rodar e ver falhar**

Run: `eval "$GO" sh -c "'go test -p 1 -run \"TestCopiloto|TestQuemDoCopiloto|TestRotasExigemPermissao\" ./cmd/okf-admin/'"`
Expected: FAIL (`quemDoCopiloto` com assinatura velha; retrato com Solar; rotas sem 403).

- [ ] **Step 4: Implementar** as mudanças do bloco Interfaces. Apagar `euDev`, o tipo `eu` continua (é a resposta de `/api/eu`), e apagar `TestEuDev`.

- [ ] **Step 5: Rodar e ver passar**

Run: o do Step 3, depois Go tudo e o race. Conferir `grep -rn "X-Studio-Papel" cmd internal` sem resultado.
Expected: PASS, nenhum `DATA RACE`, grep vazio.

- [ ] **Step 6: Commit**

```bash
git add cmd/okf-admin/copiloto.go cmd/okf-admin/copiloto_test.go cmd/okf-admin/studio.go cmd/okf-admin/studio_test.go cmd/okf-admin/rotas_acesso_test.go
```
```bash
git commit -m "fix: take copilot identity and scope from access"
```

---

### Task 5: Usuários de dev semeados e troca por `X-Dev-Usuario` (Go + front)

**Files:**
- Create: `internal/banco/migrations/20261104120000_usuarios_dev_studio.sql`; Modify: `internal/banco/migrations/atlas.sum`
- Modify: `internal/acesso/postgres.go`, `internal/acesso/postgres_test.go` (`Conceder`)
- Create: `cmd/okf-admin/acesso_dev.go`; Modify: `cmd/okf-admin/main.go` (chamar no boot), `cmd/okf-admin/autenticacao.go`
- Modify: `cmd/okf-admin/rotas_acesso_test.go` (`usuariosTeste`, `TestUsuarioSemeadoSoValeEmModoDev`)
- Modify: `web/studio/src/shared/api/http.ts`, `http.test.ts`, `src/acesso/DevRoleSwitch.tsx`, `DevRoleSwitch.test.tsx`, `src/app/Topbar.tsx`, `README.md` (lista de usuários)

**Interfaces:**
- Consumes: `semearMapeamento` (main), `admin.Repositorio.Ler/SalvarAdministradora`, `ehDesenvolvimento(ambiente)`.
- Produces:
  - Migration (dados): `usuarios` + `atribuicoes` para `construtor-superlogica` (`construtor_superlogica`, `superlogica`, `''`, nome `Construtor Superlógica (dev)`), `gestor-exemplo` (`gestor_administradora`, `administradora`, `adm-exemplo`, `Gestor Administradora exemplo (dev)`), `gestor-avancado` (`gestor_administradora`, `administradora`, `adm-avancado`, `Gestor com Avançado (dev)`). `-- +goose Down` apaga os três. `atlas.sum` regerado com `atlas migrate hash --dir file://internal/banco/migrations` (ou a mesma ferramenta que a main usou para a `20261103170000`).
  - `func (p *Postgres) Conceder(ctx context.Context, c Concessao) error` em `internal/acesso/postgres.go`: idempotente (`OnConflictColumns(administradora_id, papel_id, permissao).DoNothing()` do ent), com `timeoutCarregar`.
  - `cmd/okf-admin/autenticacao.go`: `var usuariosSemeadosDev = map[string]bool{"admin-superlogica": true, "construtor-superlogica": true, "gestor-exemplo": true, "gestor-avancado": true}` lido só (nunca escrito) e `func semeadoDev(id string) bool`. A checagem do middleware vira `semeadoDev(usuarioID) && s.authMode != authModeDev`. `usuarioSemeadoDev` (constante) continua como o padrão de `AUTH_DEV_USUARIO`.
  - `func semearAcessoDev(ctx context.Context, repo admin.Repositorio, conceder func(context.Context, acesso.Concessao) error) error` em `acesso_dev.go`: garante `admin.Administradora{ID: "adm-avancado", Nome: "Administradora com Avançado (dev)"}` (só grava se não existe no `repo.Ler`) e concede `{AdministradoraID: "adm-avancado", PapelID: "gestor_administradora", Permissao: acesso.StudioAvancado}`. Chamado no boot logo depois de `semearMapeamento`, só quando `ehDesenvolvimento(cfg.Ambiente) && authMode == authModeDev`; erro derruba o boot com contexto `"semear acesso de dev: %w"`.
  - Front: `CHAVE_USUARIO_DEV = 'studio.usuarioDev'` (substitui `CHAVE_PAPEL_DEV`); `http.ts` manda `X-Dev-Usuario` com o valor gravado e nunca `X-Studio-Papel`. `DevRoleSwitch({ usuarioAtual }: { usuarioAtual: string })` com as opções:

| valor | rótulo |
|---|---|
| `admin-superlogica` | Superlógica (admin) |
| `construtor-superlogica` | Superlógica (construtor) |
| `gestor-exemplo` | Administradora (gestor) |
| `gestor-avancado` | Administradora com Avançado |

  `Topbar` passa `usuarioAtual={eu.id}`. A troca ainda chama `invalidateQueries()` (a Task 8 troca).

- [ ] **Step 1: Testes Go (falham)**
  - `TestUsuarioSemeadoSoValeEmModoDev` vira tabela de `usuário × modo`: para cada id de `usuariosSemeadosDev`, "modo dev aceita" (200) e "fora de dev recusa" (401). `usuariosTeste()` ganha as três chaves novas com as atribuições da migration.
  - `internal/acesso/postgres_test.go`: `TestUsuariosDevDoStudioSemeados` (depois das migrations, `Carregar` de cada um dos três devolve `PapelPrincipal` esperado: `construtor_superlogica`, `gestor_administradora`, `gestor_administradora`); `TestConcederEIdempotente` (duas chamadas iguais, uma linha em `concessoes`, `PermissoesEmAlgum` do gestor da administradora inclui `studio.avancado`).
  - `cmd/okf-admin/acesso_dev_test.go`: `TestSemearAcessoDev` com `admin.NovaEmMemoria()` e um `conceder` falso que guarda o que recebeu: depois de duas chamadas, `adm-avancado` existe uma vez e o falso recebeu a concessão acima.

- [ ] **Step 2: Rodar e ver falhar**

Run: `eval "$GO" sh -c "'go test -p 1 -run \"TestUsuarioSemeado|TestUsuariosDevDoStudio|TestConceder|TestSemearAcessoDev\" ./cmd/okf-admin/ ./internal/acesso/'"`
Expected: FAIL (fora de dev aceita os novos; `Conceder` e `semearAcessoDev` indefinidos).

- [ ] **Step 3: Implementar** migration, `atlas.sum`, `Conceder`, conjunto, `semearAcessoDev` e a chamada no boot.

- [ ] **Step 4: Rodar e ver passar** (mesmo comando, depois Go tudo). Expected: PASS.

- [ ] **Step 5: Testes do front (falham)**

Em `DevRoleSwitch.test.tsx`, `switchesRoleAndRefetches` vira `switchesDevUserAndSendsHeader`:

```tsx
await user.click(within(screen.getByRole('listbox')).getByText('Administradora (gestor)'));
expect(localStorage.getItem(CHAVE_USUARIO_DEV)).toBe('gestor-exemplo');
await vi.waitFor(() => expect(falso.mock.calls.length).toBeGreaterThan(antes));
const ultima = new Headers(falso.mock.calls.at(-1)?.[1]?.headers);
expect(ultima.get('X-Dev-Usuario')).toBe('gestor-exemplo');
expect(ultima.get('X-Studio-Papel')).toBeNull();
```
Em `http.test.ts`, os casos de `X-Studio-Papel` viram `sendsDevUserHeaderOnEveryCall` (GET, PUT e DELETE levam `X-Dev-Usuario` gravado) e `stopsSendingStudioRoleHeader` (com `localStorage['studio.papelDev'] = 'construtor'` sobrando de antes, nenhum pedido leva `X-Studio-Papel`). `base` do teste usa `id: 'admin-superlogica'`.

- [ ] **Step 6: Rodar e ver falhar**

Run: `eval "$STUDIO" sh -c "'$PNPM && pnpm exec vitest run src/acesso/DevRoleSwitch.test.tsx src/shared/api/http.test.ts'"`
Expected: FAIL (`CHAVE_USUARIO_DEV` não exportado; cabeçalho ainda é `X-Studio-Papel`).

- [ ] **Step 7: Implementar** `http.ts`, `DevRoleSwitch.tsx`, `Topbar.tsx`. Em `estadoFalso.ts`, `papel` dos usuários falsos passa a `admin_superlogica`/`gestor_administradora` e `id` aos ids de dev.

- [ ] **Step 8: Rodar front tudo.** Expected: lint, typecheck e testes verdes; `grep -rn "X-Studio-Papel\|studio.papelDev" web/studio/src` só acha o teste `stopsSendingStudioRoleHeader`.

- [ ] **Step 9: Commit**

```bash
git add internal/banco/migrations internal/acesso cmd/okf-admin web/studio/src README.md
```
```bash
git commit -m "feat: seed studio dev users and switch by X-Dev-Usuario"
```

---

### Task 6: Criar condomínio e administradora com POST (front)

**Files:**
- Modify: `web/studio/src/condominios/contratos.ts`, `contratos.test.ts`
- Modify: `web/studio/src/condominios/NovoCondominio.tsx:47`, `AdministradorasAba.tsx:54`
- Test: `web/studio/src/condominios/AdministradorasAba.test.tsx`, `web/studio/src/condominios/NovoCondominio.test.tsx` (novo)

**Interfaces:**
- Consumes: `apiEnviar`, `ApiError` (`status`, `codigo`), `caminhoAdmin`, `RespostaMapeamentoSchema`.
- Produces em `contratos.ts`:
  - `criarCondominio(condominio: Condominio, escopo: Escopo): Promise<RespostaMapeamento>` → `POST /api/admin/condominio`.
  - `criarAdministradora(administradora: Administradora, escopo: Escopo): Promise<RespostaMapeamento>` → `POST /api/admin/administradora`.
  - `salvarCondominio` e `salvarAdministradora` continuam PUT e só editam (IdentificacaoSecao e renomear administradora não mudam).
  - Conflito: `ApiError` com `codigo === 'JA_EXISTE'` é relançado com a mensagem fixa. Condomínio: `Já existe um condomínio com esse identificador.` Administradora: `Já existe uma administradora com esse identificador.` Outros erros passam intactos. Uma função local `comMensagemDeConflito(mensagem: string): (erro: unknown) => never` usada no `.catch` dos dois.

- [ ] **Step 1: Testes (falham)**

`contratos.test.ts`:

```ts
it.each([
  ['criarCondominio', () => criarCondominio(condominioNovo, null), 'POST /api/admin/condominio'],
  ['criarAdministradora', () => criarAdministradora({ id: 'adm-n', nome: 'N' }, null), 'POST /api/admin/administradora'],
])('%s createsWithPost', async (_nome, chamar, rota) => {
  const falso = responderCom({ [rota]: () => json(respostaVazia, 201) });
  await chamar();
  expect(falso).toHaveBeenCalledTimes(1);
});
```
`NovoCondominio.test.tsx`: `createsCondominioWithPost` (preenche e salva; a rota falsa só tem `POST /api/admin/condominio`; o rodapé mostra "Condomínio criado.") e `showsConflictMessageOnDuplicateCondominio` (POST responde `json({ codigo: 'JA_EXISTE', error: 'condomínio cond-x já existe' }, 409)`; o rodapé mostra `Já existe um condomínio com esse identificador.`).
`AdministradorasAba.test.tsx`: `createsAdministradoraWithPost` e `showsConflictMessageOnDuplicateAdministradora`, iguais, e `renamesAdministradoraWithPut` continua (o renomear usa PUT).

- [ ] **Step 2: Rodar e ver falhar**

Run: `eval "$STUDIO" sh -c "'$PNPM && pnpm exec vitest run src/condominios/contratos.test.ts src/condominios/NovoCondominio.test.tsx src/condominios/AdministradorasAba.test.tsx'"`
Expected: FAIL (`criarCondominio` não exportado; fetch sem rota falsa `PUT /api/admin/condominio`).

- [ ] **Step 3: Implementar** e trocar as duas chamadas de criação.

- [ ] **Step 4: Rodar e ver passar** (mesmo comando, depois front tudo). Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/studio/src/condominios
```
```bash
git commit -m "fix: create condominio and administradora with post"
```

---

### Task 7: Orquestração legível para quem tem o Avançado concedido (Go)

**Files:**
- Modify: `cmd/okf-admin/admin.go` (registro de `GET /api/admin/orquestracao`)
- Modify: `cmd/okf-admin/rotas_acesso_test.go` (`acessosFake.concessoes`, `usuariosTeste`, casos)

**Interfaces:**
- Consumes: `s.protegidaEmAlgum`, `acesso.Concessao`.
- Produces: `GET /api/admin/orquestracao` com `protegidaEmAlgum(acesso.StudioAvancado, s.adminOrquestracao)`; `PUT` continua `protegida(acesso.StudioAvancado, alvoSuperlogica, …)`. `acessosFake` ganha `concessoes []acesso.Concessao`, copiado para o `Acesso` que monta. `novoAmbienteRotas` passa `concessoes: []acesso.Concessao{{AdministradoraID: "adm-av", PapelID: "gestor_administradora", Permissao: acesso.StudioAvancado}}`, e `usuariosTeste()` ganha `"gestor-av": naAdministradora("gestor_administradora", "adm-av")`.

- [ ] **Step 1: Testes (falham)**
  - Em `TestRotasExigemPermissao`, o caso `"ler orquestracao"` muda para `sem: "gestor-a", com: "gestor-av", quer: 200` (o `construtor-a` passa a ler, porque tem `studio.avancado` em `adm-a`).
  - Novo, em `rotas_acesso_test.go`:

```go
func TestOrquestracaoComAvancadoConcedido(t *testing.T) {
	amb := novoAmbienteRotas(t)
	if w := amb.pedir(t, "gestor-av", "GET", "/api/admin/orquestracao", ""); w.Code != http.StatusOK {
		t.Fatalf("leitura: status %d: %s", w.Code, w.Body.String())
	}
	w := amb.pedir(t, "gestor-av", "PUT", "/api/admin/orquestracao", `{}`)
	if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "SEM_PERMISSAO") {
		t.Fatalf("escrita: status %d: %s", w.Code, w.Body.String())
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `eval "$GO" sh -c "'go test -p 1 -run \"TestRotasExigemPermissao|TestOrquestracaoComAvancadoConcedido\" ./cmd/okf-admin/'"`
Expected: FAIL (`gestor-av` recebe 403 na leitura).

- [ ] **Step 3: Implementar** a troca do registro e o campo do fake.

- [ ] **Step 4: Rodar e ver passar** (mesmo comando, depois Go tudo). Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/okf-admin/admin.go cmd/okf-admin/rotas_acesso_test.go
```
```bash
git commit -m "feat: let granted administradoras read orchestration"
```

---

### Task 8: Tela de 401 e troca de usuário sem dados velhos (front)

**Files:**
- Modify: `web/studio/src/acesso/ErroAcesso.tsx`, `GuardaAvancado.tsx`, `app/Sidebar.tsx` (passar o erro)
- Modify: `web/studio/src/shared/api/http.ts`, `http.test.ts` (`deveTentarDeNovo`), `web/studio/src/main.tsx` (retry padrão)
- Modify: `web/studio/src/acesso/DevRoleSwitch.tsx` (`resetQueries`)
- Test: `web/studio/src/acesso/eu.test.tsx`, `DevRoleSwitch.test.tsx`, `ErroAcesso.test.tsx` (novo)

**Interfaces:**
- Consumes: Task 5 `DevRoleSwitch`, `CHAVE_USUARIO_DEV`; `ApiError`.
- Produces:
  - `ErroAcesso({ erro, aoTentar }: { erro: ApiError | null; aoTentar: () => void })`. Com `erro?.status === 401` mostra, em `role="alert"`, `Não reconhecemos seu usuário.` e a linha `Entre de novo para continuar.` sem botão de tentar. Nos outros casos, o texto e o botão de hoje.
  - `export function deveTentarDeNovo(falhas: number, erro: unknown): boolean` em `src/shared/api/http.ts`: `false` para `ApiError` com `status === 401`, senão `falhas < 3`. Usado como `retry` padrão do `QueryClient` de `src/main.tsx`. Fica no padrão global, e não só no `useEu`, porque uma sessão que cai no meio do uso dá 401 em qualquer query. Os testes continuam com `retry: false` (`src/test/render.tsx:21`).
  - `DevRoleSwitch.trocar`: `void cliente.resetQueries()` no lugar de `invalidateQueries()` (decisão 6).

- [ ] **Step 1: Testes (falham)**

```tsx
// ErroAcesso.test.tsx
it('showsUnauthenticatedScreen', () => {
  renderComProvedores(<ErroAcesso erro={new ApiError(401, 'NAO_AUTENTICADO', 'usuário não autenticado', null)} aoTentar={() => {}} />);
  expect(screen.getByRole('alert')).toHaveTextContent('Não reconhecemos seu usuário.');
  expect(screen.queryByRole('button', { name: 'Tentar de novo' })).not.toBeInTheDocument();
});
it('keepsRetryForOtherErrors', () => { /* status 500 → "Não foi possível carregar seu acesso." + botão */ });

// eu.test.tsx
it('showsUnauthenticatedScreenInLayout', async () => {
  responderCom({ 'GET /api/eu': () => json({ codigo: 'NAO_AUTENTICADO', error: 'x' }, 401) });
  renderComProvedores(<StudioLayout />);
  expect(await screen.findByText('Não reconhecemos seu usuário.')).toBeInTheDocument();
});

// http.test.ts
it.each([
  { falhas: 0, erro: new ApiError(401, 'NAO_AUTENTICADO', 'x', null), quer: false },
  { falhas: 0, erro: new ApiError(500, 'ERRO_INTERNO', 'x', null), quer: true },
  { falhas: 3, erro: new ApiError(500, 'ERRO_INTERNO', 'x', null), quer: false },
  { falhas: 0, erro: new Error('rede'), quer: true },
])('doesNotRetryOnUnauthenticated $falhas $erro.message', ({ falhas, erro, quer }) => {
  expect(deveTentarDeNovo(falhas, erro)).toBe(quer);
});

// DevRoleSwitch.test.tsx
it('dropsPreviousUserDataOnSwitch', async () => {
  // GET /api/eu responde admin (nome 'Ana') na 1ª chamada e um gestor (nome 'Bia') a partir da 2ª,
  // com a 2ª resposta presa numa Promise que o teste solta.
  // Depois de escolher 'Administradora (gestor)' e ANTES de soltar a Promise: queryByText('Ana') é null.
  // Depois de soltar: findByText('Bia').
});
```
A regra de retry é testada como função pura, sem esperar o atraso exponencial do TanStack.

- [ ] **Step 2: Rodar e ver falhar**

Run: `eval "$STUDIO" sh -c "'$PNPM && pnpm exec vitest run src/acesso src/shared/api'"`
Expected: FAIL (texto de 401 inexistente; 'Ana' continua na tela durante o refetch).

- [ ] **Step 3: Implementar** e passar `erro` (o `error` do `useEu`) nos dois usos de `ErroAcesso`.

- [ ] **Step 4: Rodar e ver passar** (mesmo comando, depois front tudo). Expected: PASS; `ErroAcesso.tsx` abaixo de 200 linhas e sem hex.

- [ ] **Step 5: Commit**

```bash
git add web/studio/src/acesso web/studio/src/app/Sidebar.tsx web/studio/src/shared/api web/studio/src/main.tsx
```
```bash
git commit -m "feat: show unauthenticated screen and reset cache on switch"
```

---

### Task 9: Contrato Go ↔ Zod por fixtures golden (Go + front)

**Files:**
- Modify: `cmd/okf-admin/contrato_test.go` (casos novos em `TestContratosDoStudio`)
- Create: `cmd/okf-admin/testdata/contrato/{skills,skill-textos,skill-fluxo,estado,copiloto}.json`
- Create: `web/studio/src/test/contrato.ts`, `web/studio/src/contrato.test.ts`

**Interfaces:**
- Consumes: Task 3 `conferirContrato`, `TestContratosDoStudio`; Task 4 base do Copiloto (`copilotoFalso`, `s.copiloto`).
- Produces:
  - Casos Go (todos como `admin`, exceto onde dito): `skills` = `GET /api/skills`; `skill-textos` = `GET /api/skills/agendar-mudanca`; `skill-fluxo` = `GET /api/skills/agendar-mudanca/fluxo`; `estado` = `GET /api/admin/estado?administradora=adm-a` como `gestor-a`; `copiloto` = `POST /api/copiloto` com tela `skills`, com `amb.s.copiloto` ligado a um `copilotoFalso` que devolve uma operação válida para a tela, e `amb.s.limiteCopiloto = novoLimiteDoCopiloto(time.Now)`.
  - `export function lerContrato(nome: string): unknown` em `src/test/contrato.ts`: `JSON.parse(readFileSync(resolve(import.meta.dirname, '../../../../cmd/okf-admin/testdata/contrato', `${nome}.json`), 'utf8'))`.
  - `src/contrato.test.ts`, tabela:

```ts
it.each([
  ['eu', EuSchema],
  ['skills', SkillsSchema],
  ['skill-textos', TextosLidosSchema],
  ['skill-fluxo', FluxoLidoSchema],
  ['estado', EstadoSchema],
  ['copiloto', RespostaDoCopilotoSchema],
] as const)('parsesGoFixture %s', (nome, schema) => {
  const resultado = schema.safeParse(lerContrato(nome));
  expect(resultado.error?.issues ?? []).toEqual([]);
});
```
  (imports: `@/acesso/eu`, `@/skills/contratos`, `@/fluxo/modelo/tipos`, `@/condominios/contratos`, `@/copiloto/contratos`.)

- [ ] **Step 1: Escrever os casos Go e o teste Vitest** (falham: arquivos ausentes).

Run: `eval "$GO" sh -c "'go test -p 1 -run TestContratosDoStudio ./cmd/okf-admin/'"` e `eval "$STUDIO" sh -c "'$PNPM && pnpm exec vitest run src/contrato.test.ts'"`
Expected: Go FAIL `open testdata/contrato/skills.json: no such file`; Vitest FAIL `ENOENT` em `skills.json`.

- [ ] **Step 2: Gerar as fixtures**

Run: `eval "$GO" sh -c "'go test -p 1 -run TestContratosDoStudio ./cmd/okf-admin/ -args -atualizar-contrato'"`
Expected: 6 arquivos em `testdata/contrato/`.

- [ ] **Step 3: Rodar os dois lados e ver passar.** Se um schema recusar a fixture, o defeito é real: corrigir o lado errado (Go ou Zod) nesta task e dizer qual no commit.

- [ ] **Step 4: Provar que o drift quebra.** Trocar temporariamente `json:"administradoraId"` por `json:"administradora_id"` em `studio.go`; rodar Go → FAIL de `conferirContrato` em `eu`; rodar com `-atualizar-contrato` e o Vitest → FAIL `parsesGoFixture eu`. Reverter com `git checkout cmd/okf-admin/studio.go cmd/okf-admin/testdata/contrato/eu.json` e rodar os dois de novo → PASS.

- [ ] **Step 5: Go tudo e front tudo.** Expected: verdes.

- [ ] **Step 6: Commit**

```bash
git add cmd/okf-admin/contrato_test.go cmd/okf-admin/testdata/contrato web/studio/src/test/contrato.ts web/studio/src/contrato.test.ts
```
```bash
git commit -m "test: pin go and zod contracts with golden fixtures"
```

---

### Task 10: Smoke ponta a ponta por papel

**Files:**
- Create: `docs/superpowers/medicoes/2026-11-04-smoke-studio-na-fundacao.md`

**Interfaces:**
- Consumes: tudo das Tasks 1 a 9.
- Produces: checklist preenchido; prints por papel guardados fora do repositório e anexados ao PR quando ele for aberto.

- [ ] **Step 1: Subir**

```bash
./build.sh --test
```
```bash
./build.sh --run
```
Expected: `--test` verde; `--run` sobe Postgres (compose) e `okf-admin` em `127.0.0.1:8090`. Antes, com dados reais de `okf/var`: `./bin/okf-admin.exe importar-json -dir okf/var` imprime contagens iguais por área.

- [ ] **Step 2: Percorrer o checklist** em `/studio/`, trocando o usuário pelo "Ver como":

| Papel (`X-Dev-Usuario`) | Conferir |
|---|---|
| `admin-superlogica` | `/` vai para `/studio/`; cria condomínio (POST, "Condomínio criado."); criar de novo com o mesmo id mostra a frase de 409; habilita uma skill; edita e salva um fluxo; Copiloto responde; `/admin` e `/backdoor` abrem |
| `construtor-superlogica` | edita fluxo; não vê "Administradoras" para editar (sem `administradoras.editar`) |
| `gestor-exemplo` | abre em Condomínios; só vê condomínios de `adm-exemplo`; fluxo só leitura; sem Avançado; Copiloto não cita condomínio de outra administradora |
| `gestor-avancado` | vê Avançado; aba Orquestração lê (200) e salvar recusa (403 legível) |
| usuário inexistente (editar `localStorage['studio.usuarioDev'] = 'fantasma'`) | tela "Não reconhecemos seu usuário." |

Na troca de usuário, nenhum dado do anterior pisca na tela.

- [ ] **Step 3: Registrar** em `docs/superpowers/medicoes/2026-11-04-smoke-studio-na-fundacao.md`: data, commit testado, cada linha da tabela com ok/falha e o que se viu. Falha vira task de correção antes do PR, não nota.

- [ ] **Step 4: Commit**

```bash
git add docs/superpowers/medicoes/2026-11-04-smoke-studio-na-fundacao.md
```
```bash
git commit -m "docs: record studio smoke on fundacao by role"
```

---

## Fora deste plano (pesquisa §6)

- Rebasear `feat/ganchos` depois do merge (3 conflitos triviais) e acrescentar `ganchos.*` ao tipo `Permissao` do front quando houver tela.
- Login de verdade (fora de dev a API continua fechada).
- Catálogo de skills no Postgres antes de mais de uma réplica.
- Paginação do `/api/admin/estado`.
- Papel só de textos (nota b da pesquisa): o front libera persona e arquivos com `skills.editar_textos` e o back exige `skills.editar_fluxo`.
