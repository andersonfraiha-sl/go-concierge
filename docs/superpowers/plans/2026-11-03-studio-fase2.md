# Studio Fase 2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fechar todos os itens P0 e P1 da análise de lacunas do Studio (resumo separado do "Quando usar", lista densa "O que o concierge faz aqui" com drawer, matriz skills × condomínios, ajustes e orientações legíveis, administradoras, identificação, ferramenta completa, Avançado), aplicar o desenho aprovado do canvas "Concierge Studio", entregar o Copiloto em todas as telas e, com a checklist de capacidades fechada, aposentar o Admin e o Backdoor antigos.

**Architecture:** O Go ganha um campo de resumo para pessoas (`summary:` no `caso.yaml`, `resumo:` no `agent.yaml`), derivado da descrição quando vazio, e volta a servir as telas antigas por enquanto (`cmd/okf-admin/legado.go`). O front em `web/studio/src` troca a seção de skills do condomínio por uma lista densa com drawer lateral que concentra papéis, orientação e ajustes da skill; o resto da ficha fica só com o que é geral. A lógica de lista (categoria, busca, contagem, origem da decisão) é pura e testada em `condominios/listaDeSkills.ts`. O Copiloto tem um núcleo puro em Go (`internal/copiloto`: operações tipadas, quais valem em cada tela, permissão de cada uma, validação com as mesmas regras da edição manual), uma borda em `internal/harness/copiloto.go` (o modelo do provedor de sempre) e `POST /api/copiloto`; no front, um painel à esquerda do conteúdo que só muda algo quando a pessoa clica em Aplicar, usando as mesmas funções de gravação das telas.

**Tech Stack:** Go 1.27 (net/http, gopkg.in/yaml.v3, cloudwego/eino via `internal/harness`), React 19.3, MUI 9.5, TanStack Query 5, React Router 8, Zod 4, Vitest 5 + Testing Library.

