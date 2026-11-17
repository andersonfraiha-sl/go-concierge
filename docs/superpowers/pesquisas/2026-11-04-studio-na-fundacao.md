# Studio na Fundação: o que falta pro PR #3 entrar na main

Data: 2026-11-04. Pesquisa só de leitura. Comparei `feat/studio-react` (e5fdf26) com `origin/main` (4f267bd, merge da Fundação, PR #2) e com `feat/ganchos` (642f03b). Base comum de Studio e main: 80c15f5. Studio está 161 commits à frente da base, main 71.

Convenção: `studio:` é arquivo na branch do Studio (`go-concierge-wt-studio`), `main:` é arquivo na `origin/main` (o checkout `go-concierge` está em `feat/fundacao-postgres-rbac`, que não difere da main). **Inferência** marca o que deduzi sem ver rodando.

## tltr

- Merge textual: 4 arquivos em conflito (README, build.sh, admin.go, main.go), 15 hunks. Tem também 3 quebras que o git não mostra: rota duplicada que dá panic no boot, `s.admin.Ler()` sem ctx no copiloto e o teste de `/` público.
- Endpoints que o front usa: 36. Funcionam como estão: 22. Precisam adaptar: 3. Faltam na main: 11, que são as rotas do Studio (`/api/eu`, `/api/skills*`, `/api/copiloto*`).
- Auth: o front lê um usuário falso (`X-Studio-Papel`). A main identifica por `X-Dev-Usuario` e devolve permissão por escopo, só que não tem `/api/eu`.
- Libs: não adicionar nenhuma agora. Fica react-router 8, sem TanStack Table nem Virtual. No lugar de codegen, fixtures golden do Go lidas pelos testes Zod.
- 10 tarefas, 4 delas em paralelo logo depois do merge.

## 1. Endpoints

Status: **ok** = a main já tem a rota com a forma e a permissão que o front espera. **adaptar** = existe, mas o front ou o back precisa mudar. **falta** = só existe no Studio, sem proteção. Precisa entrar no `rotear` da main com permissão e teste.

| # | Método e caminho | Onde no Studio | Main / Fundação | Permissão e escopo | Status |
|---|---|---|---|---|---|
| 1 | GET /api/eu | studio:cmd/okf-admin/studio.go:26, stub `euDev` :52-81 | não existe | autenticado, sem permissão | **falta** |
| 2 | GET /api/skills | studio.go:27 | não existe | `skills.ler` em algum (inferência, igual ao catálogo) | **falta** |
| 3 | POST /api/skills | studio.go:28 | não existe | `skills.criar` em superlogica | **falta** |
| 4 | GET /api/skills/{nome} | studio.go:29 | não existe | `skills.ler` em algum | **falta** |
| 5 | PUT /api/skills/{nome}/textos | studio.go:30 | não existe | `skills.editar_textos` em superlogica (inferência: o front só libera com `administradoraId === null`, studio:web/studio/src/acesso/eu.ts:44-46) | **falta** |
| 6 | POST /api/skills/{nome}/duplicar | studio.go:31 | não existe | `skills.criar` em superlogica | **falta** |
| 7 | DELETE /api/skills/{nome} | studio.go:32 | não existe | `skills.excluir` em superlogica | **falta** |
| 8 | GET /api/skills/{nome}/fluxo | studio.go:33 | não existe | `skills.ler` em algum | **falta** |
| 9 | PUT /api/skills/{nome}/fluxo | studio.go:34 | não existe | `skills.editar_fluxo` em superlogica | **falta** |
| 10 | GET /api/copiloto/estado | studio:cmd/okf-admin/copiloto.go:68 | não existe | `skills.ler` em algum | **falta** |
| 11 | POST /api/copiloto | copiloto.go:69, usuário de `euDev` :96 | não existe | `skills.ler` em algum, e o retrato recortado pelo Acesso | **falta** |
| 12 | GET /api/agentes | studio:main.go:255 | main:cmd/okf-admin/main.go:391 | `skills.ler` em algum (main.go:399) | ok |
| 13 | GET /api/agente | main.go:256 | main.go:392 | `skills.ler` em algum | ok |
| 14 | PUT /api/agente/config | main.go:257 | main.go:404 | `skills.editar_fluxo` em superlogica (main.go:411) | ok |
| 15 | POST /api/agentes/importar | main.go:258 | main.go:407 | `skills.editar_fluxo` em superlogica | ok |
| 16 | GET /api/agentes/exportar | main.go:259 | main.go:393 | `skills.ler` em algum | ok (ver nota a) |
| 17 | GET /api/arquivo | main.go:260 | main.go:394 | `skills.ler` em algum | ok |
| 18 | PUT /api/arquivo | main.go:261 | main.go:408 | `skills.editar_fluxo` em superlogica | ok (ver nota b) |
| 19 | DELETE /api/arquivo | main.go:262 | main.go:409 | `skills.editar_fluxo` em superlogica | ok |
| 20 | GET /api/arquivo/modelo | main.go:263 | main.go:395 | `skills.ler` em algum | ok |
| 21 | GET /api/projecao | main.go:264 | main.go:396 | `skills.ler` em algum | ok |
| 22 | GET /api/nexus/recursos | main.go:265 | main.go:397 | `skills.ler` em algum | ok |
| 23 | PUT /api/nexus/recurso | main.go:266 | main.go:415 | `ferramentas.editar` em superlogica (main.go:419) | ok |
| 24 | DELETE /api/nexus/recurso | main.go:267 | main.go:416 | `ferramentas.editar` em superlogica | ok |
| 25 | POST /api/agente/recursos | main.go:268 | main.go:417 | `ferramentas.editar` em superlogica | ok |
| 26 | GET /api/admin/orquestracao | studio:admin.go:31 | main:cmd/okf-admin/admin.go:69 | `studio.avancado` em superlogica | **adaptar** (nota c) |
| 27 | PUT /api/admin/orquestracao | admin.go:32 | main:admin.go:70 | `studio.avancado` em superlogica | ok |
| 28 | GET /api/admin/estado | admin.go:33 | main:admin.go:72, recorte em rotas_acesso.go:293-326 | `skills.ler` em algum, resposta recortada | ok (nota d) |
| 29 | PUT /api/admin/administradora (criar e editar) | admin.go:34, upsert | main:admin.go:74-75: POST cria (201, 409 `JA_EXISTE`), PUT só edita (404) | `administradoras.editar` em superlogica | **adaptar** (nota e) |
| 30 | DELETE /api/admin/administradora | admin.go:35 | main:admin.go:76 | `administradoras.editar` em superlogica | ok |
| 31 | PUT /api/admin/condominio (criar e editar) | admin.go:36, upsert | main:admin.go:79-80: POST cria, PUT edita | `condominios.editar` na administradora do corpo e na dona atual (admin.go:254) | **adaptar** (nota e) |
| 32 | DELETE /api/admin/condominio | admin.go:37 | main:admin.go:81 | `condominios.editar` no condomínio do `?id=` | ok |
| 33 | PUT /api/admin/habilitacao | admin.go:38 | main:admin.go:83 | `condominios.habilitar` no condomínio do corpo (`*` = superlogica) | ok |
| 34 | PUT /api/admin/orientacao | admin.go:40 | main:admin.go:84 | idem | ok |
| 35 | PUT /api/admin/ajustes | admin.go:42 | main:admin.go:85 | idem | ok |
| 36 | POST /api/admin/sugerir-payload | admin.go:41 | main:admin.go:86 | idem | ok |

Total: 22 ok, 3 adaptar, 11 falta. `PUT /api/admin/payload` existe nas duas, mas o front não chama `salvarPayload` em lugar nenhum.

Notas:

- **a.** O exportar é um `<a href>` (studio:web/studio/src/avancado/ImportarExportarAba.tsx:25), então não manda cabeçalho. Em dev só funciona como o `AUTH_DEV_USUARIO` padrão. Com cookie de sessão de verdade, funciona. Não bloqueia o merge.
- **b.** O front libera a edição de persona e arquivos com `skills.editar_textos` (DocumentoEditavel, ConciergePage:129), mas a main exige `skills.editar_fluxo`. Hoje não quebra, porque quem tem `editar_textos` em superlogica (construtor e admin) também tem `editar_fluxo` (main:internal/acesso/acesso.go:80-83). Se alguém criar um papel só de textos, quebra. Fica registrado, sem tarefa.
- **c.** O gestor com a concessão do Avançado vê a aba Orquestração (`pode(eu,'studio.avancado')` em studio:src/acesso/GuardaAvancado.tsx:10). A concessão vale dentro da administradora (acesso.go:152-156), não em superlogica, então o GET responde 403. Decidi que a leitura vira `protegidaEmAlgum(StudioAvancado)` e a escrita continua em superlogica. É o mesmo padrão das outras leituras do catálogo.
- **d.** No merge, `casoDeUso.resumo` e `icone` (studio:admin.go, diff contra a base) têm de continuar. O Zod tem default `''` (studio:src/condominios/contratos.ts:59-61), então não quebra, mas a tela perde o resumo e o ícone.
- **e.** `salvarCondominio` e `salvarAdministradora` usam PUT pra criar (studio:src/condominios/contratos.ts:93-129, chamados em NovoCondominio.tsx:47 e AdministradorasAba.tsx:54). Na main, criar com PUT dá 404 `NAO_ENCONTRADO`. Precisa de `criarCondominio` e `criarAdministradora` com POST. O 409 `JA_EXISTE` já sai legível: o `http.ts` lê `codigo` e `error` (studio:src/shared/api/http.ts:24-28).

O formato de erro da main é `{codigo, error}` (main:autenticacao.go:93-97, rotas_acesso.go:109) e o do Studio é `{codigo, mensagem}`. O cliente aceita os dois, então não precisa adaptar nada.

## 2. Fontes de dados

| Feature do Studio | Hoje na branch | Na main | O que fica |
|---|---|---|---|
| Condomínios, administradoras, habilitação, orientação, ajustes | `*admin.Store` em `mapeamento.json` (studio:main.go:167, `ADMIN_DIR`) | `admin.Repositorio` no Postgres. O turno lê pelo `admin.Cache`, o Admin e o RBAC leem direto do banco (main:main.go, hunk do merge; internal/admin/postgres.go, cache.go) | Postgres. Fica a versão da main. |
| Copiloto, retrato dos condomínios | `s.admin.Ler()` sem ctx no arquivo (studio:copiloto.go:243), recorte por `Visao(quem.Administradora)` | `Ler(ctx)` e o recorte por `recorteDoUsuario` e `visaoPara` (main:rotas_acesso.go:293-326) | Postgres, recortado pelo Acesso. |
| Seed de condomínio vazio no boot | `s.admin.Ler()` (studio:main.go:168) | some, porque o import vem pelo `importar-json` | Apagar. |
| Usuário e permissões (`/api/eu`) | fixo em código, por papel (studio:studio.go:52-72) | `usuarios`, `atribuicoes`, `papeis`, `concessoes` no Postgres, carregados por `acesso.Postgres.Carregar` (main:internal/acesso/postgres.go:33) | Postgres. |
| Papel de dev | `localStorage['studio.papelDev']` mandado como `X-Studio-Papel` (studio:src/shared/api/http.ts:116-131) | `X-Dev-Usuario` só com `AUTH_MODE=dev` (main:autenticacao.go:30-42) | Cabeçalho da main e usuários de dev semeados. |
| Skills: textos, fluxo, criar, duplicar, excluir | YAML e MD em `okf/agentes`, versão = sha256 do conteúdo (studio:internal/okf/skills.go:64-71, :159) | igual, o catálogo continua em arquivo (main:docs/superpowers/specs/2026-11-03-fundacao-design.md:28 e :204; studio:docs/superpowers/specs/2026-11-03-studio-design.md:31) | **Arquivo, por decisão registrada.** Não bloqueia o merge. |
| Persona, arquivos, `agent.yaml`, orquestração | arquivo (`okf`), orquestração no `agent.yaml` (main:admin.go:469-487) | igual | Arquivo. |
| Recursos do Nexus (fixtures) | arquivo, `okf.SalvarRecursoFixture` (main:main.go:480) | igual | Arquivo. |
| Limite do copiloto | em memória, por processo (studio:copiloto_limite.go:55) | não existe | Fica em memória, mas a chave passa a ser `Acesso.UsuarioID`. |

Sobre o catálogo em arquivo: o Postgres não substitui nada aqui no PR #3, porque as duas specs adiam essa decisão. O risco é de deploy. **Inferência:** com duas réplicas, cada uma grava no próprio disco, e a checagem de versão por sha256 só protege dentro de um processo. Enquanto houver uma réplica, isso está ok. Antes de escalar, o catálogo precisa ir pro banco, e isso pede um projeto próprio.

## 3. Auth e sessão

**Como o front sabe hoje.** `useEu` faz `GET /api/eu` (studio:src/acesso/eu.ts:35-37) e recebe `{id, nome, papel, permissoes: string[], administradoraId: string|null, dev}` (eu.ts:21-28). As permissões são uma lista plana. `pode` é um `includes` (eu.ts:39-41). `podeNoCatalogo` exige `administradoraId === null` (eu.ts:44-46). `usePodeEditar` também bloqueia tela estreita (eu.ts:48-58). O escopo de cada chamada do Admin é `?administradora=<administradoraId>` (studio:src/condominios/contratos.ts:86-91, 143-146). O papel vem do `DevRoleSwitch`, que grava no localStorage e invalida todas as queries (studio:src/acesso/DevRoleSwitch.tsx).

**Como a Fundação expõe.** O middleware `autenticado` resolve o usuário pelo `Autenticador` e carrega o `acesso.Acesso` dele (main:autenticacao.go:71-90). Em `AUTH_MODE=dev`, o usuário vem de `X-Dev-Usuario` ou de `AUTH_DEV_USUARIO`. Fora de dev a API fica fechada (`autenticadorFechado`, autenticacao.go:46-55), porque ainda não existe login. O `Acesso` é escopado: atribuições por escopo (superlogica, administradora ou condomínio), herança de papéis e concessões por administradora (main:internal/acesso/acesso.go:43-73). Cada rota confere `Permitido(a, p, alvo)` ou `PermitidoEmAlgum` (rotas_acesso.go:47-89). Só existe o usuário semeado `admin-superlogica`, e ele só vale em dev (main:internal/banco/migrations/20261103170000_usuario_dev.sql, autenticacao.go:74).

**Gap:**

1. Não tem `/api/eu` na main. Precisa montar a resposta a partir do `Acesso`:
   - `permissoes`: cada `Permissao` com `PermitidoEmAlgum`.
   - `administradoraId`: `null` se `Permitido(SkillsLer, superlogica)`. Senão, a única administradora visível.
   - `papel`: o papel da atribuição de maior nível.
   - `dev`: `authMode == dev`.

   A forma que o front espera continua igual, então o front não muda.
2. O `Acesso` não traz `nome`. A tabela `usuarios` tem (main:internal/banco/ent/schema/usuario.go:15-18). Basta uma query pelo id, ou levar o nome no `Carregar`.
3. Usuário com mais de uma administradora: o `recorteDoUsuario` devolve 400 `ADMINISTRADORA_OBRIGATORIA` se nenhuma vier pedida (rotas_acesso.go:306-308), e o front só conhece uma. Hoje nenhum papel semeado tem duas. Fica a regra de que `/api/eu` responde 409 nesse caso, com o código explícito, e o seletor só entra quando existir o caso real (YAGNI).
4. A lista plana de permissões perde o escopo. Pra tela isso basta: ela só mostra o que o `estado` recortado devolve, e o back recusa o resto. Não vale mandar a matriz por escopo pro front.
5. O dev switch troca `X-Studio-Papel` por `X-Dev-Usuario`. Pra continuar vendo como gestor, precisa de usuários de dev semeados (gestor de uma administradora de exemplo, e outro com a concessão do Avançado). A guarda de `usuarioSemeadoDev` (autenticacao.go:22, :74) passa a ser um conjunto.
6. O copiloto usa `euDev` pra saber quem pergunta, o escopo e a chave do limite (studio:copiloto.go:88, :96, :127, :151). Precisa ir tudo pro `Acesso`, senão um gestor manda `X-Studio-Papel: construtor` e o copiloto mostra condomínios de todas as administradoras. Isso é **bloqueante de segurança**.
7. O front não trata 401: não tem tela pra `NAO_AUTENTICADO`. E, ao trocar de usuário, ele invalida o cache em vez de limpar, então dados do usuário anterior aparecem até o refetch. Trocar `invalidateQueries()` por `clear()` no switch.

## 4. Conflitos de merge (dry run)

Fiz num clone temporário no scratchpad, com `git merge --no-commit` e depois `merge --abort`. Nenhuma branch foi alterada.

**`feat/studio-react` contra `origin/main`:** 4 arquivos em conflito, 15 hunks.

| Arquivo | Hunks | Natureza | Resolução |
|---|---|---|---|
| cmd/okf-admin/main.go | 6 | struct `servidor` (Studio: `studio`, `copiloto`, `limiteCopiloto`; main: `admin.Repositorio`, implantação, autenticador, acessos), literal do `servidor`, boot do mapeamento (cache da main contra copiloto do Studio), registro das rotas (Studio monta o mux em main.go, a main monta em `rotear`), `rotas()` (Studio sem proteção, main com `protegida`) | Fica a main inteira. Somar os 3 campos e o `NovoCopiloto`. `rotasStudio` e `rotasCopiloto` entram em `rotear` (main:rotas_acesso.go:28-41). |
| cmd/okf-admin/admin.go | 2 | registro das rotas do Admin (Studio: `telaAntiga` e `GET /admin`; main: `protegida` e POST de criação) e `responderMapeamento` (Studio) contra `escrever` (main) | Fica a main. Somar `telaAntiga`, `GET /admin` e os campos `resumo`/`icone` do `casoDeUso`. |
| build.sh | 5 | Studio adiciona pnpm e lint do front e troca a imagem Go pra trixie. Main adiciona Docker do host, `-p 1` e compose do Postgres no `--run` | União dos dois. |
| README.md | 2 | seção de rodar e de áreas | União. Aproveitar e fechar o item 4 da revisão final da Fundação (seção Acesso). |

Quebras que o git não acusa (vi no resultado do merge):

1. **Panic no boot por rota duplicada.** `rotasLegado` registra `GET /api/anatomia`, `POST /api/agentes`, `DELETE /api/agente`, `POST /api/agentes/mover`, `GET /backdoor` e `GET /backdoor/orquestracao` sem proteção (studio:cmd/okf-admin/legado.go:14-20). A main registra as mesmas rotas protegidas (main:main.go:390-406, admin.go:29-59). O `ServeMux` dá panic com padrão duplicado. Resolução: tirar o `rotasLegado` e manter só os handlers, que a main referencia.
2. **Não compila.** `copiloto.go:243` chama `s.admin.Ler()`, mas o `Repositorio` pede `Ler(ctx)`. E `copiloto_test.go:42` usa `admin.NewStore`, que trocar por `admin.NovaEmMemoria()` (main:internal/admin/fake.go:18).
3. **Teste da main quebra.** `TestRotasAutenticacao` espera 200 em `/` (main:cmd/okf-admin/rotas_acesso_test.go, caso "página do Admin é pública"). O Studio redireciona `/` pra `/studio/` com 302 (studio:main.go:252). A tela antiga de Admin passa a morar em `GET /admin`. Atualizar o caso.

**`feat/studio-react` contra `feat/ganchos`:** os mesmos 4 arquivos e os mesmos 15 hunks. Ganchos não acrescenta conflito textual além dos que já vêm da main. Ganchos parte da main (base 4f267bd, 74 commits à frente, 0 atrás) e mexe em `main.go` (+68/-9), `rotas_acesso.go` (+1, `s.rotasGanchos(mux)` na linha 33), `config.go` e `internal/acesso/acesso.go` (`ganchos.editar` e `ganchos.historico`).

**Inferência**, pra quem mergear depois do Studio: o rebase de ganchos vai conflitar em 3 pontos triviais.

- `rotear`: `rotasStudio` e `rotasCopiloto` ficam do lado da linha `rotasGanchos`.
- Struct `servidor`: `regras` e `segredoExportacao` entram no mesmo bloco dos campos do Studio.
- Literal do `servidor`.

Tem ainda um ponto semântico: o `/api/eu` vai listar `ganchos.*` em `permissoes`. O Zod aceita, porque é `z.array(z.string())`, mas o tipo `Permissao` do front (studio:src/acesso/eu.ts:6-17) não conhece essas permissões. Só precisa entrar quando houver tela de ganchos. Ordem que recomendo: Studio entra primeiro (PR mais antigo, conflito maior), ganchos rebaseia depois.

## 5. Libs do front

O que já está instalado: TanStack Query 5.105, react-router 8.5, Zod 4.6, react-hook-form, MUI 9 e React Flow (studio:web/studio/package.json).

- **TanStack Router contra react-router 8: não migrar.** São 8 rotas num `createBrowserRouter` só (studio:src/app/router.tsx:34-55), sem loader nem action. Search params aparecem só pra `?aba=` em 5 arquivos (13 usos de `useParams`/`useSearchParams`). O ganho do TanStack Router seria tipar search params e loaders, e nenhum dos dois dói hoje. Migrar mexe em todas as telas e não fecha nenhum gap do merge. Vale reabrir se as telas começarem a carregar dados no loader ou a ter filtros longos na URL.
- **TanStack Table / Virtual: não agora.** `TabelaDeCondominios` filtra em memória sobre `estado.mapeamento.condominios` (studio:src/condominios/TabelaDeCondominios.tsx:73-76), e a matriz é condomínio × skill. O gargalo real, se aparecer, é o `GET /api/admin/estado`, que devolve o mapeamento inteiro com habilitações e payloads (main:admin.go:157-187). Virtualizar a tabela não diminui o JSON. **Inferência:** quando a Superlógica passar de umas centenas de condomínios, o certo é paginar e filtrar no servidor (sqlc com LIMIT), usando o `TablePagination` do MUI que já está instalado. Só depois disso pensar em virtualização.
- **Gerar tipos do Go no lugar do Zod escrito à mão: não, mas fechar o drift com fixtures golden.** São 68 `z.object` à mão (avancado 6, catalogo 16, condominios 9, copiloto 14, skills 6, fluxo 16, fluxoDaSkill 1). O Go não tem OpenAPI. Gerar tipos (tygo ou OpenAPI) traz toolchain nova e não dispensa o Zod, que faz parse em runtime com regra própria (`listaDoGo` converte o slice nil, studio:src/shared/api/listaDoGo.ts:3-8; `mapaDoGo`; `z.json()`). O que paga é contrato por fixture: o teste Go grava em `testdata/contrato/*.json` a resposta real do handler, e o vitest faz parse desse arquivo com o schema. O padrão já existe à mão em `skills/abas/fixturesDeAbas.ts:24` ("no formato que o Go devolve"). O custo é um helper de cada lado, e o drift quebra o teste dos dois lados.
- **TanStack Query: já atende.** Só ajustar o uso: `clear()` na troca de usuário, e uma fábrica de chaves por recurso. Hoje convivem `['eu']`, `['skills']`, `['avancado', ...]`, `['estado-condominios', escopo]`, `['agente']`, `['arquivo']` e `['ferramentas']` (studio:src/catalogo/contratos.ts:124-138, avancado/contratos.ts:434-627). A fábrica só faz sentido se a chave passar a levar o usuário. Como o `clear()` resolve, ela fica de fora.

Recomendação: nenhuma lib nova. Só as fixtures golden (tarefa 9).

## 6. Tarefas

Cada uma termina com teste verde e commit próprio. Skills de cada brief: `code-quality` com `go` ou `react`. Em rota, também `api-design`.

| # | Tarefa | Entrega de teste | Depende de |
|---|---|---|---|
| 1 | Mergear `origin/main` em `feat/studio-react`. Resolver os 4 conflitos como na seção 4. Tirar o `rotasLegado`, colocar `rotasStudio` e `rotasCopiloto` em `rotear`, ajustar `copiloto.go:243` pra `Ler(ctx)` com `visaoPara`, trocar o fake no `copiloto_test`, atualizar o caso `/` do `TestRotasAutenticacao`. Nesta tarefa as rotas do Studio ainda ficam sem proteção. | `go build ./... && go vet ./... && go test -p 1 ./...` verde. Teste novo: `rotear(nil)` não dá panic e `GET /` responde 302 pra `/studio/`. `pnpm test` verde. | nada |
| 2 | Proteger as 9 rotas de skills (tabela, linhas 2 a 9) com `protegida` e `protegidaEmAlgum`. | Linhas novas em `TestRotasExigemPermissao`, com sem e com permissão por rota (`operador-a` lê, `construtor-a` não escreve no catálogo, `admin` escreve). | 1 |
| 3 | `GET /api/eu` a partir do `Acesso`, com nome da tabela `usuarios`, permissões por `PermitidoEmAlgum`, `administradoraId` e 409 pra mais de uma administradora. Apagar `euDev`. | Table-driven por usuário (admin, construtor-a, gestor-a, gestor com concessão, sem papel) com o JSON esperado. A mesma resposta gravada como fixture golden. | 1 |
| 4 | Copiloto no `Acesso`: quem pergunta, recorte por `recorteDoUsuario` e `visaoPara`, limite por `UsuarioID`, log sem `X-Studio-Papel`. Proteger as 2 rotas (linhas 10 e 11). | Teste: gestor-a manda `X-Studio-Papel: construtor` e o retrato só traz condomínios de A. Linhas em `TestRotasExigemPermissao`. | 1, 3 |
| 5 | Usuários de dev: migration de dados com gestor de `adm-exemplo` e gestor com concessão do Avançado, guarda de semeados vira conjunto. `DevRoleSwitch` manda `X-Dev-Usuario` e `http.ts` deixa de mandar `X-Studio-Papel`. | Go: `TestUsuarioSemeadoSoValeEmModoDev` cobre os novos. Vitest: o switch grava o usuário e o cabeçalho sai em toda chamada. | 3 |
| 6 | Front: `criarCondominio` e `criarAdministradora` com POST, PUT só pra editar. Mensagem do 409 `JA_EXISTE`. | Vitest de NovoCondominio e AdministradorasAba: método e caminho certos, e o 409 aparece no snackbar. | 1 |
| 7 | Leitura da orquestração pra quem tem a concessão do Avançado: GET vira `protegidaEmAlgum(StudioAvancado)`, PUT continua em superlogica. | Linha em `TestRotasExigemPermissao`: gestor com concessão lê (200) e não grava (403). | 1 |
| 8 | Front: tela pra `NAO_AUTENTICADO` (401) no `ErroAcesso`, e `queryClient.clear()` na troca de usuário. | Vitest: 401 no `/api/eu` mostra a tela, e a troca não deixa cache do usuário anterior. | 5 |
| 9 | Contrato por fixtures golden: helper Go grava `testdata/contrato/<rota>.json` a partir do handler real (eu, skills, textos, fluxo, estado, copiloto). Helper vitest faz parse de cada um com o schema. | Os dois testes. Mudar um campo no Go quebra o vitest. | 3, 4 |
| 10 | Smoke ponta a ponta: `./build.sh --run` com o Postgres do compose, `importar-json`, Studio em `/studio/` como admin, construtor, gestor e gestor com Avançado. Checklist das telas: criar condomínio, habilitar skill, editar fluxo, copiloto. | Checklist registrado no PR, com print por papel. | 2 a 9 |

Paralelismo:

- **Onda 1:** tarefa 1, sozinha.
- **Onda 2, em paralelo:** 2, 3, 6 e 7. Arquivos disjuntos, com uma exceção: 2 e 7 tocam `rotas_acesso_test.go` em linhas diferentes da mesma tabela. Fazer em sequência dentro da onda, ou aceitar um conflito trivial.
- **Onda 3, em paralelo:** 4 e 5, que dependem do 3.
- **Onda 4:** 8, depois do 5, e 9, depois do 3 e do 4. Os dois em paralelo.
- **Onda 5:** 10.

Fora do PR #3:

- Rebasear `feat/ganchos` depois do merge (3 conflitos triviais, seção 4).
- Login de verdade, porque fora de dev a API continua fechada.
- Catálogo no Postgres antes de rodar mais de uma réplica.
- Paginação do `/api/admin/estado` quando o número de condomínios pedir.