**Spec:** `docs/superpowers/analises/2026-11-03-studio-lacunas.md` (fonte dos itens P0/P1; seção §7) · Desenho aprovado: canvas "Concierge Studio" (https://claude.ai/artifact/28eSkdhr5PCpVhujtuexQ4; os tokens e o layout que valem estão nas Global Constraints e na Task 0) · Referência do Copiloto: `C:\Users\felip\Desktop\Superlogica\echo-atende-frontend-ng-studio\src\features\bots\components\copilot` (`CopilotPanel`, `MessageBubble`, `CopilotComposer`, `CopilotEmptyState`, `ProposalsDialog`) e `…\features\bots\lib\ai` (`CopilotPort.ts`, `httpCopilot.ts`, `history.ts`) · Spec original: `docs/superpowers/specs/2026-11-03-studio-design.md` · Plano original: `docs/superpowers/plans/2026-11-03-studio.md` · Constituição: `docs/superpowers/constitution.md`

Os caminhos da análise apontam para `studio/src/...` da branch `feat/studio-react`; aqui tudo está em `web/studio/src/...` (branch `sdd/studio-t16`).

## Decisões do Felipe (2026-11-03)

1. O robô se chama **Concierge** em todo o Studio. Sai "Assistente": item do menu, página, textos ("o concierge", nunca "a assistente"). A aba continua "Persona".
2. Fase 2 = todos os itens **P0 + P1** da análise (§7, itens 1 a 21). P2 fica fora.
3. O Admin antigo (`web/okf/index.html` + `admin.js`) e as páginas do Backdoor continuam servidos até a fase 2 cobrir cada capacidade. A última task (Task 31) roda a checklist de capacidades e só então os apaga.
4. O visual segue o canvas aprovado "Concierge Studio" (Skills, Skill › Fluxo com painel lateral, Skill › Copiloto aberto, Condomínio com lista compacta + drawer, Avançado). Tokens nas Global Constraints; base na Task 0.
5. O **Copiloto** (era o projeto 4) entra na fase 2, em **todas** as telas (Skills, editor de skill, Fluxo, Condomínios e ficha, Concierge/Persona, Ferramentas, Avançado): botão no header (alterna, `aria-pressed`), painel à esquerda do conteúdo, contexto da tela atual. Conversa, explica e **propõe** mudanças em cartões com Aplicar / Descartar; nada muda sem Aplicar; toda proposta respeita as permissões e o escopo de quem usa (o gestor só recebe o que pode aplicar) e nunca passa por cima de "Gravar precisa de Confirmar". Tasks 23 a 28.

## Global Constraints

- **As telas aprovadas em `docs/superpowers/design/studio-fase2` são a referência visual; tokens e textos delas valem sobre o plano em caso de conflito.** Cada task de UI abaixo nomeia a(s) tela(s) (`*.dc.html`) que deve conferir antes de implementar; `canvas.json` tem o título de cada uma.
- O front segue, em paralelo, as regras do Atende que estão sendo adicionadas ao projeto: Biome (lint/format), `react-hook-form` + `zodResolver` em todo formulário, componentes com menos de 200 linhas, rotas com `lazy()`, zero cor hexadecimal fora de `tokens.ts`, e Zod em toda borda de API (`contratos.ts` de cada módulo).
- Valem todas as Global Constraints de `docs/superpowers/plans/2026-11-03-studio.md`, com três trocas: (a) tipografia e visual pelo desenho aprovado (abaixo), não Lato; (b) no vocabulário, "Assistente" vira "Concierge"; (c) a aba "Projeção" do Avançado passa a se chamar "Prompt montado".
- **Desenho aprovado (tokens exatos, em `web/studio/src/theme/tokens.ts`, Task 0):**
  - Espaçamento em proporção áurea: 8 / 13 / 21 / 34 / 55 px (`theme.spacing(1..5)`).
  - Tipos: 11 / 12 / 13 / 14 / 15 / 17 / 21 / 26 px. Títulos e números em `"Paralucent", "Plus Jakarta Sans", system-ui, sans-serif` peso 700 (o Paralucent não é carregado; o que aparece é o Plus Jakarta Sans do canvas); corpo `"Argumentum", "Inter", system-ui, sans-serif` pesos 400/500/600; `"JetBrains Mono", ui-monospace, monospace` só para arquivo e código no Avançado.
  - Cores: primária `#1034F2`, primária hover `#0A24B8`, navy `#071950` (títulos, Copiloto); texto `#22252B` / `#323B49` / `#536379` / `#637792` / `#7789A3`; bordas `#E4E7EC` / `#DBDFE5` / `#EDEFF2`; fundo `#F5F6F8`; superfície `#FFFFFF`; sutil `#F8F9FB`; tint `#EEF1FE`; sucesso `#1B993B` em `#E8F5EC`; aviso texto `#8A5A00` em `#FFF6DF`; perigo `#D61D1E`; realce de campo aplicado pelo Copiloto `#FFF7CC`; conectores do fluxo `#C1CAD6`.
  - Raios 8 / 9 / 10 / 12 / 13; cartões raio 13, borda 1 px, **sem sombra**. Só o bloco em edição tem `0 8px 21px rgba(16,52,242,.12)`.
  - Header 58 px branco com borda inferior; sidebar 236 px; item do menu 40 px de altura, raio 10, ativo = fundo tint + texto primário peso 600.
  - Caixa de frase em tudo (nada de abas em maiúsculas); só os sobretítulos de seção usam 11 px maiúsculo com `letter-spacing: .06em`. Abas são texto sublinhado, com 26 px entre elas.
  - Switch 40 × 22. Painel do Copiloto 360 a 400 px à esquerda do conteúdo; drawers e painel do fluxo 380 a 420 px à direita.
- Skills obrigatórias de quem implementa: `code-quality` + `go` (tasks Go); `code-quality` + `typescript` + `react` (tasks de front); + `frontend-design` em toda task que mexe em tela. `code-quality` prevalece sobre precedente local em código novo.
- Vocabulário: fora de `src/avancado/`, proibidos Backdoor, Admin, caso de uso, subagente, operador, orquestrador, roteador, projeção, orquestração, `caso.yaml`, `principal.md`, `persona.md`, `rules/`, `knowledge/`, Nexus, MCP, YAML (teste `src/vocabulario.test.ts`). Em **todo** `.tsx`, inclusive Avançado: proibido "Assistente(s)" e "a/da/na/pela/à assistente" (Task 3). Valores técnicos (`'MCP'`, caminhos) só em `.ts`.
- Resumo: uma linha, até 90 caracteres, PT-BR simples, sem termo técnico. Nunca usar `agent.yaml descricao` como resumo.
- TypeScript strict, sem `any`, sem `as` fora de teste, sem `!`. Go: erro com contexto, nada engolido, sem estado global mutável.
- Somente leitura abaixo de 900 px para qualquer papel (já em `usePodeEditar*`). Salvamento automático de 1000 ms via `shared/autosave/useAutosave.ts`.
- Testes: Go `go test ./internal/okf/ ./cmd/okf-admin/`; front `cd web/studio && npx vitest run <arquivo>` e, ao fim de cada task, `npm run typecheck && npm test`. Nome de teste em verbo de 3ª pessoa, sem "should".
- Commits: conventional sem ticket (`feat: …`, `fix: …`, `test: …`, `chore: …`), descrição minúscula, até 72 caracteres, um por task no mínimo, `git commit` em comando próprio, sem heredoc, nunca mencionar IA. Commit (e push, se a branch tiver upstream) assim que a task fecha.

## Review Focus

1. **Ajustes gravados por dois lugares da mesma ficha** (drawer da skill e "Ajustes gerais"): `PUT /api/admin/ajustes` troca o mapa inteiro do condomínio; cada lado precisa mesclar o que está no cache na hora de gravar, ou um apaga o outro. → Task 6 `savesSkillAdjustmentsKeepingOthers`, Task 7 `keepsSkillAdjustmentsWhenSaving`.
2. **Orientação geral e orientação da skill gravadas por componentes diferentes**: `PUT /api/admin/orientacao` leva `geral` e `por_caso` juntos. → Task 6 `savesSkillGuidanceKeepingGeneral`, Task 8 `savesGuidanceOneSecondAfterTyping` (corpo mantém `por_caso`).
3. **Skill sem resumo** (skill nova, caso sem `summary`, skill sem caso): com caso, deriva da descrição; sem caso, fica vazio, nunca o texto de classificador do agente. → Task 2 `TestResumoDe`, `TestSkillSemCasoSemResumoFicaVazio`.
4. **Tirar o último papel restrito**: `papeis: []` quer dizer "todos"; a tela tem de mostrar "Todos" marcado, não um estado vazio que libera em silêncio. → Task 6 `deselectingLastRoleFallsBackToTodos`.
5. **Proposta do Copiloto que a pessoa não pode aplicar, fora do escopo dela, ou que tira o Confirmar antes de Gravar**: some no servidor com o motivo, e a gravação de verdade passa de novo pela validação manual (422). → Task 23 `TestValidarRecusaFluxoSemConfirmar`, `TestFiltrarPorPermissaoEEscopo`; Task 24 `TestCopilotoNaoPropoeAlemDoPapel`; Task 26 `refusesFlowRejectedByServer`.

A ferramenta antiga com código fora do padrão `grupo.acao` também está coberta (Task 12 `rejectsInvalidCodeOnlyWhenChanged`).

---

## Mapa de arquivos

| Arquivo | Responsabilidade | Tasks |
|---|---|---|
| `web/studio/src/theme/{tokens.ts,theme.ts,theme.test.ts}` | tokens do desenho aprovado e overrides do MUI | 0 |
| `web/studio/src/app/{Topbar,StudioLayout}.tsx` | casca: header 58; botão e painel do Copiloto | 0, 25 |
| `cmd/okf-admin/legado.go` (novo, apagado na 31) | telas e rotas antigas mantidas no ar | 1, 31 |
| `cmd/okf-admin/main.go` | registra `rotasLegado`, `rotasCopiloto`; `PUT /api/agente/config` volta de vez | 1, 24, 31 |
| `web/okf/{index.html,backdoor.*,orquestracao.*}` | restaurados de `a7012bb~1`, apagados na 31 | 1, 31 |
| `internal/copiloto/{operacoes,telas}.go` (novos) | operações tipadas, telas, permissões, validação; puro | 23 |
| `internal/harness/copiloto.go` (novo) | prompt e chamada ao modelo | 24 |
| `cmd/okf-admin/copiloto.go` (novo) | `POST /api/copiloto`, `GET /api/copiloto/estado` | 24 |
| `web/studio/src/copiloto/*` (novo) | contratos, painel, aplicação de propostas, sugestões, contexto por tela | 25-28 |
| `web/studio/src/fluxo/**` | fluxo no desenho aprovado + "Como o morador vê" | 29 |
| `web/studio/src/avancado/{AvancadoPage,ArquivosAba}.tsx` | árvore de arquivos agrupada, editor com números de linha | 30 |
| `internal/okf/resumo.go` (novo) | `ResumoDe`, puro | 2 |
| `internal/okf/caso.go`, `skills.go` | `summary`, `icon`, `Textos.Resumo/Visibilidade`, textos de skill sem caso | 2 |
| `cmd/okf-admin/admin.go` | `casoDeUso.Resumo/Icone` | 2, 23 |
| `cmd/okf-admin/vocabulario_test.go` (novo) | vocabulário dos dados que o Studio mostra | 5 |
| `okf/agentes/concierge/agentes/**` | 10 resumos, ícone, descrição sem "Nexus" | 2, 5 |
| `web/studio/src/concierge/ConciergePage.tsx` (ex-`assistente/AssistentePage.tsx`) | página Concierge (Persona) | 3, 14 |
| `web/studio/src/shared/busca.ts` (novo) | `contemBusca` | 4 |
| `web/studio/src/condominios/listaDeSkills.ts` (novo) | modelo puro da lista de skills do condomínio | 4 |
| `web/studio/src/condominios/paginaDeTeste.tsx` (novo) | helpers de teste da ficha | 4 |
| `web/studio/src/skills/icones.ts` (novo) | ícone lucide → MUI | 4 |
| `web/studio/src/condominios/SkillDoCondominioDrawer.tsx` (novo) | drawer da skill no condomínio | 6 |
| `web/studio/src/condominios/ajustes.ts`, `PreencherComIA.tsx` (novos) | unidade do rótulo; caixa de sugestão única | 7 |
| `web/studio/src/condominios/identificacao.ts` (novo) | validação de formato | 11 |
| `web/studio/src/condominios/MatrizDeSkills.tsx` (novo) | matriz skills × condomínios | 18 |
| `web/studio/src/skills/NovaSkillDialog.tsx` (novo) | criar skill com resumo e critério | 13 |
| `web/studio/src/app/RotaInicial.tsx`, `rotaInicial.ts` (novos) | rota inicial por papel | 16 |
| `web/studio/src/avancado/IdentidadeAba.tsx`, `AjustesTecnicosAba.tsx` (novos) | identidade técnica; payload livre | 21, 22 |
| `web/studio/src/concierge/BoasVindasAba.tsx` (novo) | boas-vindas e "quem pode conversar" do Concierge | 32 |

---

### Task 0: Base visual do desenho aprovado (front)

**Files:**
- Modify: `web/studio/src/theme/tokens.ts`, `web/studio/src/theme/theme.ts`, `web/studio/package.json` (+ `@fontsource/jetbrains-mono`, versão exata mais nova, `npm install --save-exact`), `web/studio/src/main.tsx` (importar a fonte)
- Modify: `web/studio/src/app/Topbar.tsx`, `app/StudioLayout.tsx` (o `Sidebar.tsx` não muda aqui: largura vem de `tokens.layout` e o item ativo do override do tema; o rótulo "Concierge" é da Task 3)
- Test: `web/studio/src/theme/theme.test.ts` (recriar; a st16 apagou)

**Board:** `docs/superpowers/design/studio-fase2/Main.dc.html` e `Condominio.dc.html` (header 58, sidebar 236, tipos, cores e cartões sem sombra aparecem nas duas; qualquer tela serve para conferir espaçamento e fontes).

**Interfaces:**
- Produces em `tokens.ts`: `ESPACOS = [0, 8, 13, 21, 34, 55] as const`; `TIPOS = { overline: 11, caption: 12, body2: 13, body1: 14, subtitle1: 15, h6: 17, h5: 21, h4: 26 } as const` (h1/h2/h3 = 26, `h5` é o título de página); `RAIOS = { campo: 8, chip: 9, menu: 10, painel: 12, cartao: 13 } as const`; `SOMBRA_DO_BLOCO_EM_EDICAO = '0 8px 21px rgba(16,52,242,.12)'`; cores das Global Constraints em `tokens.color` (`primary.main`, `primary.dark` = hover, `navy`, `text.{primary,strong,secondary,tertiary,muted}`, `border.{default,strong,subtle}`, `background.{default,paper,subtle,tint}`, `success`, `warning`, `danger`, `realceDoCopiloto`, `conector`); `FONTE_MONO = '"JetBrains Mono", ui-monospace, monospace'`; `tokens.layout = { sidebarWidth: 236, sidebarCollapsedWidth: 56, navbarHeight: 58, painelLateral: 400, copiloto: 380 }`.
- Produces em `theme.ts`: `espaco(fator: number): number` (inteiro 0-5 → `ESPACOS[fator]`; outro valor → `Math.round(fator * 8)`), usado como `spacing`; overrides: `MuiCard`/`MuiPaper` outlined com raio 13 e `boxShadow: 'none'`; `MuiButton`, `MuiTab` com `textTransform: 'none'`; `MuiTabs` com indicador sublinhado e `gap` 26 px entre abas; `MuiSwitch` 40 × 22; `MuiChip` raio 9; `MuiListItemButton` do menu com 40 px, raio 10, selecionado = fundo tint, texto primário, peso 600; `overline` 11 px, maiúsculo, `letterSpacing: '.06em'`.
- Produces em `StudioLayout.tsx`: a casca `Topbar` + (`Sidebar`, `<Box component="main">`) com um lugar vazio entre o menu e o conteúdo, onde a Task 25 monta o painel do Copiloto.

- [ ] **Step 1: Testes que falham**: `theme.test.ts`: `usesGoldenSpacing` (`it.each` 1→8, 2→13, 3→21, 4→34, 5→55, 0.5→4), `usesApprovedTypeScale` (h5 21, body1 14, body2 13, overline 11 maiúsculo), `usesApprovedFonts` (corpo começa com `"Argumentum", "Inter"`, h5 com `"Paralucent", "Plus Jakarta Sans"` peso 700), `cardsHaveNoShadow`, `tabsAreSentenceCase`, `selectedNavItemUsesTint` (override de `MuiListItemButton` com fundo `#EEF1FE`, texto primário, peso 600, altura 40, raio 10), `layoutMatchesCanvas` (`navbarHeight` 58, `sidebarWidth` 236).
- [ ] **Step 2:** `cd web/studio && npx vitest run src/theme` → FAIL.
- [ ] **Step 3:** Implementar. Páginas existentes ganham o espaçamento novo sem editar cada uma (o `spacing` muda no tema); o título "Studio" do header fica em navy 700, 17 px.
- [ ] **Step 4:** `npm run typecheck && npm test` → PASS (ajustar só asserções de estilo que o tema mudou).
- [ ] **Step 5:** `git add web/studio` e `git commit -m "feat: apply approved studio visual foundation"`.

---

### Task 1: Telas antigas continuam no ar (Go)

Decisão 3: a st16 apagou `index.html`, `backdoor.*`, `orquestracao.*` e as rotas que só o Backdoor usava. Elas voltam, isoladas num arquivo que a Task 31 apaga.

**Files:**
- Restore: `web/okf/index.html`, `web/okf/backdoor.html`, `web/okf/backdoor.js`, `web/okf/orquestracao.html`, `web/okf/orquestracao.js` (de `a7012bb~1`)
- Modify: `web/okf/index.html`, `backdoor.html`, `orquestracao.html` (link "Admin" do menu), `web/okf/admin.js` (`href="/?adm=` → `href="/admin?adm=`)
- Create: `cmd/okf-admin/legado.go`
- Modify: `cmd/okf-admin/main.go` (`rotas`: volta `PUT /api/agente/config`; registra `s.rotasLegado(mux)` junto de `s.rotasAdmin(mux)`), `web/web.go` (comentário de `OKF`)
- Test: `cmd/okf-admin/studio_web_test.go`

**Interfaces:**
- Produces: `func (s *servidor) rotasLegado(mux *http.ServeMux)` registrando `GET /admin` (`index.html`), `GET /backdoor` (`backdoor.html`), `GET /backdoor/orquestracao` (`orquestracao.html`) via o `s.telaAntiga(arquivo)` que já existe em `admin.go`, e `GET /api/anatomia`, `POST /api/agentes`, `DELETE /api/agente`, `POST /api/agentes/mover` com os handlers `anatomia`, `criar`, `excluir`, `mover` copiados de `git show a7012bb~1:cmd/okf-admin/main.go`. `PUT /api/agente/config` (handler `config`) volta para `main.go` e **fica** depois da Task 31 (a Task 21 usa).

- [ ] **Step 1: Escrever os testes que falham**

Em `studio_web_test.go`: tirar de `TestRaizVaiParaStudio` os casos `/backdoor` e `/backdoor/orquestracao` (404) e criar:

```go
func TestTelasAntigasContinuamNoAr(t *testing.T) {
	web := fstest.MapFS{}
	for _, f := range []string{"index.html", "backdoor.html", "orquestracao.html", "administradoras.html", "payloads.html"} {
		web[f] = &fstest.MapFile{Data: []byte(f)}
	}
	s := &servidor{web: web, studio: fstest.MapFS{}}
	mux := http.NewServeMux()
	s.rotas(mux); s.rotasStudio(mux); s.rotasAdmin(mux); s.rotasLegado(mux)
	// casos: /admin→index.html, /backdoor→backdoor.html, /backdoor/orquestracao→orquestracao.html,
	// /backdoor/administradoras→administradoras.html, /backdoor/payloads→payloads.html: 200 e corpo = arquivo.
	// "/" continua 302 para "/studio/".
}

func TestRotasDaTelaAntigaRegistradas(t *testing.T) {
	// para cada {método, caminho, padrão}: GET /api/anatomia, POST /api/agentes, PUT /api/agente/config,
	// DELETE /api/agente, POST /api/agentes/mover: _, padrao := mux.Handler(httptest.NewRequest(...)); padrao == "<método> <caminho>"
}
```

- [ ] **Step 2:** `go test ./cmd/okf-admin/ -run 'TestTelasAntigas|TestRotasDaTelaAntiga|TestRaizVaiParaStudio' -v` → FAIL (`rotasLegado` indefinido).
- [ ] **Step 3:** Restaurar com `git checkout a7012bb~1 -- web/okf/index.html web/okf/backdoor.html web/okf/backdoor.js web/okf/orquestracao.html web/okf/orquestracao.js`; criar `legado.go` (comentário de pacote: "Telas e rotas antigas, mantidas até a fase 2 do Studio cobrir todas as capacidades; a Task 31 do plano da fase 2 apaga este arquivo"); ajustar `main.go`. Nos HTML/JS antigos, todo destino do Admin passa de `/` para `/admin` (`grep -n 'href="/"\|/?adm=' web/okf/*.html web/okf/*.js`).
- [ ] **Step 4:** Mesmo comando → PASS; `go build ./... && go test ./cmd/okf-admin/` → PASS.
- [ ] **Step 5:** `git add web/okf cmd/okf-admin web/web.go` e, em comando próprio, `git commit -m "feat: keep old admin and backdoor pages served during studio phase 2"`.

---

### Task 2: Resumo, ícone e textos completos da skill (Go + dados)

**Files:**
- Create: `internal/okf/resumo.go`, `internal/okf/resumo_test.go`
- Modify: `internal/okf/caso.go:17-32` (struct `Caso`), `internal/okf/skills.go` (`Textos`, `lerTextos`, `textosDoAgente`, `salvarTextos`, `gravarTextosNoCaso`), `internal/okf/skills_test.go`
- Modify: `cmd/okf-admin/admin.go:62-120` (`casoDeUso`, `casosDeUso`), `cmd/okf-admin/studio.go` (tabela de erros, perto da linha 330), `cmd/okf-admin/studio_test.go`
- Modify (dados): `okf/agentes/concierge/agentes/{agendar-mudanca,atualizacao-cadastral,codigo-retirada,comunicar-obra,encomendas,liberacao-visitante,listar-unidades,reserva-area-comum}/caso.yaml`, `okf/agentes/concierge/agentes/{faq,regimento}/agent.yaml`

**Interfaces:**
- Produces (Go):
  - `Caso` ganha `Summary string \`json:"summary"\`` e `Icon string \`json:"icon"\`` (chaves `summary:` e `icon:` do `caso.yaml`).
  - `const LimiteDoResumo = 90`; `func ResumoDe(summary, description string) string`.
  - `Textos` ganha `Resumo string \`json:"resumo"\`` (caso: `summary`; sem caso: `agent.yaml resumo`) e `Visibilidade string \`json:"visibilidade"\`` (caso: `visibility`; sem caso: `""`).
  - Skill sem caso guarda `palavras_chave` e `frases_de_exemplo` (listas) e `resumo` no `agent.yaml`.
  - `var ErrVisibilidadeInvalida = errors.New("visibilidade inválida")`, mapeado em `studio.go` para 422 `{"codigo":"VISIBILIDADE_INVALIDA","mensagem":"Escolha quem pode usar a skill."}`.
  - JSON de `casoDeUso` (vale para `GET /api/admin/estado` e `GET /api/skills`) ganha `"resumo"` e `"icone"`.

- [ ] **Step 1: Teste puro do resumo** em `resumo_test.go`, tabela `TestResumoDe` (`summary`, `description`, `quer`):

| caso | summary | description | quer |
|---|---|---|---|
| resumo próprio vence | `"  Agenda o dia da mudança. "` | `"qualquer"` | `"Agenda o dia da mudança."` |
| primeira frase | `""` | `"Mostra as unidades vinculadas ao cadastro.\nOutra linha."` | `"Mostra as unidades vinculadas ao cadastro."` |
| espaços colapsados | `""` | `"Atualiza dados   cadastrais\n da unidade. Depois."` | `"Atualiza dados cadastrais da unidade."` |
| prefixo "quando" | `""` | `"Usado quando o morador quer liberar um visitante. Mais."` | `"Quer liberar um visitante."` |
| prefixo dono | `""` | `"Usado para o dono ou morador da unidade autorizar a entrada de um visitante na portaria (amigo, parente)."` | `"Autorizar a entrada de um visitante na portaria (amigo, parente)."` |
| prefixo morador | `""` | `"Usado para o morador ver as encomendas."` | `"Ver as encomendas."` |
| corta em 90 | `""` | `"Palavra" + strings.Repeat(" palavra", 15)` | `"Palavra" + strings.Repeat(" palavra", 10) + "…"` |
| vazio | `""` | `""` | `""` |

Regras que a tabela fixa: `summary` aparado vence; senão colapsa espaços da descrição, corta na primeira `". "` (mantém o ponto), tira o primeiro prefixo que casar de `["Usado para o dono ou morador da unidade ", "Usado quando o morador ", "Usado para o morador "]` e põe a primeira letra em maiúscula; com mais de 90 runas, corta no último espaço dentro das 89 primeiras e acrescenta `…`.

- [ ] **Step 2:** `go test ./internal/okf/ -run TestResumoDe -v` → FAIL; implementar `resumo.go`; → PASS.
- [ ] **Step 3: Testes de textos** em `skills_test.go` (catálogo de `catalogoDeTeste`):
  - `TestLerTextosTrazResumoEVisibilidade`: depois de gravar `summary: Agenda a mudança.` no caso de `agendar-mudanca`, `LerTextos` devolve `Resumo == "Agenda a mudança."` e `Visibilidade == "private"`.
  - `TestSalvarTextosGravaResumoSemMexerNoRoteamento`: muda só `Resumo`; o `caso.yaml` relido tem `summary`; `agent.yaml descricao` e a linha do `routers/rotas.md` ficam byte a byte iguais.
  - `TestSalvarTextosApagaResumoVazio`: `Resumo: ""` tira a chave `summary` do `caso.yaml`.
  - `TestSalvarTextosRecusaVisibilidadeInvalida`: `Visibilidade: "todos"` → `errors.Is(err, ErrVisibilidadeInvalida)`; `""` mantém a atual; `"public"` grava `visibility: public`.
  - `TestSalvarTextosDeSkillSemCasoGravaNoAgente`: em `agentes/concierge/agentes/faq`, grava `Resumo`, `PalavrasChave: ["boleto"]`, `FrasesDeExemplo: ["qual o horário da piscina?"]`; `LerTextos` relê os três; `agent.yaml` tem `resumo`, `palavras_chave`, `frases_de_exemplo`; lista vazia apaga a chave.
- [ ] **Step 4:** `go test ./internal/okf/ -run 'TestLerTextos|TestSalvarTextos' -v` → FAIL.
- [ ] **Step 5: Implementar** em `skills.go`: `lerTextos`/`textosDoAgente` preenchem os campos novos (listas via o `stringList` que já existe); em `salvarTextos`, validar visibilidade quando `atual.TemFluxo` (`""` vira a atual; fora de `private|public` → `ErrVisibilidadeInvalida`); `gravarTextosNoCaso` acrescenta os pares `summary` (nó nil quando vazio, que `definirChave` apaga) e `visibility`; sem caso, nova `func salvarTextosDoAgente(dir string, t Textos) error` (lê com `lerConfig`, põe ou apaga `resumo`, `palavras_chave`, `frases_de_exemplo`, grava com `gravarConfig`). `mudaRoteamento` não muda: resumo e visibilidade não mexem na rota.
- [ ] **Step 6:** Mesmo comando → PASS.
- [ ] **Step 7: Testes da API** em `studio_test.go` (usam `handlerDeTeste`, que copia o `okf/` real):
  - `TestSkillsReaisTemResumo`: `GET /api/skills`, toda skill tem `resumo` não vazio com até 90 runas e nenhum `resumo` é igual à `descricao`.
  - `TestSkillSemCasoSemResumoFicaVazio`: apagar a linha `resumo:` de `agentes/concierge/agentes/regimento/agent.yaml` na cópia; `GET /api/skills` → `regimento.resumo == ""`.
  - `TestCasoSemSummaryDerivaResumo`: apagar `summary:` de `listar-unidades/caso.yaml` na cópia → `resumo == "Mostra as unidades (apartamentos/casas) vinculadas ao cadastro de quem pergunta."`.
  - `TestSkillsTrazemIcone`: `agendar-mudanca.icone == "truck"`, `faq.icone == ""`.
  - Caso novo na tabela de `TestStudioAPI`: `PUT …/textos` com `visibilidade: "todos"` → 422 `VISIBILIDADE_INVALIDA`.
- [ ] **Step 8:** `go test ./cmd/okf-admin/ -run 'TestSkills|TestSkillSem|TestCasoSem|TestStudioAPI' -v` → FAIL.
- [ ] **Step 9: Implementar** em `admin.go`: `casoDeUso` ganha `Resumo string \`json:"resumo"\`` e `Icone string \`json:"icone"\``; com caso, `Resumo = okf.ResumoDe(sub.Caso.Summary, sub.Caso.Description)` e `Icone = sub.Caso.Icon`; sem caso, `Resumo` = `sub.Config["resumo"]` se for string, senão `""`. Mapear o erro em `studio.go`.
- [ ] **Step 10: Dados.** Linha `summary:` logo depois de `name:` em cada `caso.yaml`; `resumo:` no `agent.yaml` de faq e regimento; `icon: user-check` depois de `visibility:` em `liberacao-visitante/caso.yaml`. Textos exatos:

| skill | resumo |
|---|---|
| agendar-mudanca | Agenda o dia e o período da mudança, com o elevador de serviço. |
| atualizacao-cadastral | Atualiza e-mail, veículo e pets da unidade. |
| codigo-retirada | Passa o código para retirar uma encomenda na portaria. |
| comunicar-obra | Avisa a administração sobre uma obra e libera os prestadores. |
| encomendas | Mostra as encomendas da unidade. |
| liberacao-visitante | Autoriza a entrada de um visitante para hoje ou amanhã. |
| listar-unidades | Mostra as unidades ligadas ao cadastro do morador. |
| reserva-area-comum | Reserva churrasqueira, salão ou quadra para uma data. |
| faq | Responde dúvidas gerais: horários, endereço, contato, boleto. |
| regimento | Responde o que o regimento interno permite, citando o artigo. |

- [ ] **Step 11:** `go test ./internal/okf/ ./cmd/okf-admin/` → PASS (inclui `TestPrincipalEmDiaComCaso` e `TestSkillsReaisSalvamSemMudarOCaso`: `summary` não entra no `principal.md`).
- [ ] **Step 12:** `git add internal/okf cmd/okf-admin okf/agentes` e `git commit -m "feat: add plain summary and icon to studio skills"`.

---

### Task 3: "Concierge" no lugar de "Assistente" (front)

**Files:**
- Move: `web/studio/src/assistente/AssistentePage.tsx` → `web/studio/src/concierge/ConciergePage.tsx`, `AssistentePage.test.tsx` → `concierge/ConciergePage.test.tsx` (`git mv`)
- Modify: `app/router.tsx` (`{ path: 'concierge', element: <ConciergePage /> }`, sai `assistente`), `app/Sidebar.tsx` (`{ rotulo: 'Concierge', destino: '/concierge', … }`), `avancado/AvancadoPage.tsx`, `avancado/OrquestracaoAba.tsx`, `condominios/CondominiosPage.tsx`, `condominios/OrientacoesSecao.tsx`, `condominios/SkillsSecao.tsx`, `skills/QuandoUsarAba.tsx`, `skills/SkillsPage.tsx`, `skills/abas/ConhecimentoAba.tsx`, `ferramentas/contratos.ts`, `vocabulario.test.ts`
- Test: `app/StudioLayout.test.tsx`, `acesso/GuardaAvancado.test.tsx`, `avancado/AvancadoPage.test.tsx`, `condominios/CondominiosPage.test.tsx`, `skills/SkillsPage.test.tsx`, `skills/abas/ConhecimentoAba.test.tsx`, `concierge/ConciergePage.test.tsx`

**Board:** `docs/superpowers/design/studio-fase2/Concierge.dc.html` e `Main.dc.html` (item "Concierge" na sidebar, em vez de "Assistente").

**Interfaces:**
- Produces: `export function ConciergePage(): ReactNode` em `concierge/ConciergePage.tsx`; rota `/concierge`. O campo `assistente` da API (`SkillsSchema`, `skillsFalsas`) **não muda** (contrato com o Go).

- [ ] **Step 1: Teste de vocabulário que falha.** Em `vocabulario.test.ts`, um segundo `it.each` sobre **todas** as telas `.tsx` (inclusive `./avancado/`, fora `.test.tsx`) com `const NOME_ANTIGO: readonly RegExp[] = [/\bAssistentes?\b/, /\b(a|da|na|pela|à) assistente\b/i]`, teste `'%s chama o robô de Concierge'`.
- [ ] **Step 2:** `npx vitest run src/vocabulario.test.ts` → FAIL listando os arquivos acima.
- [ ] **Step 3: Trocar os textos** (exatos):
  - Menu e página: `Concierge`; `aria-label="Seções do Concierge"`; Persona: `ajuda="A voz do concierge em todas as respostas ao morador."`.
  - `AvancadoPage`: `"Ainda não há concierge nem skills para mostrar."`, seletor `label="Concierge ou skill"`.
  - `OrquestracaoAba`: `"Como o concierge decide o que fazer com cada mensagem."`.
  - `CondominiosPage`: `"Diga o que o concierge faz em cada condomínio."`.
  - `SkillsSecao`: `ajuda="Ligue o que o concierge pode fazer aqui."`, vazio `"O concierge ainda não tem skills."`.
  - `OrientacoesSecao`: `ajuda="O que o concierge precisa saber deste condomínio."`.
  - `QuandoUsarAba`: `"Diga em uma frase quando o concierge deve usar esta skill."`.
  - `SkillsPage` `FRASE_VAZIA`: `'Nenhuma skill ainda. Uma skill é algo que o concierge sabe fazer pelo morador, como reservar uma área comum.'`.
  - `ConhecimentoAba`: `"Vem do Concierge"`.
  - `ferramentas/contratos.ts`: `GRUPOS.agente = 'Agentes'`, `TIPOS.agent = 'Agente'` (um agente de ferramenta não é o Concierge).
  Atualizar as asserções dos testes listados para os textos novos.
- [ ] **Step 4:** `npm run typecheck && npm test` → PASS.
- [ ] **Step 5:** `git add -A web/studio/src` e `git commit -m "feat: call the robot concierge across the studio"`.

---

### Task 4: Contratos e modelo puro da lista de skills (front)

**Files:**
- Modify: `web/studio/src/condominios/contratos.ts`, `condominios/contratos.test.ts`, `skills/contratos.ts`, `skills/contratos.test.ts`, `condominios/estadoFalso.ts`
- Create: `shared/busca.ts` + `shared/busca.test.ts`, `condominios/listaDeSkills.ts` + `listaDeSkills.test.ts`, `skills/icones.ts` + `icones.test.ts`, `condominios/paginaDeTeste.tsx`
- Split: `condominios/CondominioPage.test.tsx` → `SkillsSecao.test.tsx` (`togglesSkill`, `resetsToDefault`, `savesRolesWhenChipIsToggled`), `OrientacoesSecao.test.tsx` (`savesGuidanceOneSecondAfterTyping`), `AjustesSecao.test.tsx` (`fillsAdjustmentsFromSuggestion`, `clearsAdjustmentOverrideWithUsarOPadrao`), `IdentificacaoSecao.test.tsx` (`gestorCannotEditIdentification`, `savesIdentificationOnExplicitClick`); ficam em `CondominioPage.test.tsx` `showsDefaultWithoutIdentification`, `gestorCannotEditDefault`, `staysReadOnlyBelow900px`. Os testes movem sem mudar o corpo; isso existe só para as tasks da onda 2 não dividirem arquivo.

**Interfaces:**
- Produces:
  - `CasoSchema` + `id: z.string().default('')`, `resumo: z.string().default('')`, `icone: z.string().default('')`; `SkillResumoSchema` + `resumo`, `icone` (mesmo default); `TextosSchema` + `resumo: z.string().default('')`, `visibilidade: z.string().default('')`.
  - `condominios/contratos.ts`: `salvarPayload(corpo: { condominio: string; recurso: string; payload: Record<string, Valor>; apagar: boolean }, escopo: Escopo): Promise<RespostaMapeamento>` (`PUT /api/admin/payload`); `faltamIds(c: Condominio): boolean` (algum de `account`, `company_id`, `tenant` vazio depois de `trim`); `ferramentasDaSkill(estado: Estado, caso: Caso): Ferramenta[]` (recursos com `definicoes` e nome em `caso.recursos`); `ferramentasGerais(estado: Estado): Ferramenta[]` (com `definicoes` e fora de todo `caso.recursos`).
  - `shared/busca.ts`: `normalizarBusca(texto: string): string` (NFD, sem acento, minúsculas, `trim`); `contemBusca(alvos: readonly string[], busca: string): boolean` (busca vazia → `true`).
  - `skills/icones.ts`: `iconeDaSkill(icone: string): SvgIconComponent` com `truck`→`LocalShippingOutlined`, `file-pen-line`→`EditNoteOutlined`, `key-round`→`KeyOutlined`, `hammer`→`HandymanOutlined`, `package`→`Inventory2Outlined`, `calendar-check`→`EventAvailableOutlined`, `building-2`→`ApartmentOutlined`, `user-check`→`HowToRegOutlined`; qualquer outro → `ExtensionOutlined`.
  - `condominios/listaDeSkills.ts`:
    ```ts
    export type Categoria = 'registra' | 'consulta' | 'duvida';
    export const ROTULO_DA_CATEGORIA: Record<Categoria, string>; // 'Registra pedidos' | 'Consultas' | 'Dúvidas'
    export function categoriaDe(caso: Caso): Categoria; // escrita → registra; id !== '' → consulta; senão duvida
    export function rotuloDoPapel(papel: string): string; // morador, proprietário, síndico, público; desconhecido = o próprio
    export function rotuloDePapeis(papeis: readonly string[]): string | null; // [] → null; ['sindico','proprietario'] → 'só síndico, proprietário'
    export type Origem = 'propria' | 'padrao' | 'nenhuma';
    export function origemDe(mapeamento: Mapeamento, condominio: string, caso: string): Origem;
    export type LinhaDeSkill = { caso: Caso; titulo: string; habilitacao: Habilitacao; propria: boolean; categoria: Categoria };
    export function linhasDeSkills(estado: Estado, condominio: string, titulos: ReadonlyMap<string, string>): LinhaDeSkill[];
    export function filtrarLinhas(linhas: readonly LinhaDeSkill[], filtro: { busca: string; soQueMuda: boolean }): LinhaDeSkill[];
    export function agruparPorCategoria(linhas: readonly LinhaDeSkill[]): { categoria: Categoria; linhas: LinhaDeSkill[] }[]; // ordem registra, consulta, duvida; grupo vazio some
    export function contarLigadas(linhas: readonly LinhaDeSkill[]): { ligadas: number; total: number };
    export function exemploDeOrientacao(nomeDaSkill: string): string;
    ```
    `origemDe`: decisão do condomínio → `propria`; senão, fora do Padrão e com decisão no Padrão → `padrao`; senão `nenhuma`. `filtrarLinhas` busca em `titulo` e `caso.resumo` com `contemBusca`; `soQueMuda` deixa só `propria`. `exemploDeOrientacao` (regex no nome, primeira que casar): `/reserva/` → `'a churrasqueira só pode ser reservada por quem está em dia com o condomínio'`; `/visitante/` → `'prestadores só entram com documento; a portaria liga para a unidade antes'`; `/mudanca/` → `'o elevador de serviço precisa de proteção; o zelador acompanha'`; `/obra/` → `'obras só com o formulário da administração assinado'`; `/encomenda|retirada/` → `'a portaria guarda encomendas por até 7 dias'`; senão `'o que o concierge deve saber ou fazer diferente aqui'`.
  - `condominios/paginaDeTeste.tsx`: `gravador(rota: string): { corpos: Record<string, unknown>[]; handlers: Record<string, (req: Request) => Promise<Response>> }` e `abrirCondominio(id?: string)` (os helpers que hoje estão no topo de `CondominioPage.test.tsx`).
  - `estadoFalso.ts`: casos e skills ganham `resumo` (`'Agenda o dia da mudança.'`, `'Reserva o salão de festas.'`) e `icone` (`'truck'`, `''`); `reserva-do-salao` passa a ter `recursos: ['reserva-salao']`; `recursos` ganha `reserva-salao` (sem `definicoes`) e `horario-portaria` com `definicoes: [{ name: 'horario', label: 'Horário da portaria', type: 'string', default: '7h às 22h' }]` (ferramenta geral).

- [ ] **Step 1: Testes que falham**: `busca.test.ts` (`ignoresAccentsAndCase`, `acceptsEmptySearch`); `icones.test.ts` (`it.each` dos 8 nomes + `mapsUnknownToFallback`); `listaDeSkills.test.ts` (`categorizesByWriteAndCase` em tabela, `labelsOnlyRestrictedRoles`, `tellsOriginOfDecision` em tabela com os 3 casos e o Padrão, `filtersBySearchOnTitleAndSummary`, `keepsOnlyOwnDecisions`, `groupsInFixedOrderDroppingEmpty`, `countsEnabled` → `{ ligadas: 1, total: 2 }` para `vila-verde` no `estadoFalso`, `picksGuidanceExampleByName` em tabela); `contratos.test.ts` (`flagsMissingIntegrationIds`, `splitsSkillAndGeneralTools` → `ferramentasDaSkill(agendar-mudanca)` = `['agenda-mudanca']`, `ferramentasGerais` = `['horario-portaria']`, `defaultsSummaryAndIconWhenGoOmits`).
- [ ] **Step 2:** `npx vitest run src/shared src/skills/icones.test.ts src/condominios/listaDeSkills.test.ts src/condominios/contratos.test.ts` → FAIL.
- [ ] **Step 3:** Implementar os arquivos com as assinaturas acima.
- [ ] **Step 4:** Mesmo comando → PASS. Fazer o split dos testes da ficha; `npm run typecheck && npm test` → PASS (mesma contagem de testes da ficha que antes).
- [ ] **Step 5:** `git add -A web/studio/src` e `git commit -m "feat: add summary contracts and skill list model to studio"`.

---

### Task 5: Vocabulário dos dados que o Studio mostra (Go + dados)

**Files:**
- Create: `cmd/okf-admin/vocabulario_test.go`
- Modify: `okf/agentes/concierge/agentes/atualizacao-cadastral/caso.yaml` (`description`), `…/atualizacao-cadastral/agent.yaml` (`descricao`), `…/atualizacao-cadastral/workflows/principal.md` (linha 3)

**Interfaces:**
- Consumes: `casoDeUso.Resumo` (Task 2); `handlerDeTeste` de `studio_test.go`.

- [ ] **Step 1: Teste que falha**:

```go
var termoTecnico = regexp.MustCompile(`(?i)\bnexus\b|\bmcp\b|\byaml\b|caso de uso|subagente|orquestrador|roteador|backdoor`)

func TestDadosDoStudioSemVocabularioTecnico(t *testing.T) {
	// servidor como handlerDeTeste + admin: admin.NewStore(filepath.Join(t.TempDir(), "mapeamento.json")), com rotasAdmin e rotasStudio.
	// GET /api/admin/estado?administradora=adm-exemplo: casos[].titulo, .resumo, .descricao; recursos[].titulo;
	//   recursos[].definicoes[].label, .description.
	// GET /api/skills: skills[].titulo, .resumo, .descricao.
	// Cada valor que casar com termoTecnico vira t.Errorf("%s %s: %q", rota, campo, valor).
}
```

- [ ] **Step 2:** `go test ./cmd/okf-admin/ -run TestDadosDoStudio -v` → FAIL em `atualizacao-cadastral` ("vêm do Nexus").
- [ ] **Step 3:** Nova descrição, nos três arquivos (no `agent.yaml` sem o sufixo "Caso de uso: …"): `Atualiza dados cadastrais da unidade (e-mail, veículo, pets...). Os campos do formulário mudam conforme a unidade escolhida.` A linha 3 do `principal.md` fica igual a esse texto (é o que `GerarPrincipal` produz).
- [ ] **Step 4:** `go test ./cmd/okf-admin/ ./internal/okf/` → PASS (inclui `TestPrincipalEmDiaComCaso`).
- [ ] **Step 5:** `git add cmd/okf-admin/vocabulario_test.go okf/agentes` e `git commit -m "test: guard studio data against technical vocabulary"`.

---

### Task 6: Drawer da skill no condomínio (front)

**Files:**
- Create: `web/studio/src/condominios/SkillDoCondominioDrawer.tsx`, `SkillDoCondominioDrawer.test.tsx`

**Board:** `docs/superpowers/design/studio-fase2/Condominio.dc.html` (drawer "O que o concierge faz aqui": cabeçalho, "Quem pode usar", "Orientação desta skill", "Voltar ao Padrão", "Ver o fluxo").

**Interfaces:**
- Consumes: `origemDe`, `rotuloDoPapel`, `exemploDeOrientacao` (Task 4); `ferramentasDaSkill`, `salvarHabilitacao`, `salvarOrientacao`, `salvarAjustes`, `habilitacaoDe`, `PADRAO` (contratos); `CampoAjuste`; `iconeDaSkill`; `useAutosave`, `SaveIndicator`; `usePodeEditar`, `useEu`, `pode`.
- Produces:
  ```ts
  type Props = { estado: Estado; condominio: string; caso: Caso; titulo: string; editavel: boolean; onFechar: () => void };
  export function SkillDoCondominioDrawer(props: Props): ReactNode;
  ```

Conteúdo (Drawer `anchor="right"`, papel `width: 'min(420px, 100vw)'`, `role="dialog"`, `aria-label` = `` `${titulo} · ${nomeDoCondominio}` `` com `nomeDoCondominio` = nome do condomínio ou `'Padrão (todos)'`):
1. Cabeçalho: ícone, título (`h2`), resumo, `Switch` com `aria-label="Ligada neste condomínio"` (grava `habilitado` mantendo `papeis`) e a frase de origem: `propria` → `'Este condomínio tem decisão própria.'`; `padrao` → `'Sem decisão própria: vale o Padrão. Mudar aqui cria a decisão deste condomínio.'`; `nenhuma` → `'Sem decisão: ligada para todos.'`.
2. "Quem pode usar": pílulas que alternam (`ToggleButton` com raio 9 e `aria-pressed`): `Todos` + uma por `estado.papeis` (rótulo `rotuloDoPapel`). `Todos` grava `papeis: []` e desmarca as outras; marcar um papel desmarca `Todos` e grava a lista; desmarcar o último papel volta a marcar `Todos` e grava `papeis: []`.
3. "Orientação para esta skill": `TextField` multiline, `minRows={3}`, `placeholder` `` `Ex.: ${exemploDeOrientacao(caso.nome)}` ``, autosave que grava `{ condominio, geral: <geral atual do estado>, por_caso: <por_caso atual do estado sem textos vazios, com esta skill trocada> }`. O "atual" é lido de `estado.mapeamento.orientacoes[condominio]` dentro do `salvar` (o `useAutosave` usa sempre a função da última renderização).
4. "Ajustes desta skill" (some quando `ferramentasDaSkill` é vazio): por ferramenta, subtítulo `tituloDe(ferramenta)` e um `CampoAjuste` por definição (`herdado` = payload do Padrão ?? `definicao.default`). Autosave grava `{ ...(estado.mapeamento.payloads[condominio] ?? {}), ...ajustesDestaSkill }`.
5. "Como o concierge reconhece o pedido": `Accordion` fechado com `caso.descricao` só leitura, só para `usePodeEditar('skills.editar_textos')`.
6. Rodapé: `Voltar ao Padrão` (só com `propria` e `editavel`; `limpar: true`); link `Ver o fluxo` para `` `/skills/${encodeURIComponent(caso.nome)}?aba=fluxo` `` quando `pode(eu, 'skills.ler')`.
Com `editavel` falso, tudo desabilitado.

- [ ] **Step 1: Testes que falham** (renderizam o drawer direto com `estadoFalso`, `renderComProvedores` e `responderCom`): `showsOriginOfDecision` (`it.each` das 3 frases), `restrictsToChosenRoles` (pílula síndico → corpo `{ papeis: ['sindico'] }`, `Todos` com `aria-pressed="false"`), `deselectingLastRoleFallsBackToTodos`, `showsDefaultAsPlaceholder` (campo de ajuste com placeholder do Padrão), `savesSkillGuidanceKeepingGeneral` (orientação com `geral: 'Piscina fecha às 22h.'` no estado; corpo mantém `geral`), `savesSkillAdjustmentsKeepingOthers` (payload de `horario-portaria` no estado; corpo de `PUT /api/admin/ajustes` mantém `horario-portaria` e traz `agenda-mudanca`), `resetsToDefault`, `hidesWhenToUseFromGestor`, `linksToFlow`, `disablesEverythingWhenReadOnly`.
- [ ] **Step 2:** `npx vitest run src/condominios/SkillDoCondominioDrawer.test.tsx` → FAIL.
- [ ] **Step 3:** Implementar. Seções com `Typography variant="subtitle2" component="h3"`; sem sombra; espaçamento 2.
- [ ] **Step 4:** Mesmo comando → PASS; `npm run typecheck`.
- [ ] **Step 5:** `git add web/studio/src/condominios` e `git commit -m "feat: add skill drawer to condominio page"`.

---

### Task 7: CampoAjuste e Ajustes gerais (front)

**Files:**
- Create: `web/studio/src/condominios/ajustes.ts`, `ajustes.test.ts`, `PreencherComIA.tsx`, `CampoAjuste.test.tsx`
- Modify: `condominios/CampoAjuste.tsx`, `condominios/AjustesSecao.tsx`, `condominios/AjustesSecao.test.tsx`

**Board:** `docs/superpowers/design/studio-fase2/Condominio.dc.html` (bloco "Ajustes", campos com placeholder do padrão e "Dias à frente para liberar" como exemplo de unidade).

**Interfaces:**
- Consumes: `ferramentasGerais` (Task 4), `sugerirAjustes`, `salvarAjustes`.
- Produces: `separarUnidade(rotulo: string): { rotulo: string; unidade: string | null }` (parêntese no fim com até 12 caracteres vira unidade: `'Antecedência mínima da reserva (dias)'` → `{ rotulo: 'Antecedência mínima da reserva', unidade: 'dias' }`; sem parêntese → `unidade: null`). `PreencherComIA({ condominio, ferramentas, valores, onPreencher })` com `onPreencher: (recurso: string, campos: Record<string, Valor>) => void`. Props de `CampoAjuste` não mudam.

- [ ] **Step 1: Testes que falham**:
  - `ajustes.test.ts`: `separatesUnitFromLabel` em tabela (com unidade, sem, parêntese longo demais fica no rótulo).
  - `CampoAjuste.test.tsx`: `showsInheritedValueAsPlaceholderEvenWithDescription` (definição com `description: 'Quantos dias antes.'` e `herdado: 2` → `placeholder="2"` e o texto `'Quantos dias antes.'`), `showsSemPadraoWhenNothingIsInherited`, `offersUsarOPadraoForEveryType` (`it.each(['number','string','text','select','boolean'])`), `showsUnitAsAdornment` (`'dias'` ao lado do campo de número).
  - `AjustesSecao.test.tsx`: `showsOnlyToolsOutsideSkills` (aparece "Horário da portaria", não aparece "Antecedência mínima (dias)"), `keepsSkillAdjustmentsWhenSaving` (payload de `agenda-mudanca` no estado continua no corpo), `fillsAdjustmentsFromSuggestion` (agora clica `Preencher com IA` antes), `clearsAdjustmentOverrideWithUsarOPadrao` (no campo `horario`).
- [ ] **Step 2:** `npx vitest run src/condominios/ajustes.test.ts src/condominios/CampoAjuste.test.tsx src/condominios/AjustesSecao.test.tsx` → FAIL.
- [ ] **Step 3: Implementar.** `CampoAjuste`: `placeholder` = valor herdado em texto (vazio → `'Sem padrão'`), `slotProps.inputLabel.shrink: true`, `helperText` = `definicao.description` (só ela); `select` com primeira opção `` `Padrão: ${herdado}` `` ou `'Sem padrão'`; boolean sem decisão mostra `(padrão: sim|não)`; `Usar o padrão` em todo tipo quando há decisão; número com `unidade` ganha `InputAdornment position="end"`. `AjustesSecao`: título `"Ajustes gerais"`, ajuda `"Prazos, horários e códigos que não são de uma skill só. Campo vazio vale o padrão."`, só `ferramentasGerais`, estado local só dessas ferramentas, grava `{ ...(estado.mapeamento.payloads[condominio] ?? {}), ...ajustes }`; sem ferramenta geral: `"Os ajustes de cada skill ficam na própria skill, na lista acima."`. A caixa "Descreva em uma frase / Sugerir" sai de cada ferramenta e vira um só `PreencherComIA` (botão `Preencher com IA` que abre a caixa; `select` de ferramenta só com mais de uma).
- [ ] **Step 4:** Mesmo comando → PASS; `npm run typecheck`.
- [ ] **Step 5:** `git add web/studio/src/condominios` e `git commit -m "fix: show inherited adjustment values and keep only general ones"`.

---

### Task 8: Orientações gerais legíveis (front)

**Files:**
- Modify: `web/studio/src/condominios/OrientacoesSecao.tsx`, `OrientacoesSecao.test.tsx`

**Board:** `docs/superpowers/design/studio-fase2/Condominio.dc.html` (seção "Orientações para este condomínio": `"Vale para todas as conversas daqui. Escreva como falaria com um porteiro novo."`).

**Interfaces:**
- Props de `OrientacoesSecao` perdem `titulos` (`{ estado, condominio, editavel }`); `CondominioPage` muda na Task 17.

- [ ] **Step 1: Testes que falham**: `savesGuidanceOneSecondAfterTyping` (campo `Orientações gerais`; com `por_caso: { 'agendar-mudanca': 'Zelador acompanha.' }` no estado, o corpo mantém `por_caso`), `showsExampleAndHelp` (placeholder começa com `'Ex.: A piscina fecha às 22h.'`; ajuda visível), `showsDefaultGuidanceReadOnly` (Padrão com `geral: 'Silêncio após as 22h.'` → `'Também vale o do Padrão:'` + texto, não editável), `hidesPerSkillGuidance` (nenhum campo com o título de skill).
- [ ] **Step 2:** `npx vitest run src/condominios/OrientacoesSecao.test.tsx` → FAIL.
- [ ] **Step 3: Implementar.** Título `"Orientações gerais"`; ajuda `"Escreva como você explicaria a um atendente novo: horários, nomes, regras locais, o que não fazer. O concierge lê isso em toda conversa deste condomínio."`; campo `aria-label="Orientações gerais"`, `minRows={4}`, placeholder `"Ex.: A piscina fecha às 22h. O síndico é o sr. Paulo (não passe o telefone dele). Não aceitamos mudança aos domingos. Entregadores só até as 21h."`; estado local só `geral`; grava `{ condominio, geral, por_caso: estado.mapeamento.orientacoes[condominio]?.por_caso ?? {} }`; bloco do Padrão só fora do Padrão e com texto.
- [ ] **Step 4:** Mesmo comando → PASS.
- [ ] **Step 5:** `git add web/studio/src/condominios` e `git commit -m "feat: explain general guidance and show the default text"`.

---

### Task 9: Lista de condomínios em tabela (front)

**Files:**
- Modify: `web/studio/src/condominios/CondominiosPage.tsx`, `CondominiosPage.test.tsx`

**Board:** `docs/superpowers/design/studio-fase2/Condominios.dc.html` (tabela com "Situação", "Skills ativas", chips "Faltam ids" e "Sem administradora", linha "Padrão (todos)" fixa).

**Interfaces:**
- Consumes: `linhasDeSkills`, `contarLigadas` (Task 4), `faltamIds`, `contemBusca`.
- Produces: aba na URL `?aba=condominios|administradoras` (padrão `condominios`; a Task 18 acrescenta `skills`); filtro `?administradora=<id>` (a Task 10 aponta para ele).

- [ ] **Step 1: Testes que falham**: `listsCondominiosAsTableWithSkillCount` (linha "Vila Verde" com `'1 de 2'`), `pinsDefaultFirst` (primeira linha "Padrão (todos)"), `filtersBySearch` (campo `Buscar condomínio`, ignora acento), `filtersByAdministradoraFromQuery` (`?administradora=adm-exemplo` → chip `Administradora: Administradora Exemplo` com botão de tirar), `flagsMissingIntegrationIds` (condomínio sem `tenant` → chip `Faltam ids`), `highlightsWithoutAdministradora` (chip `Sem administradora`), `hidesAdministradoraColumnForGestor`, `opensTabFromQuery` (`?aba=administradoras`). Os testes existentes continuam (`linksLegacyScreensForSuperlogica` inclusive).
- [ ] **Step 2:** `npx vitest run src/condominios/CondominiosPage.test.tsx` → FAIL.
- [ ] **Step 3: Implementar.** `Table size="small"` em `Paper variant="outlined"`; colunas Condomínio (nome + chips), Administradora (só com escopo `null`), Skills ligadas (`` `${ligadas} de ${total}` ``); linha inteira clicável para `/condominios/:id`; Padrão fixo no topo com legenda `"Vale para todo condomínio que não tem decisão própria."`; vazio `"Nenhum condomínio ainda."`, sem resultado `"Nenhum condomínio encontrado."`. Abas lidas e gravadas com `useSearchParams` (`replace: true`).
- [ ] **Step 4:** Mesmo comando → PASS.
- [ ] **Step 5:** `git add web/studio/src/condominios` e `git commit -m "feat: list condominios as a searchable table"`.

---

### Task 10: Administradoras: renomear, contar, ver condomínios (front)

**Files:**
- Modify: `web/studio/src/condominios/AdministradorasAba.tsx`
- Create: `condominios/AdministradorasAba.test.tsx`

**Board:** `docs/superpowers/design/studio-fase2/Condominios.dc.html` (aba "Administradoras": nome, id e contagem de condomínios).

**Interfaces:**
- Consumes: `InlineTitle` (`shared/ui/InlineTitle.tsx`), `salvarAdministradora`.
- Produces: link `` `/condominios?aba=condominios&administradora=${encodeURIComponent(id)}` ``.

- [ ] **Step 1: Testes que falham** (renderizam `<AdministradorasAba administradoras condominios />`): `renamesAdministradoraInPlace` (clica `Editar nome da administradora`, digita, Enter → `PUT /api/admin/administradora` com `{ id: 'adm-exemplo', nome: 'Nova' }`), `showsCondominioCount` (`it.each`: 0 → `'Nenhum condomínio'`, 1 → `'1 condomínio'`, 3 → `'3 condomínios'`), `linksToFilteredList`, `showsId`.
- [ ] **Step 2:** `npx vitest run src/condominios/AdministradorasAba.test.tsx` → FAIL.
- [ ] **Step 3:** Implementar; `InlineTitle` com `rotulo="nome da administradora"`, sucesso `'Administradora renomeada.'`; id em `caption` monoespaçado; o aviso "Mova os condomínios dela antes de remover." continua.
- [ ] **Step 4:** Mesmo comando → PASS; `npx vitest run src/condominios/CondominiosPage.test.tsx` → PASS.
- [ ] **Step 5:** `git add web/studio/src/condominios` e `git commit -m "feat: rename administradoras and show their condominios"`.

---

### Task 11: Identificação com formato e salvamento automático (front)

**Files:**
- Create: `web/studio/src/condominios/identificacao.ts`, `identificacao.test.ts`
- Modify: `condominios/IdentificacaoSecao.tsx`, `IdentificacaoSecao.test.tsx`

**Board:** `docs/superpowers/design/studio-fase2/Condominio.dc.html` (seção "Identificação" com id e placeholders de formato; chip "Faltam ids" na ficha e na lista).

**Interfaces:**
- Produces: `type CampoDeIdentificacao = 'nome' | 'account' | 'company_id' | 'tenant'`; `validarIdentificacao(c: Condominio): Partial<Record<CampoDeIdentificacao, string>>`:
  - `nome` vazio → `'Informe o nome.'`
  - `account` preenchido fora de `/^[a-z0-9][a-z0-9-]*$/` → `'Use letras minúsculas, números e hífen, como cond-001.'`
  - `company_id` preenchido fora de `/^\d+$/` → `'Use só números, como 5982.'`
  - `tenant` preenchido fora de `/^[0-9a-f]{24}$/` → `'São 24 caracteres, de 0 a 9 e de a a f.'`

- [ ] **Step 1: Testes que falham**: `identificacao.test.ts` `validatesEachFormat` (tabela: válidos, vazios aceitos, cada inválido com a mensagem); `IdentificacaoSecao.test.tsx`: `gestorCannotEditIdentification` (mantido), `savesIdentificationAutomatically` (substitui `savesIdentificationOnExplicitClick`: digita no nome, 1 s depois sai o `PUT /api/admin/condominio`), `blocksSaveOnInvalidTenant` (tenant `xyz` → mensagem sob o campo e nenhum PUT), `showsIdAndPlaceholders` (id `vila-verde` só leitura; placeholders `cond-001`, `5982`, `69a5a131e67bd8e2f54e5981`), `flagsMissingIds` (chip `Faltam ids de integração`).
- [ ] **Step 2:** `npx vitest run src/condominios/identificacao.test.ts src/condominios/IdentificacaoSecao.test.tsx` → FAIL.
- [ ] **Step 3: Implementar.** Sai o botão Salvar; `useAutosave` com `bloqueio` = `'Não salvo: corrija o campo marcado.'` quando há erro; `SaveIndicator` no `acao` da seção; nome com `required`; `Remover condomínio` fica.
- [ ] **Step 4:** Mesmo comando → PASS.
- [ ] **Step 5:** `git add web/studio/src/condominios` e `git commit -m "feat: validate and autosave condominio identification"`.

---

### Task 12: Drawer da ferramenta completo (front)

**Files:**
- Modify: `web/studio/src/ferramentas/FerramentaDrawer.tsx`, `ferramentas/contratos.ts`, `ferramentas/FerramentasPage.test.tsx` (só os testes que o redesenho quebra)
- Create: `ferramentas/FerramentaDrawer.test.tsx`, `ferramentas/contratos.test.ts`

**Board:** `docs/superpowers/design/studio-fase2/Ferramentas.dc.html` (seções Básico/Parâmetros/Saída/Conexão/Amostra/Ajustes por condomínio, "Código" com helper `grupo.acao`).

**Interfaces:**
- Produces em `ferramentas/contratos.ts`:
  - `export const METODOS: Record<string, string> = { '': '—', GET: 'GET', POST: 'POST', PUT: 'PUT', PATCH: 'PATCH', DELETE: 'DELETE', MCP: 'Servidor de ferramentas' }` (valor técnico só no `.ts`).
  - `export const TIPOS_DE_AJUSTE: Record<string, string> = { string: 'Texto curto', text: 'Texto longo', number: 'Número', boolean: 'Sim/não', select: 'Escolha' }`.
  - `type Cabecalho = { chave: string; valor: string }`; `cabecalhosEmLista(headers: Record<string, string> | undefined): Cabecalho[]`; `listaEmCabecalhos(lista: readonly Cabecalho[]): Record<string, string> | undefined` (chave vazia some; nada sobrando → `undefined`).
  - `padraoDoAjuste(tipo: string | undefined, texto: string): Valor | undefined` (`''` → `undefined`; `number` → `Number` se finito, senão `undefined`; `boolean` → `'sim'`→`true`, `'não'`→`false`; resto → texto).
  - `validarCodigo(codigo: string): string | null` → `/^[a-z0-9_-]+(\.[a-z0-9_-]+)+$/` ou `'Use grupo.acao, em minúsculas, como reservas.criar.'`.
  - `paraGravar` passa a tirar `options` de definição que não é `select`.

- [ ] **Step 1: Testes que falham**: `contratos.test.ts` (`roundTripsHeaders`, `parsesAdjustmentDefault` em tabela, `validatesToolCode` em tabela, `dropsOptionsOutsideChoice`); `FerramentaDrawer.test.tsx` (`savesHeadersAsMap` → corpo `conexao.headers = { Authorization: 'Bearer x' }`, `offersToolServerMethod` → opção `Servidor de ferramentas` grava `metodo: 'MCP'`, `savesOutputFieldType`, `definitionOptionsOnlyForChoice`, `savesDefinitionTypeAndDefault` → `{ type: 'number', default: 3 }`, `rejectsInvalidCodeOnlyWhenChanged` (ferramenta com código `Legado_X` salva se o código não mudou), `requiresTitle` → `'Informe o título.'`, `keepsConnectionCollapsedByDefault`).
- [ ] **Step 2:** `npx vitest run src/ferramentas` → FAIL.
- [ ] **Step 3: Implementar.** Seções em `Accordion` (`disableGutters`, `variant="outlined"`): **Básico** aberta (Título obrigatório; **Código** no lugar de "Nome", helper `'Identificador usado pelas skills, como reservas.criar.'`, validado só quando difere do original; Tipo; Visibilidade; Grava no sistema; Descrição), **Parâmetros**, **Saída** (cada campo com select de tipo de `TIPOS_DE_DADO`) + Resultado principal, **Conexão** fechada (Método de `METODOS`, Endereço, **Cabeçalhos** com `ListaEditavel<Cabecalho>`, Corpo), **Amostra**, **Ajustes por condomínio** (cada definição: nome, rótulo, Tipo de `TIPOS_DE_AJUSTE`, Padrão, Opções só em `select`, descrição). Ajustar em `FerramentasPage.test.tsx` só o que o redesenho mudou (rótulo `Código`, abrir seção antes de preencher).
- [ ] **Step 4:** `npx vitest run src/ferramentas` → PASS; `npm run typecheck`.
- [ ] **Step 5:** `git add web/studio/src/ferramentas` e `git commit -m "feat: complete tool drawer with headers, types and defaults"`.

---

### Task 13: "Quando usar" em dois campos e nova skill com textos (front)

**Files:**
- Modify: `web/studio/src/skills/QuandoUsarAba.tsx`, `skills/SkillsPage.tsx`, `skills/contratos.ts`, `skills/SkillEditorPage.test.tsx`, `skills/SkillsPage.test.tsx`
- Create: `skills/NovaSkillDialog.tsx`

**Board:** `docs/superpowers/design/studio-fase2/SkillQuandoUsar.dc.html` (contador `63/90`, campos "Resumo" e "Como reconhecer o pedido" separados) e `SkillsVazio.dc.html` (diálogo "Nova skill").

**Interfaces:**
- Consumes: `Textos.resumo` (Task 4; Go na Task 2 grava também para skill sem caso).
- Produces: `export const LIMITE_DO_RESUMO = 90` em `skills/contratos.ts`; `preencherTextosNovos(nome: string, novos: { resumo: string; quandoUsar: string }): Promise<void>` (lê `GET /api/skills/:nome`, grava `PUT …/textos` com a versão lida; não chama nada se os dois vierem vazios). `NovaSkillDialog({ onEnviar, onCancelar })` com `onEnviar: (nova: { nome: string; titulo: string; resumo: string; quandoUsar: string }) => Promise<void>`. `NomeDialog` continua só para duplicar.

- [ ] **Step 1: Testes que falham**: `SkillEditorPage.test.tsx`: `savesSummaryOneSecondAfterTyping` (campo `Resumo` → corpo `textos.resumo`), `limitsSummaryTo90Characters` (`maxLength` 90 e contador `` `${n}/90` ``), `savesQuandoUsarOneSecondAfterTyping` (agora no campo `Como reconhecer o pedido`), `showsExamplesWithoutFlow` (substitui `hidesExamplesWithoutFlow`), `addsKeywordAndExamplePhrase` (frase agora entra com Enter, como chip); `SkillsPage.test.tsx`: `createsSkillWithSummaryAndWhenToUse` (POST `/api/skills`, GET textos, PUT textos com `resumo` e `quandoUsar`, abre o editor).
- [ ] **Step 2:** `npx vitest run src/skills/SkillEditorPage.test.tsx src/skills/SkillsPage.test.tsx` → FAIL.
- [ ] **Step 3: Implementar.** `QuandoUsarAba`: seção **Resumo** (ajuda `'Uma linha para as pessoas entenderem o que a skill faz. Aparece para as administradoras.'`, `htmlInput.maxLength = LIMITE_DO_RESUMO`, contador); seção **Como reconhecer o pedido** (ajuda `'Para o concierge decidir quando usar esta skill: em que situação o morador pede isto. Ex.: o morador quer reservar churrasqueira, salão ou quadra para uma data.'`, `aria-label="Como reconhecer o pedido"`); Palavras-chave e Frases de exemplo sempre visíveis, as duas como chips com Enter (um componente local `ListaDeChips` substitui `PalavrasChave` e `FrasesDeExemplo`; frase em branco não entra). `NovaSkillDialog`: Nome (identificador embaixo, como no `NomeDialog`), Resumo (mesmo limite), Como reconhecer o pedido. `SkillsPage` usa o diálogo novo para "Nova skill".
- [ ] **Step 4:** Mesmo comando → PASS; `npm run typecheck`.
- [ ] **Step 5:** `git add web/studio/src/skills` e `git commit -m "feat: split skill summary from when to use"`.

---

### Task 14: Persona com aviso, modelo e tom (front)

**Files:**
- Modify: `web/studio/src/concierge/ConciergePage.tsx`, `concierge/ConciergePage.test.tsx`

**Board:** `docs/superpowers/design/studio-fase2/Concierge.dc.html` (aba "Persona e tom de voz": botões de tom `Acolhedor/Direto/Formal/Descontraído`, ajuda `"Até 3 frases. Use lista quando houver passos."`).

**Interfaces:**
- Consumes: `lerModeloDeArquivo(CAMINHO_PERSONA)` (`catalogo/contratos.ts`).
- Produces: `TONS: Record<'acolhedor' | 'direto' | 'formal' | 'descontraido', string>` em `ConciergePage.tsx` com a frase de tom que cada botão insere (ex.: `acolhedor` → `'Fale como um vizinho prestativo e gentil: frases curtas, calorosas, sem gírias.'`). Clicar num botão troca só a frase de tom dentro do texto (acha e substitui a frase de tom anterior, se houver, senão acrescenta); não mexe no resto da persona. Mesmo autosave de 1000 ms que já existe.

- [ ] **Step 1: Testes que falham**: `warnsWhenPersonaIsEmpty` (`'Sem persona, o concierge responde sem um jeito próprio de falar.'`), `fillsPersonaFromTemplate` (botão `Usar o modelo` → `GET /api/arquivo/modelo?caminho=persona.md` → campo preenchido → 1 s depois `PUT /api/arquivo`), `gestorSeesNoTemplateButton`, `pickingToneReplacesToneSentenceOnly` (`it.each` dos 4 tons: clicar troca só a frase de tom, mantém o resto do texto), `gestorSeesNoToneButtons`.
- [ ] **Step 2:** `npx vitest run src/concierge` → FAIL.
- [ ] **Step 3:** `Alert severity="warning"` acima do campo quando `persona.trim() === ''`; botão só para quem edita; falha do modelo vai para o snackbar com a mensagem do erro; 4 `Chip`/`Button` de tom acima do campo, só para quem edita.
- [ ] **Step 4:** Mesmo comando → PASS.
- [ ] **Step 5:** `git add web/studio/src/concierge` e `git commit -m "feat: warn on empty persona and offer template and tone"`.

---

### Task 15: Orquestração com ajuda por campo (front)

**Files:**
- Modify: `web/studio/src/avancado/OrquestracaoAba.tsx`, `avancado/contratos.ts` (`LIMITES_DE_ORQUESTRACAO`)
- Create: `avancado/OrquestracaoAba.test.tsx` (renderiza a aba direto; `AvancadoPage.test.tsx` é da Task 30)

**Board:** `docs/superpowers/design/studio-fase2/Avancado.dc.html` (aba "Orquestração" dentro do desenho do Avançado; sem mock próprio no canvas, seguir o padrão de `helperText` das outras abas do Avançado).

**Interfaces:**
- Produces: cada item de `LIMITES_DE_ORQUESTRACAO` ganha `ajuda: string`; `MODOS` ganha `quando: string`.

| campo | ajuda |
|---|---|
| bind_confidence | Abaixo disso, o item vira pergunta de confirmação ao morador antes de executar (0,5 a 1). |
| max_itens | Quantos pedidos o plano aceita de uma vez (1 a 10). |
| max_iteracoes | Passos de pensar e agir por mensagem; no limite, junta o que já tem (1 a 15). |

| modo | quando |
|---|---|
| router | Pedidos de um item só. |
| planning | Mensagens com vários pedidos; menos idas e vindas. |
| react | Pedidos abertos, em que o próximo passo depende do resultado do anterior. |

- [ ] **Step 1: Testes que falham**: `showsHelpForEachLimit`, `showsWhenToUseEachMode` (`'Quando: Pedidos de um item só.'`).
- [ ] **Step 2:** `npx vitest run src/avancado/OrquestracaoAba.test.tsx` → FAIL.
- [ ] **Step 3:** `helperText={ajuda}` em cada limite; linha `Quando: …` sob a descrição de cada modo.
- [ ] **Step 4:** Mesmo comando → PASS.
- [ ] **Step 5:** `git add web/studio/src/avancado` e `git commit -m "feat: explain orchestration limits and modes"`.

---

### Task 16: Rota inicial por papel (front)

**Files:**
- Create: `web/studio/src/app/rotaInicial.ts`, `rotaInicial.test.ts`, `app/RotaInicial.tsx`
- Modify: `app/router.tsx` (rota `index` usa `<RotaInicial />`)

**Interfaces:**
- Produces: `rotaInicial(eu: Eu): '/condominios' | '/skills'` → `'/condominios'` quando `eu.administradoraId !== null && pode(eu, 'condominios.habilitar')`, senão `'/skills'`. `RotaInicial` lê `useEu`: carregando → `null`; erro → `<Navigate to="/skills" replace />`; senão `<Navigate to={rotaInicial(eu)} replace />`.

(A regra da análise, "sem `skills.editar_textos`", não serve: o gestor dev tem essa permissão. O que separa o gestor é ter administradora.)

- [ ] **Step 1: Teste que falha**: `rotaInicial.test.ts` `picksStartPageByRole` em tabela: gestor (`administradoraId: 'adm-exemplo'`, com `condominios.habilitar`) → `/condominios`; construtor (`null`) → `/skills`; administradora sem `condominios.habilitar` → `/skills`.
- [ ] **Step 2:** `npx vitest run src/app/rotaInicial.test.ts` → FAIL.
- [ ] **Step 3:** Implementar e ligar no `router.tsx`.
- [ ] **Step 4:** `npx vitest run src/app` → PASS.
- [ ] **Step 5:** `git add web/studio/src/app` e `git commit -m "feat: open condominios first for administradora users"`.

---

### Task 17: "O que o concierge faz aqui" em lista densa (front)

**Files:**
- Modify: `web/studio/src/condominios/SkillsSecao.tsx`, `SkillsSecao.test.tsx`, `condominios/CondominioPage.tsx` (`OrientacoesSecao` sem `titulos`)

**Board:** `docs/superpowers/design/studio-fase2/Condominio.dc.html` (seção "O que o concierge faz aqui": `"8 de 10 ligadas · 2 diferentes do Padrão"`, grupos "Registram no sistema"/"Consultas e dúvidas", switch "Só o que muda do Padrão").

**Interfaces:**
- Consumes: `linhasDeSkills`, `filtrarLinhas`, `agruparPorCategoria`, `contarLigadas`, `rotuloDePapeis`, `ROTULO_DA_CATEGORIA` (Task 4); `iconeDaSkill`; `SkillDoCondominioDrawer` (Task 6).
- Props de `SkillsSecao` não mudam.

Desenho (§5.2 da análise + canvas aprovado, tela Condomínio):
- Cabeçalho da seção: título `"O que o concierge faz aqui"`, `` `${ligadas} de ${total} ligadas` `` (sobre todas as linhas, não as filtradas), `TextField size="small"` com `placeholder="Buscar…"` e `aria-label="Buscar skill"`, `Switch` `"Só o que muda do Padrão"` (escondido na ficha do Padrão).
- Grupos com sobretítulo `overline` (11 px, maiúsculo, `.06em`) na ordem de `agruparPorCategoria`.
- Linha (~52 px): `ListItemButton` com `minHeight: 52`, quadro de ícone 30 × 30 (fundo tint, raio 8, ícone 18 px primário), título peso 600 e, na mesma linha, o resumo em `text.secondary` (`noWrap`, `flex: 1`, dentro de `Tooltip` com o resumo inteiro), chips pequenos só de exceção: `rotuloDePapeis(papeis)` quando não é `null`; `"deste condomínio"` quando `propria` fora do Padrão. Nada de "ligado por padrão" nem "vale para todos". À direita, `ChevronRightOutlined` e o `Switch` 40 × 22 com `aria-label` `` `${titulo} neste condomínio` `` e `onClick` que para a propagação. Desligada: `opacity: 0.6`. Linhas separadas por borda `#EDEFF2`, dentro de um cartão (raio 13, sem sombra).
- Clique na linha abre `SkillDoCondominioDrawer` daquela skill.
- Sem resultado: `"Nenhuma skill encontrada."`.

- [ ] **Step 1: Testes que falham** (`SkillsSecao.test.tsx`; saem `resetsToDefault` e `savesRolesWhenChipIsToggled`, cobertos na Task 6): `togglesSkill` (mantido), `showsCountOfEnabled` (`'1 de 2 ligadas'`), `filtersBySearch`, `showsOnlyChangesFromDefault`, `groupsByCategory` (`'Registra pedidos'`), `showsRoleChipOnlyWhenRestricted` (`'só síndico'`), `showsOwnDecisionChip`, `opensDrawerOnRowClick`, `toggleDoesNotOpenDrawer`, `hidesOnlyChangesToggleOnDefault`.
- [ ] **Step 2:** `npx vitest run src/condominios/SkillsSecao.test.tsx` → FAIL.
- [ ] **Step 3:** Implementar; ajustar `CondominioPage` (ordem da ficha: Skills → Orientações gerais → Ajustes gerais → Identificação).
- [ ] **Step 4:** `npx vitest run src/condominios` → PASS; `npm run typecheck`.
- [ ] **Step 5:** `git add web/studio/src/condominios` e `git commit -m "feat: dense skill list with drawer on condominio page"`.

---

### Task 18: Matriz skills × condomínios (front)

**Files:**
- Create: `web/studio/src/condominios/MatrizDeSkills.tsx`, `MatrizDeSkills.test.tsx`
- Modify: `condominios/CondominiosPage.tsx` (aba `skills`, rótulo `"Visão por skill"`, entre Condomínios e Administradoras, visível para quem tem `condominios.habilitar`)

**Board:** `docs/superpowers/design/studio-fase2/CondominiosMatriz.dc.html` (`"clique na célula para ver papéis, orientação e ajustes"`, ponto de `"decisão deste condomínio"`, `"sem ponto: segue o Padrão"`).

**Interfaces:**
- Consumes: `habilitacaoDe`, `salvarHabilitacao`, `PADRAO`, `agruparPorCategoria`, `linhasDeSkills` (para ordem e títulos), `SkillDoCondominioDrawer`, `useExecutar`.
- Produces: `MatrizDeSkills({ estado, titulos, editavel }: { estado: Estado; titulos: ReadonlyMap<string, string>; editavel: boolean }): ReactNode`.

Tabela com rolagem horizontal; primeira coluna (skill) fixa (`position: sticky`); colunas `Padrão` + cada condomínio do estado. Célula: `Switch size="small"` com `aria-label` `` `${titulo} em ${nomeDaColuna}` `` (grava `habilitado` mantendo `papeis`) e, quando a decisão é própria, um ponto de 6 px `primary.main` com `Tooltip` `"Decisão deste condomínio"` e `aria-label` igual. Clique na célula fora do switch abre o drawer daquela skill naquele condomínio. Coluna Padrão travada quando o escopo não é `null` (a mesma regra de `CondominioPage`).

- [ ] **Step 1: Testes que falham**: `togglesCellForCondominio`, `marksOwnDecision`, `opensDrawerFromCell`, `locksDefaultColumnForGestor`, `opensMatrixTabFromQuery` (`?aba=skills`).
- [ ] **Step 2:** `npx vitest run src/condominios/MatrizDeSkills.test.tsx` → FAIL.
- [ ] **Step 3:** Implementar.
- [ ] **Step 4:** `npx vitest run src/condominios` → PASS.
- [ ] **Step 5:** `git add web/studio/src/condominios` e `git commit -m "feat: add skills by condominio matrix"`.

---

### Task 19: Hub de skills: números, grupos, ícone, resumo, problemas e busca (front)

**Files:**
- Modify: `web/studio/src/skills/SkillCard.tsx`, `skills/SkillsPage.tsx`, `skills/SkillsPage.test.tsx`, `avancado/contratos.ts` (`useArvore`)

**Board:** `docs/superpowers/design/studio-fase2/Main.dc.html` (4 cartões de número, grupos "Registram no sistema"/"Consultas e dúvidas") e `SkillsVazio.dc.html` (estado vazio; os chips "Ou peça ao Copiloto" são da Task 28).

**Interfaces:**
- Produces: `useArvore(opcoes?: { enabled?: boolean })`; `SkillCard` ganha `variante: 'completo' | 'compacto'` e `problemas?: { erros: number; avisos: number }` (ausente = não mostra).

Desenho (canvas, tela Skills): 4 cartões de número no topo (título 12 px `text.tertiary`, número 26 px display 700): `Skills` (total), `Registram no sistema` (`escrita`), `Consultas e dúvidas` (resto), `Com problema` (skills com erro ou aviso; só para quem edita fluxo, para os outros o 4º cartão é `Ferramentas`, o total de `recursos` distintos das skills). Abaixo, grupo `Registram no sistema` em 2 colunas de cartões `completo` (quadro de ícone 42 × 42 tint raio 12, título 15 px 600, resumo de uma linha, chips) e grupo `Consultas e dúvidas` em 3 colunas de cartões `compacto` (ícone 30, título, resumo). Grupo vazio some.

- [ ] **Step 1: Testes que falham**: `showsFourStatCards` (`Skills 2`, `Registram no sistema 2`…), `groupsWritingSkillsFirst`, `showsSummaryAndIcon` (cartão mostra `resumo`, não `descricao`), `filtersSkillsBySearch` (`aria-label="Buscar skill"`, título e resumo; os números não mudam com a busca), `showsProblemCountForBuilder` (nó da árvore com `erros: 1` → chip `'1 erro'`), `hidesProblemCountForGestor` (e não chama `GET /api/agentes`).
- [ ] **Step 2:** `npx vitest run src/skills/SkillsPage.test.tsx` → FAIL.
- [ ] **Step 3: Implementar.** Ícone de `iconeDaSkill(skill.icone)`; resumo no lugar da descrição (vazio → nada); sai o chip "N ferramentas"; chips `'N erro(s)'` (`color="error"`) e `'N aviso(s)'` (texto `#8A5A00` em `#FFF6DF`) só com contagem > 0 e só para `usePodeEditar('skills.editar_fluxo')`, cruzando `no.caminho === skill.caminho`; busca acima dos grupos.
- [ ] **Step 4:** Mesmo comando → PASS.
- [ ] **Step 5:** `git add web/studio/src/skills web/studio/src/avancado/contratos.ts` e `git commit -m "feat: show icon, summary and problems on skill cards"`.

---

### Task 20: Gestor não vê descrição técnica nem id de informação (front)

**Files:**
- Modify: `web/studio/src/ferramentas/FerramentasPage.tsx`, `ferramentas/FerramentasPage.test.tsx`, `skills/abas/FerramentasAba.tsx`, `skills/abas/FerramentasAba.test.tsx`, `fluxo/painel/InformacoesPainel.tsx`, `skills/abas/FluxoAba.test.tsx`

**Board:** `docs/superpowers/design/studio-fase2/Ferramentas.dc.html`, `SkillFerramentas.dc.html` (aba Ferramentas da skill, sem descrição técnica para o gestor) e `FluxoGestor.dc.html` (painel de Informações sem o id técnico).

- [ ] **Step 1: Testes que falham**: `hidesToolDescriptionFromGestor` (página Ferramentas), `hidesToolDescriptionInSkillFromGestor` (aba Ferramentas da skill), `hidesInformationIdWhenReadOnly` (painel de Informações sem `unidade_id` para quem não edita).
- [ ] **Step 2:** `npx vitest run src/ferramentas src/skills/abas` → FAIL.
- [ ] **Step 3:** Descrição da ferramenta só com `usePodeEditar('ferramentas.editar')` (a busca continua olhando a descrição); `info.nome` só quando `editavel`; placeholder da Pergunta `'Ex.: Para que dia é a reserva?'`. Sem título, a ferramenta aparece como `'Ferramenta sem nome'` para quem não edita.
- [ ] **Step 4:** Mesmo comando → PASS.
- [ ] **Step 5:** `git add web/studio/src` e `git commit -m "fix: hide technical tool text and ids from gestores"`.

---

### Task 21: Identidade técnica no Avançado (front)

**Files:**
- Create: `web/studio/src/avancado/IdentidadeAba.tsx`, `avancado/IdentidadeAba.test.tsx`
- Modify: `avancado/AvancadoPage.tsx` (aba `{ id: 'identidade', rotulo: 'Identidade', porItem: true }` depois de Arquivos), `catalogo/contratos.ts`

**Board:** `docs/superpowers/design/studio-fase2/Avancado.dc.html` (aba "Identidade": `tipo`, `modelo`, `nome`, `descricao`).

**Interfaces:**
- Consumes: `PUT /api/agente/config` (Task 1); `useTextos`, `salvarTextos` (`visibilidade`, Task 2/4).
- Produces em `catalogo/contratos.ts`: `AgenteSchema.agente` + `tipo: z.string()`, `tipo_declarado: z.boolean()`, `modelo: z.string().default('')`; `salvarConfigDoAgente(c: { agente: string; campos: { tipo: string; modelo: string } }): Promise<void>` (`PUT /api/agente/config`, resposta `{ salvo: string }`).

Formulário com botão `Salvar` (padrão do Avançado): **Tipo** (`''` `Automático`, `orquestrador` `Orquestrador`, `operador` `Operador`; valor inicial `tipo_declarado ? tipo : ''`), **Modelo** (texto livre, helper `'Vazio usa o modelo padrão.'`), **Visibilidade** só quando `no.caso` (`private` `Só morador identificado`, `public` `Qualquer contato`; nome da skill = último segmento de `no.caminho`; grava por `salvarTextos` com a versão lida). Somente leitura sem `studio.avancado` de edição.

- [ ] **Step 1: Testes que falham**: `savesTypeAndModel`, `savesVisibilityForSkill`, `hidesVisibilityWithoutFlow`, `readOnlyWithoutEditPermission`.
- [ ] **Step 2:** `npx vitest run src/avancado/IdentidadeAba.test.tsx` → FAIL.
- [ ] **Step 3:** Implementar.
- [ ] **Step 4:** `npx vitest run src/avancado` → PASS.
- [ ] **Step 5:** `git add web/studio/src/avancado web/studio/src/catalogo` e `git commit -m "feat: edit skill type, model and visibility in advanced"`.

---

### Task 22: Ajustes técnicos por condomínio no Avançado (front)

**Files:**
- Create: `web/studio/src/avancado/AjustesTecnicosAba.tsx`, `avancado/AjustesTecnicosAba.test.tsx`, `avancado/payloadLivre.ts`, `avancado/payloadLivre.test.ts`
- Modify: `avancado/AvancadoPage.tsx` (aba `{ id: 'ajustes', rotulo: 'Ajustes por condomínio (técnico)', porItem: false }`)

**Board:** `docs/superpowers/design/studio-fase2/Avancado.dc.html` (aba "Ajustes por condomínio (técnico)", chip `"Mudanças aqui afetam todos os condomínios"`).

**Interfaces:**
- Consumes: `useEstado`, `salvarPayload`, `PADRAO` (Task 4); `ListaEditavel` (`ferramentas/ListaEditavel.tsx`).
- Produces: `type ParDoPayload = { chave: string; valor: string }`; `payloadEmPares(payload: Record<string, Valor>): ParDoPayload[]` (valor não texto vira `JSON.stringify`); `paresEmPayload(pares: readonly ParDoPayload[]): Record<string, Valor>` (chave vazia some; `'true'`/`'false'` → boolean; número finito → number; resto → texto).

Select de condomínio (Padrão + condomínios do escopo); tabela com todo `estado.recursos`: título (ou código), chip `próprio` quando há payload do condomínio, `herdado` quando só o Padrão tem; clique abre Drawer com `ListaEditavel<ParDoPayload>`, `Salvar` (`apagar: false`) e `Limpar deste condomínio` (`apagar: true`, com `ConfirmDialog`).

- [ ] **Step 1: Testes que falham**: `payloadLivre.test.ts` `convertsPairsAndValues` (tabela), `roundTripsPayload`; `AjustesTecnicosAba.test.tsx`: `marksOwnAndInherited`, `savesFreePayload` (corpo `{ condominio: 'vila-verde', recurso: 'reserva-salao', payload: { limite: 3 }, apagar: false }`), `clearsPayloadForCondominio`.
- [ ] **Step 2:** `npx vitest run src/avancado/payloadLivre.test.ts src/avancado/AjustesTecnicosAba.test.tsx` → FAIL.
- [ ] **Step 3:** Implementar.
- [ ] **Step 4:** `npx vitest run src/avancado` → PASS.
- [ ] **Step 5:** `git add web/studio/src/avancado` e `git commit -m "feat: edit free per condominio payloads in advanced"`.

---

### Task 23: Operações do Copiloto, telas e permissões (Go, puro)

**Files:**
- Create: `internal/copiloto/operacoes.go`, `internal/copiloto/telas.go`, `internal/copiloto/operacoes_test.go`, `internal/copiloto/telas_test.go`

**Interfaces:**
- Produces:
  ```go
  type TipoDeOperacao string
  const (
  	PreencherTextos TipoDeOperacao = "preencher_textos" // skills.editar_textos
  	TrocarFluxo     TipoDeOperacao = "trocar_fluxo"     // skills.editar_fluxo (inclui pôr, tirar e reordenar blocos)
  	CriarSkill      TipoDeOperacao = "criar_skill"      // skills.criar
  	ExcluirSkill    TipoDeOperacao = "excluir_skill"    // skills.excluir (o front pede confirmação)
  	HabilitarSkill  TipoDeOperacao = "habilitar_skill"  // condominios.habilitar
  	Orientar        TipoDeOperacao = "orientar"         // condominios.habilitar (Skill vazia = orientação geral)
  	Ajustar         TipoDeOperacao = "ajustar"          // condominios.habilitar
  	EditarPersona   TipoDeOperacao = "editar_persona"   // skills.editar_textos
  	EditarArquivo   TipoDeOperacao = "editar_arquivo"   // studio.avancado
  )
  // Operacao é uma união discriminada por Tipo; só os campos do tipo vêm preenchidos.
  type Operacao struct {
  	Tipo        TipoDeOperacao  `json:"tipo"`
  	Skill       string          `json:"skill,omitempty"`
  	Textos      *TextosParciais `json:"textos,omitempty"`
  	Fluxo       *okf.Fluxo      `json:"fluxo,omitempty"`
  	Titulo      string          `json:"titulo,omitempty"`
  	Condominio  string          `json:"condominio,omitempty"`
  	Habilitado  *bool           `json:"habilitado,omitempty"`
  	Papeis      []string        `json:"papeis,omitempty"`
  	Recurso     string          `json:"recurso,omitempty"`
  	Campos      map[string]any  `json:"campos,omitempty"` // payload dinâmico do recurso (exceção da constituição)
  	Texto       string          `json:"texto,omitempty"`
  	Agente      string          `json:"agente,omitempty"`
  	Caminho     string          `json:"caminho,omitempty"`
  	Conteudo    string          `json:"conteudo,omitempty"`
  }
  type TextosParciais struct { Titulo, Resumo, QuandoUsar *string; PalavrasChave, FrasesDeExemplo []string } // tags json: titulo, resumo, quandoUsar, palavrasChave, frasesDeExemplo
  type Tela string // "skills" | "skill" | "fluxo" | "condominios" | "condominio" | "concierge" | "ferramentas" | "avancado"
  type Contexto struct { Tela Tela `json:"tela"`; Skill string `json:"skill,omitempty"`; Condominio string `json:"condominio,omitempty"`; Agente string `json:"agente,omitempty"`; Caminho string `json:"caminho,omitempty"` }
  type Quem struct { Permissoes []string; Administradora string } // "" = Superlógica
  type Recusa struct { Indice int `json:"indice"`; Motivo string `json:"motivo"` }

  func PermissaoDe(t TipoDeOperacao) string
  func OperacoesDaTela(t Tela) []TipoDeOperacao
  func ValidarForma(op Operacao) error // campos obrigatórios do tipo
  func Filtrar(ops []Operacao, ctx Contexto, quem Quem, condominiosDoEscopo func(id string) bool) (aceitas []Operacao, recusas []Recusa)
  func ValidarFluxoProposto(f okf.Fluxo, escrita func(string) bool) []okf.ProblemaFluxo // Reclassificar + ValidarFluxo
  ```
- Operações por tela (`OperacoesDaTela`): `skills` → criar_skill, excluir_skill, preencher_textos; `skill` → preencher_textos; `fluxo` → trocar_fluxo, preencher_textos; `condominios` → habilitar_skill, orientar; `condominio` → habilitar_skill, orientar, ajustar; `concierge` → editar_persona; `ferramentas` → nenhuma (só explica); `avancado` → editar_arquivo.
- `Filtrar` recusa, com motivo em PT-BR simples: tipo fora da tela (`"Esta tela não faz isso."`), sem a permissão (`"Você não tem permissão para isso."`), condomínio fora do escopo ou, na tela `condominio`, diferente do aberto (`"Esse condomínio não está com você."`), catálogo (`preencher_textos`, `trocar_fluxo`, `criar_skill`, `excluir_skill`, `editar_persona`) pedido por quem tem administradora (`"Só a Superlógica muda isso."`, a mesma regra de `podeNoCatalogo` do front), forma inválida (`"Proposta incompleta."`).

- [ ] **Step 1: Testes que falham**: `TestPermissaoDeCadaOperacao` (tabela dos 9 tipos), `TestOperacoesDaTela` (tabela das 8 telas), `TestValidarForma` (tabela: `habilitar_skill` sem `Habilitado` → erro; `orientar` sem `Condominio` → erro; `editar_arquivo` sem `Caminho` → erro), `TestFiltrarPorPermissaoEEscopo` (gestor de `adm-exemplo` com as permissões de `euDev("gestor")`: `habilitar_skill` em condomínio dele aceita; em condomínio de outra administradora recusa; `trocar_fluxo` recusa; `editar_arquivo` recusa), `TestFiltrarForaDaTela`, `TestValidarRecusaFluxoSemConfirmar` (fluxo com Gravar sem Confirmar antes → problema de nível `erro`).
- [ ] **Step 2:** `go test ./internal/copiloto/ -v` → FAIL.
- [ ] **Step 3:** Implementar com mapas (`map[TipoDeOperacao]string`, `map[Tela][]TipoDeOperacao`), sem `switch` longo; tudo puro.
- [ ] **Step 4:** Mesmo comando → PASS.
- [ ] **Step 5:** `git add internal/copiloto` e `git commit -m "feat: add typed copilot operations with screen and role rules"`.

---

### Task 24: `POST /api/copiloto` com o modelo do provedor (Go)

**Files:**
- Create: `internal/harness/copiloto.go`, `internal/harness/copiloto_test.go`, `cmd/okf-admin/copiloto.go`, `cmd/okf-admin/copiloto_test.go`
- Modify: `cmd/okf-admin/main.go` (campo `copiloto` no `servidor`, montado com `harness.NovoCopiloto(s.atende.Config)`; `s.rotasCopiloto(mux)` junto das outras rotas)

**Interfaces:**
- Consumes: `internal/copiloto` (Task 23); `euDev` (`studio.go`); `s.cat.LerTextos`, `s.cat.LerFluxo`, `s.admin.Ler().Visao(...)`, `s.casosDeUso()`; `ResolverProvedor`, `novoModelo` (`internal/harness/provedor.go`).
- Produces:
  ```go
  // internal/harness
  type MensagemDoCopiloto struct { Papel string `json:"papel"`; Texto string `json:"texto"` } // "pessoa" | "copiloto"
  type PedidoAoCopiloto struct { Mensagem string; Historico []MensagemDoCopiloto; Contexto string; Operacoes []copiloto.TipoDeOperacao }
  type RespostaDoModelo struct { Mensagem string `json:"mensagem"`; Operacoes []copiloto.Operacao `json:"operacoes"` }
  type Copiloto func(ctx context.Context, p PedidoAoCopiloto) (RespostaDoModelo, error)
  func NovoCopiloto(cfg Config) (Copiloto, bool) // false: sem provedor (mock ou sem chave)
  func LerRespostaDoModelo(raw string) (RespostaDoModelo, error) // aceita JSON puro ou dentro de ```json; puro
  ```
  Modelo: `MODEL_COPILOTO`, senão `MODEL_PLANNER`, senão o do provedor; prazo 90 s (`novoModelo(..., 90*time.Second)`). O prompt diz quem é o Copiloto do Studio, o vocabulário das Global Constraints (sem termo técnico fora do Avançado), as operações permitidas nesta chamada com a forma JSON de cada uma, e pede só JSON `{"mensagem": "...", "operacoes": [...]}`.
- HTTP:
  - `GET /api/copiloto/estado` → `{"disponivel": bool}`.
  - `POST /api/copiloto` corpo `{ "mensagem": string (1..4000), "historico": MensagemDoCopiloto[] (até 20), "contexto": copiloto.Contexto }`, cabeçalho `X-Studio-Papel` como `/api/eu`. Resposta 200 `{ "mensagem": string, "propostas": [{ "id": string, "resumo": string, "operacao": Operacao }], "recusas": Recusa[] }`. `id` = índice em texto; `resumo` é uma linha legível gerada no Go por tipo (ex.: `"Ligar Reserva de Área Comum em Vila Verde"`). Erros: 400 `PEDIDO_INVALIDO`; 503 `COPILOTO_INDISPONIVEL` (`"O Copiloto está fora do ar. O resto do Studio continua funcionando."`); 504 `COPILOTO_DEMOROU` quando o prazo estoura.
  - O contexto mandado ao modelo é montado no servidor, nunca vindo do cliente: tela `skill`/`fluxo` → textos e fluxo da skill; `condominio` → habilitações, orientação e ajustes daquele condomínio já filtrados pela `Visao` do escopo; `skills` → lista de skills com resumo; `avancado` → conteúdo do arquivo aberto (só com `studio.avancado`).
  - Depois do modelo: `copiloto.Filtrar`; cada `trocar_fluxo` aceito passa por `ValidarFluxoProposto` com o classificador do catálogo e, com problema de nível `erro`, vira recusa `"A proposta deixa o fluxo com erro: <texto do primeiro problema>"`.

- [ ] **Step 1: Testes que falham**: `internal/harness/copiloto_test.go` `TestLerRespostaDoModelo` (tabela: JSON puro, dentro de cerca, lixo → erro). `cmd/okf-admin/copiloto_test.go` (servidor como `handlerDeTeste` + `admin` em pasta temporária + `copiloto` falso que devolve operações fixas): `TestCopilotoIndisponivel` (campo nil → 503 e `estado` com `disponivel: false`), `TestCopilotoDevolvePropostasDaTela` (tela `condominio`, falso devolve `habilitar_skill` → 200 com uma proposta e `resumo`), `TestCopilotoNaoPropoeAlemDoPapel` (cabeçalho `gestor`, falso devolve `trocar_fluxo` e `habilitar_skill` em condomínio de outra administradora → 0 propostas, 2 recusas), `TestCopilotoRecusaFluxoComErro`, `TestCopilotoMandaContextoDoServidor` (o falso recebe no `Contexto` o título da skill aberta), `TestCopilotoRecusaMensagemVazia` (400).
- [ ] **Step 2:** `go test ./internal/harness/ -run TestLerRespostaDoModelo -v && go test ./cmd/okf-admin/ -run TestCopiloto -v` → FAIL.
- [ ] **Step 3:** Implementar. Handler fino: decodifica, resolve `eu`, monta contexto, chama, filtra, responde; erros com contexto (`"copiloto: montar contexto da skill %s: %w"`), log sem conteúdo da conversa.
- [ ] **Step 4:** Mesmo comando → PASS; `go build ./...`.
- [ ] **Step 5:** `git add internal/harness cmd/okf-admin` e `git commit -m "feat: add studio copilot endpoint"`.

---

### Task 25: Painel do Copiloto e botão no header (front)

**Files:**
- Create: `web/studio/src/copiloto/contratos.ts`, `contratos.test.ts`, `CopilotoProvider.tsx`, `PainelDoCopiloto.tsx`, `PainelDoCopiloto.test.tsx`, `BolhaDeMensagem.tsx`, `Compositor.tsx`, `EstadoVazio.tsx`, `CartaoDeProposta.tsx`
- Modify: `app/Topbar.tsx` (botão), `app/StudioLayout.tsx` (provider + painel à esquerda do `main`), `app/StudioLayout.test.tsx`

**Board:** `docs/superpowers/design/studio-fase2/Copiloto.dc.html` (painel aberto na skill, referência principal do layout, bolhas e cartões de proposta) e, para o botão no header em cada tela, `ConciergeCopiloto.dc.html`, `CondominioCopiloto.dc.html`, `AvancadoCopiloto.dc.html`.

**Interfaces:**
- Produces em `copiloto/contratos.ts` (espelho do Go da Task 23/24, Zod na borda): `TipoDeOperacaoSchema` (`z.enum` dos 9 tipos), `OperacaoSchema` (`z.discriminatedUnion('tipo', …)` com os campos de cada tipo), `PropostaSchema = { id, resumo, operacao }`, `RespostaDoCopilotoSchema = { mensagem, propostas, recusas: { indice, motivo }[] }`, `ContextoDoCopiloto` (`{ tela: Tela; skill?: string; condominio?: string; agente?: string; caminho?: string }`), `LIMITE_DA_MENSAGEM = 4000`, `TAMANHO_DO_HISTORICO = 20`, `PRAZO_DO_COPILOTO_MS = 95_000`; `conversar(corpo, sinal: AbortSignal): Promise<RespostaDoCopiloto>`; `useEstadoDoCopiloto()` (`GET /api/copiloto/estado`).
- Produces em `CopilotoProvider.tsx`:
  ```ts
  type Status = 'pronto' | 'pensando' | 'indisponivel';
  type Mensagem = { id: string; papel: 'pessoa' | 'copiloto'; texto: string; propostas: Proposta[]; tom: 'normal' | 'erro' };
  export function useCopiloto(): {
    aberto: boolean; alternar: () => void; status: Status; mensagens: Mensagem[];
    enviar: (texto: string) => Promise<void>;
    aplicar: (proposta: Proposta) => Promise<void>;   // Task 26 implementa; aqui lança 'ainda não aplica'
    descartar: (idDaProposta: string) => void;
    definirContexto: (contexto: ContextoDoCopiloto) => void; // Task 28 chama em cada tela
  };
  export function useRealceDoCopiloto(alvo: string): SxProps<Theme>; // fundo #FFF7CC por 1500 ms quando `alvo` foi aplicado
  ```
  `aberto` lembrado em `localStorage` (`studio.copilotoAberto`, preferência de tela, com `try/catch` como o menu); conversa só na memória da sessão.
- Desenho (canvas, Skill › Copiloto aberto; portado de `CopilotPanel`, `MessageBubble`, `CopilotComposer`, `CopilotEmptyState`, `ProposalsDialog` do Atende Studio): botão `Copiloto` no header, à direita, `aria-pressed`, contornado quando fechado e **preenchido navy** quando aberto; painel de 380 px à esquerda do conteúdo, borda direita, cabeçalho com título `Copiloto` em navy e o status (`Pronto` verde, `Pensando…`, `Indisponível` com aviso `"O Copiloto está fora do ar. O resto do Studio continua funcionando."`); lista de mensagens (bolha da pessoa em tint à direita, do Copiloto em superfície à esquerda; erro em perigo); cartões de proposta com o `resumo`, botões `Aplicar` (primário) e `Descartar`; compositor com `Enter` envia, `Shift+Enter` quebra linha, contador quando passa de 3500, desabilitado em `pensando` e `indisponivel`. O painel fica dentro de um `ErrorBoundary`: se quebrar, mostra `"O Copiloto teve um problema. Feche e abra de novo."` e o resto da tela segue.

- [ ] **Step 1: Testes que falham**: `contratos.test.ts` (`parsesEveryOperationType` com uma operação de cada tipo, `rejectsUnknownOperation`, `keepsLast20MessagesInHistory`); `PainelDoCopiloto.test.tsx` (`togglesFromHeaderButton` com `aria-pressed`, `sendsOnEnterAndBreaksLineOnShiftEnter`, `showsThinkingWhileWaiting`, `showsUnavailableAndDisablesComposer` (estado `disponivel: false`), `showsProposalCards`, `discardsProposal`, `keepsStudioWorkingWhenCopilotCrashes`, `cancelsRequestOnClose` (AbortController)); `StudioLayout.test.tsx` (`placesCopilotLeftOfContent`).
- [ ] **Step 2:** `npx vitest run src/copiloto src/app` → FAIL.
- [ ] **Step 3:** Implementar; nenhuma chamada sem prazo (`AbortSignal.any([sinal, AbortSignal.timeout(PRAZO_DO_COPILOTO_MS)])`).
- [ ] **Step 4:** Mesmo comando → PASS; `npm run typecheck`.
- [ ] **Step 5:** `git add web/studio/src` e `git commit -m "feat: add copilot panel and header toggle"`.

---

### Task 26: Aplicar propostas do Copiloto (front)

**Files:**
- Create: `web/studio/src/copiloto/aplicar.ts`, `aplicar.test.ts`
- Modify: `copiloto/CopilotoProvider.tsx` (`aplicar` real), `copiloto/CartaoDeProposta.tsx` (confirmação de exclusão, estado aplicada/erro)

**Board:** `docs/superpowers/design/studio-fase2/Copiloto.dc.html` (cartão `"Preparei duas mudanças no passo 3. Confere antes de eu aplicar:"`, botões `Aplicar`/`Descartar`, texto `"Pronto"`).

**Interfaces:**
- Consumes: `salvarTextos`, `useTextos`/`apiGet` de textos, `salvarFluxo`, `useCriarSkill`, `useExcluirSkill`, `preencherTextosNovos` (Task 13) de `skills/contratos.ts`; `salvarHabilitacao`, `salvarOrientacao`, `salvarAjustes` (`condominios/contratos.ts`); `salvarArquivo`, `CAMINHO_PERSONA` (`catalogo/contratos.ts`).
- Produces:
  ```ts
  type Aplicacao = { alvos: string[]; invalidar: QueryKey[] }; // alvos: ids de realce, ex. 'skill:agendar-mudanca:resumo', 'condominio:vila-verde:skill:encomendas'
  export const APLICADORES: { [T in TipoDeOperacao]: (op: Extract<Operacao, { tipo: T }>, ambiente: Ambiente) => Promise<Aplicacao> };
  export async function aplicarOperacao(op: Operacao, ambiente: Ambiente): Promise<Aplicacao>;
  type Ambiente = { escopo: Escopo; lerEstado: () => Estado | undefined; confirmar: (texto: string) => Promise<boolean> };
  ```
  Cada aplicador lê a versão atual antes de gravar (textos e fluxo usam a `versao` do GET; 409 vira mensagem `"Esta skill mudou desde a proposta. Peça de novo ao Copiloto."`); `orientar` e `ajustar` mesclam com o que está no estado, como as telas (Review Focus 1 e 2); `excluir_skill` só grava depois de `confirmar('Excluir "<título>"? O fluxo e as regras somem junto.')`. O servidor valida de novo (422 do fluxo, permissão da Fundação): erro de gravação vira a mensagem do erro no cartão, nada é engolido.
- O cartão só mostra `Aplicar` quando o front também acha que a pessoa pode (`pode`/`podeNoCatalogo` com a permissão do tipo); senão mostra `"Você não pode aplicar esta."` (defesa em dobro; o servidor já filtra).

- [ ] **Step 1: Testes que falham** (`aplicar.test.ts`, com `responderCom`): `appliesTextsWithCurrentVersion`, `refusesFlowRejectedByServer` (422 → erro no cartão, nada aplicado), `asksBeforeDeletingSkill` (confirmar `false` → nenhuma chamada), `mergesGuidanceWithCurrentState`, `mergesAdjustmentsWithCurrentState`, `togglesSkillKeepingRoles`, `writesPersonaFile`, `writesAdvancedFile`, `returnsHighlightTargets` (tabela tipo → alvos).
- [ ] **Step 2:** `npx vitest run src/copiloto/aplicar.test.ts` → FAIL.
- [ ] **Step 3:** Implementar `APLICADORES` como mapa tipado (um aplicador por tipo, nenhum `switch`); no provider, `aplicar` roda `aplicarOperacao`, invalida as consultas, marca os alvos para `useRealceDoCopiloto` e troca o cartão para `Aplicada`.
- [ ] **Step 4:** `npx vitest run src/copiloto` → PASS.
- [ ] **Step 5:** `git add web/studio/src/copiloto` e `git commit -m "feat: apply copilot proposals through studio saves"`.

---

### Task 27: Sugestões no estado vazio do Copiloto (front)

**Files:**
- Create: `web/studio/src/copiloto/sugestoes.ts`, `sugestoes.test.ts`
- Modify: `copiloto/EstadoVazio.tsx`

**Board:** `docs/superpowers/design/studio-fase2/ConciergeCopiloto.dc.html` (seção "Sugestões": `"Escreva boas-vindas mais curtas"`, `"Revise a persona para respostas curtas"`) e `Copiloto.dc.html`.

**Interfaces:**
- Produces: `sugestoesPara(contexto: ContextoDoCopiloto, eu: Eu): string[]` (até 3, só o que a pessoa pode pedir). Textos:

| tela | sugestões |
|---|---|
| skills | "Crie uma skill para reservar a quadra" · "Quais skills gravam no sistema?" · "Escreva o resumo das skills que estão sem" |
| skill | "Melhore o resumo desta skill" · "Sugira frases de exemplo" · "Explique quando esta skill é usada" |
| fluxo | "Explique este fluxo passo a passo" · "Adicione um Confirmar antes de gravar" · "Pergunte o horário antes de reservar" |
| condominios | "Quais condomínios estão sem ids?" · "Desligue a reserva de área comum onde não há área" |
| condominio | "O que muda deste condomínio para o Padrão?" · "Escreva uma orientação sobre mudanças aos domingos" · "Ajuste a antecedência da reserva para 3 dias" |
| concierge | "Deixe a persona mais acolhedora" · "Revise a persona para respostas curtas" |
| ferramentas | "Explique o que esta ferramenta faz" · "Quais skills usam esta ferramenta?" |
| avancado | "Explique este arquivo" · "Revise este arquivo e proponha melhorias" |

  Sugestões que levam a uma operação que a pessoa não pode aplicar somem (ex.: gestor em `skills` só vê a segunda).

- [ ] **Step 1: Testes que falham**: `suggestsByScreen` (`it.each` das 8 telas, construtor), `hidesSuggestionsGestorCannotApply`, `clickingSuggestionSendsIt` (no `EstadoVazio`).
- [ ] **Step 2:** `npx vitest run src/copiloto/sugestoes.test.ts` → FAIL.
- [ ] **Step 3:** Implementar; cada sugestão é um `Chip` clicável que chama `enviar(texto)`; título do estado vazio `"Como posso ajudar?"` em navy.
- [ ] **Step 4:** `npx vitest run src/copiloto` → PASS.
- [ ] **Step 5:** `git add web/studio/src/copiloto` e `git commit -m "feat: suggest copilot requests per screen"`.

---

### Task 28: Contexto do Copiloto em cada tela (front)

**Files:**
- Modify: `web/studio/src/skills/SkillsPage.tsx`, `skills/SkillEditorPage.tsx`, `skills/QuandoUsarAba.tsx`, `skills/abas/FluxoAba.tsx`, `condominios/CondominiosPage.tsx`, `condominios/CondominioPage.tsx`, `condominios/SkillsSecao.tsx`, `condominios/OrientacoesSecao.tsx`, `concierge/ConciergePage.tsx`, `ferramentas/FerramentasPage.tsx`, `avancado/ArquivosAba.tsx`
- Create: `web/studio/src/copiloto/contextoPorTela.test.tsx`

**Board:** `docs/superpowers/design/studio-fase2/ConciergeCopiloto.dc.html`, `CondominioCopiloto.dc.html`, `AvancadoCopiloto.dc.html` (contexto de cada tela e realce `#FFF7CC`) e `SkillsVazio.dc.html` (chips `"Crie uma skill de 2ª via de boleto"` / `"Crie uma skill de agendamento de mudança"` / `"Crie uma skill para reservar o salão"` no estado vazio do hub, não no painel).

**Interfaces:**
- Consumes: `useCopiloto().definirContexto`, `useRealceDoCopiloto` (Task 25); `sugestoesPara` (Task 27).
- Produces: cada tela chama `definirContexto` ao montar e quando o item muda (dentro de `useEffect` com as dependências do contexto):

| tela | contexto | realce (`alvo`) |
|---|---|---|
| `SkillsPage` | `{ tela: 'skills' }` | cartão `skill:<nome>` |
| `SkillEditorPage` (aba ≠ fluxo) | `{ tela: 'skill', skill }` | campos `skill:<nome>:resumo`, `:quandoUsar`, `:palavrasChave`, `:frasesDeExemplo` |
| `FluxoAba` | `{ tela: 'fluxo', skill }` | bloco `skill:<nome>:bloco:<id>` |
| `CondominiosPage` | `{ tela: 'condominios' }` | linha `condominio:<id>` |
| `CondominioPage` | `{ tela: 'condominio', condominio: id }` | linha `condominio:<id>:skill:<nome>`, campo `condominio:<id>:orientacao` |
| `ConciergePage` | `{ tela: 'concierge' }` | campo `persona` |
| `FerramentasPage` | `{ tela: 'ferramentas' }` | nenhum |
| `ArquivosAba` | `{ tela: 'avancado', agente, caminho }` (só com `studio.avancado`) | editor `arquivo:<agente>:<caminho>` |

  No estado vazio do hub (`SkillsPage` sem skills, `SkillsVazio.dc.html`), mostrar `"Ou peça ao Copiloto"` com os 3 `Chip` de `sugestoesPara({ tela: 'skills' }, eu)`; clicar abre o painel (`useCopiloto().alternar()` se fechado) e chama `enviar(texto)` direto, sem esperar a pessoa digitar de novo.

- [ ] **Step 1: Testes que falham** em `contextoPorTela.test.tsx`, uma tela por teste, com o `PainelDoCopiloto` aberto e `POST /api/copiloto` gravado: `sendsSkillsHubContext`, `sendsSkillEditorContext`, `sendsFlowContext`, `sendsCondominiosContext`, `sendsCondominioContext`, `sendsConciergeContext`, `sendsToolsContext`, `sendsAdvancedFileContext`, `omitsAdvancedContextWithoutPermission` (gestor sem `studio.avancado`), `highlightsAppliedSkillRow` (aplicar `habilitar_skill` na ficha → linha com fundo `#FFF7CC`), `highlightsAppliedSummaryField`; em `SkillsPage.test.tsx`, `offersCopilotSuggestionsOnEmptyHub` (clicar num chip abre o painel e envia o texto).
- [ ] **Step 2:** `npx vitest run src/copiloto/contextoPorTela.test.tsx src/skills/SkillsPage.test.tsx` → FAIL.
- [ ] **Step 3:** Implementar; o realce entra pelo `sx` de cada campo/linha.
- [ ] **Step 4:** `npm run typecheck && npm test` → PASS.
- [ ] **Step 5:** `git add web/studio/src` e `git commit -m "feat: give the copilot the context of each studio screen"`.

---

### Task 29: Fluxo no desenho aprovado (front)

**Files:**
- Modify: `web/studio/src/fluxo/canvas/FluxoCanvas.tsx`, `fluxo/canvas/BlocoNode.tsx`, `fluxo/canvas/ArestaComMais.tsx`, `fluxo/painel/BlocoPainel.tsx`, `skills/abas/FluxoAba.tsx`, `skills/abas/FluxoAba.test.tsx`
- Create: `fluxo/painel/ComoOMoradorVe.tsx`, `fluxo/painel/ComoOMoradorVe.test.tsx`

**Board:** `docs/superpowers/design/studio-fase2/Fluxo.dc.html` (fluxo editável, "Como o morador vê" no painel) e `FluxoGestor.dc.html` (mesma tela para o gestor: badge `"Só leitura"` no header, sem os controles de edição do bloco).

**Interfaces:**
- Produces: `ComoOMoradorVe({ bloco }: { bloco: Bloco }): ReactNode` (prévia de conversa: a pergunta ou resposta do bloco como bolha do concierge, as opções como chips; blocos sem fala → `"Este passo não aparece para o morador."`).

Desenho (canvas, Skill › Fluxo): fundo pontilhado (`canvasBg` + pontos `canvasDot`); blocos numerados (`1.`, `2.`…) com 420 px de largura, cartão raio 13, borda 1 px, **barra de 4 px à esquerda na cor do tipo** (cores das Global Constraints do plano original), sem sombra, exceto o bloco em edição (`SOMBRA_DO_BLOCO_EM_EDICAO`); conectores de 2 px `#C1CAD6`; painel à direita de 400 px com os campos do bloco, opções como chips e a seção `"Como o morador vê"` no fim. Sem `skills.editar_fluxo` (gestor, `FluxoGestor.dc.html`): `Chip` `"Só leitura"` ao lado do título "Fluxo", bloco e painel sem campo editável (só `ComoOMoradorVe` e as opções como chips, texto puro), sem `+` de inserir bloco nem drag.

- [ ] **Step 1: Testes que falham**: `numbersBlocksInOrder`, `marksBlockTypeWithLeftBar` (cor da barra = cor do tipo), `onlyEditedBlockHasShadow`, `showsReadOnlyBadgeForGestor` (sem `skills.editar_fluxo`), `hidesEditingControlsForGestor`, `ComoOMoradorVe.test.tsx` (`previewsQuestionAndOptions`, `explainsSilentSteps`).
- [ ] **Step 2:** `npx vitest run src/skills/abas/FluxoAba.test.tsx src/fluxo` → FAIL.
- [ ] **Step 3:** Implementar só o visual e a prévia; o modelo do fluxo não muda.
- [ ] **Step 4:** Mesmo comando → PASS.
- [ ] **Step 5:** `git add web/studio/src` e `git commit -m "feat: restyle flow editor and preview what residents see"`.

---

### Task 30: Avançado no desenho aprovado (front)

**Files:**
- Modify: `web/studio/src/avancado/AvancadoPage.tsx`, `avancado/ArquivosAba.tsx`, `avancado/AvancadoPage.test.tsx`
- Create: `avancado/gruposDeArquivos.ts`, `gruposDeArquivos.test.ts`

**Board:** `docs/superpowers/design/studio-fase2/Avancado.dc.html` (árvore agrupada "Raiz"/"Rotas"/etc., editor com números de linha, chip `"Mudanças aqui afetam todos os condomínios"`).

**Interfaces:**
- Produces: `agruparArquivos(arquivos: Record<string, Arquivo[]>): { grupo: 'Raiz' | 'Conhecimento' | 'Rotas' | 'Modelos e testes'; arquivos: Arquivo[] }[]` (Raiz: `agent.yaml`, `persona.md`, `caso.yaml`, `rules/`, `workflows/`; Conhecimento: `knowledge/`; Rotas: `routers/`; Modelos e testes: `assets/`, `evals/`; grupo vazio some).

Desenho (canvas, Avançado): aba `"Projeção"` passa a `"Prompt montado"` (id `projecao` fica, para não quebrar links); no topo da página, chip de aviso `"Mudanças aqui afetam todos os condomínios"` (texto `#8A5A00` em `#FFF6DF`); Arquivos = cartão da árvore (grupos com sobretítulo, nomes em `FONTE_MONO` 12 px) + cartão do editor com números de linha (coluna fixa em `FONTE_MONO`, `text.muted`) e botões `Excluir` (perigo, texto) e `Salvar` (primário).

- [ ] **Step 1: Testes que falham**: `gruposDeArquivos.test.ts` (`groupsFilesByFolder` em tabela); `AvancadoPage.test.tsx` (`renamesProjectionToAssembledPrompt`, `warnsThatChangesAffectAllCondominios`, `showsLineNumbers`; `opensAndSavesFile` e `deletesFileAfterConfirmation` continuam).
- [ ] **Step 2:** `npx vitest run src/avancado` → FAIL.
- [ ] **Step 3:** Implementar.
- [ ] **Step 4:** Mesmo comando → PASS.
- [ ] **Step 5:** `git add web/studio/src/avancado` e `git commit -m "feat: restyle advanced area with grouped files and line numbers"`.

---

### Task 31: Checklist de capacidades e aposentadoria do Admin e do Backdoor

**Files:**
- Delete: `web/okf/index.html`, `admin.js`, `backdoor.html`, `backdoor.js`, `orquestracao.html`, `orquestracao.js`, `administradoras.html`, `payloads.html`; `cmd/okf-admin/legado.go`
- Modify: `cmd/okf-admin/main.go` (sai `s.rotasLegado(mux)`), `cmd/okf-admin/admin.go` (saem `GET /backdoor/payloads`, `GET /backdoor/administradoras`, `telaAntiga`; comentário do topo sem as páginas antigas), `cmd/okf-admin/studio_web_test.go`, `web/web.go` (comentário), `web/okf/console.html` (links para telas apagadas, se houver), `web/studio/src/condominios/CondominiosPage.tsx` + teste (sai o link das telas antigas e `linksLegacyScreensForSuperlogica`), `README.md` (menções a `/backdoor` e ao Admin)

- [ ] **Step 1: Checklist.** Com `npm run build` em `web/studio` e o servidor de pé, conferir no Studio cada linha (ids da análise, §1). Marcar `[x]` só o que foi visto funcionando:

| id | capacidade | coberta por |
|---|---|---|
| A2, B6 | lista com ids, situação, "sem administradora" | Task 9 |
| A3, A4, A5, A15, B2, B4, B7 | criar/remover condomínio, Padrão, reler, criar/remover administradora, mover condomínio | já no Studio |
| A6, A7 | contagem "N de M", origem da decisão, papéis, voltar ao Padrão | Tasks 6, 17 |
| A8 | matriz skills × condomínios | Task 18 |
| A9, A10 | orientação geral com exemplo; orientação por skill | Tasks 8, 6 |
| A11, A12 | ajustes por skill e gerais; padrão sempre visível | Tasks 6, 7 |
| A13, A14 | identificação com formato e id; salvamento | Task 11 |
| B1, B3 | contagem e renomear administradora | Task 10 |
| B8, B9, B11 | payload de qualquer recurso, definições livres, limpar | Task 22 |
| B10 | sugerir ajustes por frase | Task 7 |
| C3, C7 | nova skill com resumo e critério; tipo e modelo | Tasks 13, 21 |
| C4, C5, C6, C9, C10, C13, C15, C21 | importar, exportar, excluir, fluxo, ferramentas da skill, arquivos, projeção, não salvas | já no Studio |
| C8 | persona com aviso e modelo | Task 14 |
| C11 | problemas da skill | Task 19 + Avançado > Arquivos |
| C14 | paleta | substituída pelo fluxo (desenho) |
| C18, C19 | ferramenta com cabeçalhos, método do servidor de ferramentas, tipos e padrões | Task 12 |
| C20 | ajuda da orquestração | Task 15 |
| A1, B5 | abrir como uma administradora | **P2, sem cobertura**: precisa da Fundação |
| C1 | números da visão geral | **P2, parcial** (problemas por skill na Task 19) |
| C2 | mover skill na árvore | **P2, sem cobertura** (duplicar existe) |
| C12 | markdown renderizado nos arquivos | **P2, sem cobertura** (edição existe) |
| C16 | assinatura e selo na lista de ferramentas | **P2, parcial** |
| C17 | origem do catálogo | **P2, sem cobertura** |

- [ ] **Step 2: Gate.** As linhas marcadas P2 só saem com o aceite explícito do Felipe. Sem esse aceite, pare a task aqui e reporte as linhas pendentes; nada é apagado. Qualquer linha não P2 sem `[x]` também para a task.
- [ ] **Step 3: Teste que falha**: em `studio_web_test.go`, `TestTelasAntigasContinuamNoAr` vira `TestTelasAntigasSairam` (`/admin`, `/backdoor`, `/backdoor/orquestracao`, `/backdoor/administradoras`, `/backdoor/payloads` → 404; `/` → 302 `/studio/`) e `TestRotasDaTelaAntigaRegistradas` vira `TestRotasDaTelaAntigaSairam` (só `PUT /api/agente/config` continua registrada). `go test ./cmd/okf-admin/ -run 'TestTelasAntigas|TestRotasDaTelaAntiga' -v` → FAIL.
- [ ] **Step 4:** Apagar e ajustar os arquivos listados; `grep -rn "backdoor\|admin.js\|/admin\b" cmd web/okf web/studio/src README.md` sem sobra.
- [ ] **Step 5:** `go build ./... && go test ./... ` e `cd web/studio && npm run typecheck && npm test` → PASS.
- [ ] **Step 6:** `git add -A cmd web README.md` e `git commit -m "chore: retire old admin and backdoor pages"`.

---

### Task 32: Boas-vindas, Regras gerais, Base de conhecimento comum e Transbordo no Concierge (front)

O canvas aprovado mostra a página Concierge com seis blocos: Identidade, Persona e tom de voz (Task 14), Boas-vindas, Regras gerais, Base de conhecimento comum, Transbordo para atendente. Os três últimos **já existem no Go** como capacidades genéricas por agente (`rules/`, `knowledge/`), só nunca foram apontadas para o agente raiz `concierge`: `RegrasAba` e `ConhecimentoAba` (`skills/abas/*.tsx`) recebem `caminho` e listam os arquivos daquele agente, inclusive o rótulo "Vem do Concierge" para o que os filhos herdam (`ConhecimentoAba.tsx`, `documentosHerdadosDe`). Aqui elas são reaproveitadas apontando para `caminho: 'concierge'`, sem rota nova.

**Files:**
- Modify: `web/studio/src/concierge/ConciergePage.tsx`, `concierge/ConciergePage.test.tsx` (abas `persona | boas-vindas | regras | conhecimento | transbordo`, URL `?aba=`)
- Create: `web/studio/src/concierge/BoasVindasAba.tsx`, `BoasVindasAba.test.tsx`

**Board:** `docs/superpowers/design/studio-fase2/Concierge.dc.html` (blocos "Boas-vindas", "Regras gerais", "Base de conhecimento comum", "Transbordo para atendente") e `ConciergeCopiloto.dc.html`.

**Interfaces:**
- Consumes: `RegrasAba`, `ConhecimentoAba` (`skills/abas/RegrasAba.tsx`, `ConhecimentoAba.tsx`, já genéricas por `caminho`) apontadas para `caminho: 'concierge'`; `useTextos`/`salvarTextos` (`Visibilidade`, Task 2/4) para "Quem pode conversar"; `DocumentoEditavel` (`skills/abas/DocumentoEditavel.tsx`) para o texto de Transbordo.
- Produces: `ConciergePage` ganha `Tabs` com as 5 abas (`persona | boas-vindas | regras | conhecimento | transbordo`, lidas/gravadas em `?aba=` como `CondominiosPage`); `BoasVindasAba({ assistente }: { assistente: string }): ReactNode`: texto livre (`DocumentoEditavel` apontado para `concierge/rules/boas-vindas.md`, criado vazio se não existir) com ajuda `"Como o concierge se apresenta e com quem ele conversa."`, e rádio "Quem pode conversar" (`Morador identificado` = `private`, `Qualquer contato` = `public`) gravando a `Visibilidade` do agente `concierge` por `salvarTextos` (mesma função da Task 4/21, lendo a versão atual antes de gravar). A aba "Regras gerais" é `<RegrasAba caminho="concierge" />` com o título trocado para `"Regras gerais"` via prop opcional `titulo` (default mantém `"Regras"` para as skills); "Base de conhecimento comum" é `<ConhecimentoAba caminho="concierge" titulo="Base de conhecimento comum" />`. "Transbordo" reaproveita o mesmo padrão de documento único da Boas-vindas, apontado para `concierge/rules/transbordo.md`, ajuda `"Quando o concierge passa a conversa para uma pessoa da equipe."`.

- [ ] **Step 1: Testes que falham**: `ConciergePage.test.tsx`: `showsAllFiveConciergeTabs`, `opensTabFromQuery` (`?aba=regras`); `BoasVindasAba.test.tsx`: `savesWelcomeTextOneSecondAfterTyping`, `savesWhoCanTalkAsVisibility` (rádio `Qualquer contato` → `PUT …/textos` com `visibilidade: 'public'`); `RegrasAba.test.tsx` (ajustar): `titledRegrasGeraisForConcierge` (`<RegrasAba caminho="concierge" titulo="Regras gerais" />` mostra o título novo, resto do componente sem mudança de comportamento); idem para `ConhecimentoAba.test.tsx` com `"Base de conhecimento comum"`.
- [ ] **Step 2:** `npx vitest run src/concierge` → FAIL.
- [ ] **Step 3:** Implementar; `RegrasAba`/`ConhecimentoAba` ganham prop opcional `titulo?: string` (default o texto atual) para não quebrar o uso nas skills.
- [ ] **Step 4:** Mesmo comando → PASS; `npm run typecheck && npm test`.
- [ ] **Step 5:** `git add web/studio/src/concierge web/studio/src/skills/abas` e `git commit -m "feat: add welcome, rules, knowledge and handoff tabs to concierge page"`.

---

## Parallel groups

Tasks de uma mesma onda não dividem arquivo; cada onda começa quando a anterior está commitada.

| Onda | Tasks | Por que espera |
|---|---|---|
| 1 | 0, 1, 2, 3, 4 | nada (a Task 0 não toca `Sidebar.tsx` nem `StudioLayout.test.tsx`, que são da Task 3) |
| 2 | 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 23, 25, 29, 30 | 5 precisa do `resumo` (2); 6-11 precisam dos contratos, do modelo puro e dos testes já separados (4); 12-16, 29 e 30 mexem em arquivos que a Task 3 editou; 25 mexe na casca da Task 0; 23 usa os tipos do fluxo e roda junto |
| 3 | 17, 18, 19, 20, 21, 24, 26, 27, 32 | 17 e 18 usam o drawer (6) e vêm depois de 8 e 9; 19 depois de 13 (`SkillsPage`); 20 depois de 12 e 29 (`FerramentasPage.test.tsx`, `FluxoAba.test.tsx`); 21 depois de 30 (`AvancadoPage.tsx`); 24 usa 23; 26 e 27 usam o painel (25), 26 também `preencherTextosNovos` (13); 32 mexe em `ConciergePage.tsx`, que é da 14 (wave 2) |
| 4 | 22, 28 | 22: `AvancadoPage.tsx` é da 21; 28 mexe nas telas que 13, 17, 18, 19, 29 e 32 acabaram de mudar |
| 5 | 31 | checklist sobre tudo |

Arquivos compartilhados que forçam a ordem: `CondominioPage.test.tsx` é separado na Task 4 justamente para 6, 7, 8 e 11 rodarem juntas; `OrquestracaoAba.test.tsx` nasce na 15 para a 30 ficar com `AvancadoPage.test.tsx`; `ferramentas/contratos.ts` (3 → 12); `skills/SkillsPage.tsx` (3 → 13 → 19 → 28); `avancado/AvancadoPage.tsx` (3 → 30 → 21 → 22); `avancado/contratos.ts` (15 → 19); `condominios/CondominiosPage.tsx` (3 → 9 → 18 → 28); `app/StudioLayout.tsx` (0 → 25); `cmd/okf-admin/main.go` (1 → 24 → 31); `skills/abas/FluxoAba.tsx` (29 → 28); `concierge/ConciergePage.tsx` (3 → 14 → 32 → 28); `skills/abas/RegrasAba.tsx`, `ConhecimentoAba.tsx` (prop `titulo` novo na 32, sem mudar o uso das Tasks 6/17/19 nas skills).

## Decisões deste plano

- Telas antigas: `/admin` serve o Admin antigo (a raiz já vai para `/studio/`), `/backdoor` e `/backdoor/orquestracao` voltam; rotas que só o Backdoor usa ficam em `legado.go`. `PUT /api/agente/config` volta de vez, porque a aba Identidade usa.
- Resumo: `summary:` no `caso.yaml`; sem caso, `resumo:` no `agent.yaml`. Vazio deriva da descrição do caso no Go; sem caso fica vazio.
- Skill sem caso passa a guardar palavras-chave e frases no `agent.yaml`, para "Quando usar" valer igual em toda skill.
- Visibilidade entra em `Textos` para a aba Identidade não editar YAML cru.
- Matriz é uma aba de Condomínios (`?aba=skills`), não rota nova.
- Rota inicial decide por ter administradora, não por `skills.editar_textos`.
- Método do servidor de ferramentas: valor `MCP` só no `.ts`, rótulo "Servidor de ferramentas".
- Ficam de fora, por YAGNI: "Voltar tudo ao Padrão" em lote e "Preencher com IA" dentro do drawer da skill.
- Fontes: a pilha do `styles.css` fica (Paralucent/Argumentum primeiro); como essas não são carregadas, o que aparece é Plus Jakarta Sans 700 e Inter, como no canvas.
- "Quem pode usar" vira pílulas (canvas); tirar o último papel volta para "Todos" à vista, em vez de bloquear.
- Copiloto: o servidor monta o contexto e filtra as propostas (tela, permissão, escopo, catálogo só para a Superlógica, fluxo válido); o front só aplica pelas mesmas funções de gravação das telas, que o servidor valida de novo. Sem streaming na fase 2 (a resposta vem inteira).
- Copiloto: modelo `MODEL_COPILOTO`, senão `MODEL_PLANNER`, senão o do provedor; sem provedor, `503` e o painel mostra "Indisponível".
- Cartões de número do hub: `Skills`, `Registram no sistema`, `Consultas e dúvidas` e `Com problema` (construtor) ou `Ferramentas` (gestor); o canvas não fixa os rótulos.
- A aba "Projeção" muda só o rótulo; o id `projecao` na URL fica.
- Concierge (Task 32): "Regras gerais" e "Base de conhecimento comum" do canvas são `RegrasAba`/`ConhecimentoAba` já existentes apontadas para o agente raiz `concierge` (os dois componentes já leem por `caminho` e já tratam herança, `"Vem do Concierge"`); nenhuma rota Go nova. "Boas-vindas" e "Transbordo" ficam como um documento de texto cada (`concierge/rules/boas-vindas.md`, `concierge/rules/transbordo.md`), no mesmo padrão de "Regras" de uma skill — não uma lista estruturada de gatilhos.
- Concierge (Task 14): "Tom" do canvas (Acolhedor/Direto/Formal/Descontraído) entra como atalhos que escrevem uma frase pronta dentro da persona (igual ao "Usar o modelo"), não como um campo novo no Go; "Identidade" (Nome/Primeiro nome do canvas) é só a prévia `"Oi, {primeiro nome}"` do texto já existente na persona ("Trate o morador pelo primeiro nome…"), sem campo nem persistência nova. **Decisão pendente do Felipe**: se o nível de estrutura do canvas (campos de Identidade e Tom como dados próprios, não texto dentro da persona) for necessário de verdade, isso é uma Fase 3 com schema novo — fica fora desta fase por ora.
