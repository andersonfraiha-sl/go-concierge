# Studio: lacunas, condensação, redundâncias e o "O que o concierge faz aqui"

Data: 2026-11-03 · Branch analisada: `feat/studio-react` (0811a83) · Spec: `docs/superpowers/specs/2026-11-03-studio-design.md`
Referência de UI: `echo-atende-frontend-ng-studio/src/features/bots` (abaixo, `atende/…`)

## 0. Duas constatações antes de tudo

1. **O print não é do Studio, é do Admin antigo.** O texto do print ("O que o concierge faz aqui", "10 de 10", "Ligue ou desligue cada capacidade… use \"detalhes\"", "ligado por padrão", botão "detalhes") só existe em `web/okf/index.html:72-74` e `web/okf/admin.js:236-242`. O Studio não tem essa string em `studio/src`. O equivalente no Studio é `studio/src/condominios/SkillsSecao.tsx`, que tem o **mesmo problema de densidade** (e pior em um ponto: mostra os 4 chips de papel em toda linha, `SkillsSecao.tsx:65-88`). A correção da seção 5 vale para o Studio; o Admin antigo não deve ser consertado, deve ser aposentado.
2. **O Studio ainda não é servido pelo Go nesta branch.** `cmd/okf-admin/studio.go:25-35` registra só a API; não há rota `/studio` nem `go:embed` (o build vai para `web/studio/dist`, `studio/vite.config.ts:9`). Quem abre `http://…/` cai no Admin antigo, o que explica o print. O serviço embutido existe na worktree `go-concierge-wt-st16` (`feat: serve embedded studio and retire backdoor and admin pages`, ainda não mergeada), que **apaga** `backdoor.html/js`, `index.html`, `orquestracao.html/js`. Mergear a st16 antes de fechar os itens P0/P1 abaixo remove capacidades que o Studio ainda não tem.

---

## 1. Inventário de capacidades: Admin + Backdoor → Studio

Legenda: OK = coberto · PARCIAL = coberto com perda · FALTA = sem equivalente.

### 1.1 Admin (`web/okf/index.html` + `admin.js`)

| # | Capacidade antiga | Onde (antigo) | Rota Go | Studio | Status |
|---|---|---|---|---|---|
| A1 | "Entrar como" administradora (seletor que emula login) + `?adm=` vindo do Backdoor | index.html:29, admin.js:68-72, 450-452 | GET /api/admin/estado?administradora= | `acesso/DevRoleSwitch.tsx` troca papel, mas o gestor dev é fixo em `adm-exemplo` (`cmd/okf-admin/studio.go:52`) | PARCIAL: não dá para ver como uma administradora específica |
| A2 | Lista de condomínios com account/company/tenant em colunas | index.html:50, admin.js:134-140 | GET estado | `condominios/CondominiosPage.tsx:78-121` (cartões só com nome + administradora) | PARCIAL: ids e situação somem da lista |
| A3 | Novo condomínio com nome, id, account, company, tenant | index.html:114-127, admin.js:141-167 | PUT /api/admin/condominio | `CondominiosPage.tsx:125-165` (só nome + administradora; ids depois na ficha) | OK (simplificado, aceitável) |
| A4 | Remover condomínio | admin.js:168-172, 320-324 | DELETE /api/admin/condominio | `IdentificacaoSecao.tsx:27-31,48-50` | OK |
| A5 | Ficha "Padrão (todos)" | index.html:48, admin.js:223-233 | — | `CondominiosPage.tsx:86-91`, `CondominioPage.tsx:36-53` | OK |
| A6 | Capacidades por condomínio: switch, contagem "N de M", origem (decisão deste condomínio / vale o padrão / ligado por padrão) | index.html:71-75, admin.js:236-247 | PUT /api/admin/habilitacao | `SkillsSecao.tsx` | PARCIAL: sem contagem, sem origem; só aparece "Usar o padrão" quando a decisão é própria (`SkillsSecao.tsx:59-63`) |
| A7 | Drawer de habilitação: papéis + explicação da origem + "Usar o padrão" | index.html:130-141, admin.js:192-220 | PUT habilitacao (limpar) | papéis como chips sempre visíveis (`SkillsSecao.tsx:65-88`); sem explicação da origem | PARCIAL |
| A8 | **Matriz casos de uso × condomínios** (todas as decisões numa tela, clique na célula) | index.html:39,101-106, admin.js:175-191 | GET estado | nenhuma | **FALTA** |
| A9 | Orientação geral do condomínio, com explicação e exemplo de preenchimento | index.html:76-79 | PUT /api/admin/orientacao | `OrientacoesSecao.tsx:42-49` (sem placeholder, sem explicação) | PARCIAL |
| A10 | Orientação por caso, recolhida (`<details>`), aberta só se preenchida, marcador "tem orientação", exemplo por caso | admin.js:248-252, 283-285 | PUT orientacao | `OrientacoesSecao.tsx:50-60`: 10 textareas abertas, sem exemplo, sem marcador | PARCIAL |
| A11 | Ajustes agrupados **por caso de uso**, marcando "(desligado aqui)", grupo "Gerais" | admin.js:253-278 | PUT /api/admin/ajustes | `AjustesSecao.tsx:49-59` agrupa por ferramenta, sem ligação com a skill | PARCIAL |
| A12 | Campo de ajuste com "padrão: X" sempre visível como placeholder | admin.js:258-267 | — | `CampoAjuste.tsx:22`: `helperText = description ?? dica`, então quando a definição tem descrição o padrão some (ex.: `antecedencia_min_dias`, `okf/nexus/cond-001.yaml:68-73`) | PARCIAL (bug) |
| A13 | Identificação (nome, id, account, company, tenant) + subtítulo "sem ids de integração ainda" | index.html:87-96, admin.js:231, 280 | PUT condominio | `IdentificacaoSecao.tsx` (sem id, sem placeholders, sem aviso de ids faltando) | PARCIAL |
| A14 | Salvar ficha num botão (identificação + orientações + ajustes) | admin.js:286-319 | 3 PUTs | autosave em Orientações/Ajustes, botão em Identificação, imediato em Skills | OK (mas inconsistente, ver §4) |
| A15 | Reler | index.html:31 | — | TanStack refetch | OK |

### 1.2 Backdoor: administradoras e payloads (`administradoras.html`, `payloads.html`, ambos com `admin.js`)

| # | Capacidade | Onde | Rota | Studio | Status |
|---|---|---|---|---|---|
| B1 | Lista de administradoras com id e **nº de condomínios** | administradoras.html:53, admin.js:98-104 | GET estado | `condominios/AdministradorasAba.tsx:46-69` (só nome) | PARCIAL |
| B2 | Criar administradora (nome + id opcional) | administradoras.html:67-77, admin.js:105-126 | PUT /api/admin/administradora | `AdministradorasAba.tsx:20-27,36-41` | OK |
| B3 | **Renomear** administradora | admin.js:105-126 (drawer com nome editável) | PUT administradora | nenhuma | **FALTA** |
| B4 | Remover administradora | admin.js:127-131 | DELETE administradora | `AdministradorasAba.tsx:29-32,59-65` (bloqueia com condomínios) | OK |
| B5 | Botão "Admin" que abre o painel como aquela administradora | admin.js:101 | — | nenhuma | FALTA (P2; ver A1) |
| B6 | Lista global de condomínios com administradora, "sem administradora" em destaque, e ids | administradoras.html:59, admin.js:137 | GET estado | `CondominiosPage.tsx:80-81` ("Sem administradora" só como legenda) | PARCIAL |
| B7 | Mover condomínio de administradora | administradoras.html:87, admin.js:145-148 | PUT condominio | `IdentificacaoSecao.tsx:37-42` | OK |
| B8 | Payload por condomínio **para qualquer recurso**, com resumo em chips (próprio × herdado) | payloads.html:54, admin.js:328-347 | GET estado | `AjustesSecao.tsx:25` filtra só ferramentas com `definicoes`; sem resumo | PARCIAL |
| B9 | "Outras definições" chave/valor livres (recurso sem campos declarados) | payloads.html:74-77, admin.js:350, 375-378, 393-397 | PUT /api/admin/payload | nenhuma (e `PUT /api/admin/payload` não é chamado em `studio/src`) | **FALTA** |
| B10 | Sugerir payload a partir de uma frase | payloads.html:68-72, admin.js:417-428 | POST /api/admin/sugerir-payload | `AjustesSecao.tsx:79-113` | OK (repetido por ferramenta, ver §3) |
| B11 | "Limpar deste condomínio" (apaga o payload do recurso) | payloads.html:78, admin.js:441-443 | PUT payload apagar | "Usar o padrão" por campo só em number/boolean (`CampoAjuste.tsx:38,69`) | PARCIAL |

### 1.3 Backdoor: agentes, recursos, orquestração (`backdoor.html`, `backdoor.js`, `orquestracao.*`)

| # | Capacidade | Onde | Rota | Studio | Status |
|---|---|---|---|---|---|
| C1 | Visão geral com KPIs (agentes, subagentes, casos, erros, avisos, recursos, origem do Nexus) e cartões de casos com erros/avisos | backdoor.html:76-93, backdoor.js:101-128 | GET /api/anatomia, /api/agentes, /api/nexus/recursos | `skills/SkillsPage.tsx` sem números; `SkillCard.tsx` sem erros/avisos; contagem só no seletor do Avançado (`avancado/AvancadoPage.tsx:71-75`) | PARCIAL |
| C2 | Árvore de agentes com **arrastar para mover, Shift para clonar** | backdoor.html:52-63, backdoor.js:153-183 | POST /api/agentes/mover | duplicar existe (`SkillsPage.tsx:43-47`); mover não | PARCIAL (mover: FALTA, P2) |
| C3 | Novo caso: onde, nome, tipo (operador/orquestrador), descrição, modelo, persona/caso.yaml, caixinhas iniciais, recursos declarados | backdoor.html:193-219, backdoor.js:812-860 | POST /api/agentes | `NomeDialog` só com título (`SkillsPage.tsx:93-101`) | PARCIAL (simplificação proposital; falta pelo menos "Quando usar" e resumo na criação) |
| C4 | Importar zip | backdoor.html:222-234, backdoor.js:862-872 | POST /api/agentes/importar | `avancado/ImportarExportarAba.tsx:122-183` ("Instalar em" virou texto livre, antes era select "Onde" + nome) | PARCIAL |
| C5 | Exportar zip | backdoor.js:471 | GET /api/agentes/exportar | `ImportarExportarAba.tsx:107-120` | OK |
| C6 | Excluir agente (com aviso das mudanças de rota) | backdoor.js:458-470 | DELETE /api/agente | `SkillsPage.tsx:49-57` | OK |
| C7 | Identidade: nome, **tipo**, descrição (critério do roteador), **modelo** | backdoor.html:119-131, backdoor.js:428-440 | PUT /api/agente/config | nome e "Quando usar" (`SkillEditorPage.tsx:79-84`, `QuandoUsarAba.tsx:17-27`); tipo e modelo só editando `agent.yaml` cru no Avançado | PARCIAL |
| C8 | Persona + aviso "quem fala com o morador precisa de persona" + "use o modelo" | backdoor.html:134-138, backdoor.js:360-367 | GET /api/arquivo/modelo | `assistente/AssistentePage.tsx:35-63` (sem aviso, sem modelo) | PARCIAL |
| C9 | Workflow: passos de `principal.md`, passos do `caso.yaml`, **procedimentos** (`abrir_workflow`) | backdoor.html:140-143, backdoor.js:381-426 | GET /api/agente | `skills/abas/FluxoAba.tsx` + `fluxo/*` para o caso.yaml; procedimentos só no Avançado > Arquivos | OK para caso.yaml, PARCIAL para procedimentos |
| C10 | Recursos que o agente usa (declarados, origem, ok/quebrado), declarar e tirar | backdoor.html:145-148, backdoor.js:185-195, 371-375 | POST /api/agente/recursos | `skills/abas/FerramentasAba.tsx` | OK |
| C11 | Problemas do agente no topo da ficha (erro/aviso + solução) | backdoor.html:114, backdoor.js:349-350 | GET /api/agente | só no Avançado > Arquivos (`ArquivosAba.tsx:228-247`) e no Fluxo | PARCIAL |
| C12 | Arquivos: árvore própria + herdada, visor com markdown renderizado (frontmatter, links relativos, glossário em tabela) | backdoor.html:164-168, backdoor.js:484-560, 689-743 | GET /api/arquivo | `ArquivosAba.tsx` (texto cru) | PARCIAL (render: P2) |
| C13 | Editar/criar (com modelo por caixinha)/excluir arquivo | backdoor.js:542-560, 676-687, 745-781 | PUT/DELETE /api/arquivo, GET /api/arquivo/modelo | `ArquivosAba.tsx:147-226,249-283`; Regras e Base de conhecimento em `skills/abas/*` | OK |
| C14 | Paleta (botão direito) para inserir recurso na sintaxe do arquivo | backdoor.html:186-190, backdoor.js:575-686 | — | substituída pelo editor de fluxo | OK (por desenho) |
| C15 | Projeção e Roteador | backdoor.html:169-176, backdoor.js:783-789 | GET /api/projecao | `avancado/ProjecaoAba.tsx` | OK |
| C16 | Catálogo de recursos com filtro, grupos por classe, **assinatura `(params*) → output`**, selo MCP, kind/visibility/write | backdoor.html:66-73, backdoor.js:196-226 | GET /api/nexus/recursos | `ferramentas/FerramentasPage.tsx` (grupos e filtro; cartão só com título e descrição, `:188-205`) | PARCIAL |
| C17 | Origem do catálogo (ao vivo / instantâneo / indisponível) e account | backdoor.html:29-30, backdoor.js:61-64, 199 | GET /api/nexus/recursos | `FerramentasSchema` traz `origem/conferido` (`catalogo/contratos.ts:80-85`) mas nenhuma tela mostra | FALTA (P2) |
| C18 | Editar recurso: título, código, kind, visibility, write, descrição, **método incl. MCP**, URL, **headers**, corpo, params, output, saída **com tipo**, amostra | backdoor.html:237-276, backdoor.js:227-288 | PUT/DELETE /api/nexus/recurso | `ferramentas/FerramentaDrawer.tsx`: sem headers (só preservados, `ferramentas/contratos.ts:88`), sem "MCP" em `METODOS` (`FerramentaDrawer.tsx:42`), saída sem tipo (`:189-200`) | PARCIAL |
| C19 | Definições (campos de ajuste) do recurso | só lidas no Backdoor (`backdoor.js:265` preserva) | PUT recurso | `FerramentaDrawer.tsx:230-247` cria/edita, mas **sem tipo e sem padrão** (`DefinicaoSchema` tem `type` e `default`, `catalogo/contratos.ts:46-53`) | PARCIAL (novo, mas incompleto) |
| C20 | Orquestração: modo com descrição e "Quando:", aviso de ambiente, 3 ajustes **com texto de ajuda** | orquestracao.html:66,72-76, orquestracao.js:26-45 | GET/PUT /api/admin/orquestracao | `avancado/OrquestracaoAba.tsx` (texto de cada limite existe em `avancado/contratos.ts:119-121` mas não é exibido, `OrquestracaoAba.tsx:132-142`) | PARCIAL |
| C21 | Aviso de alterações não salvas + Ctrl+S | backdoor.js:293-310, 455 | — | autosave com indicador (`shared/autosave/*`) | OK (melhor) |

Resumo: **2 FALTA de alto impacto** (A8 matriz, B9 payload livre), **1 FALTA simples** (B3 renomear), e ~20 PARCIAIS. Não há rota Go nova necessária para nada do P0/P1: tudo já existe em `cmd/okf-admin/admin.go:67-78` e `main.go:245-262`, exceto o campo `resumo` (§5).

---

## 2. "Condensou demais": o que a pessoa precisa ver ou decidir e sumiu

1. **"Quando usar" engoliu dois textos diferentes.** Antes havia o texto para a administradora (`caso.yaml description`, "a descrição do caso fala com a administradora", `cmd/okf-admin/admin.go:129-131`) e o critério do roteador (`agent.yaml descricao`, "escreva como classificador", `backdoor.html:129`). Hoje `Textos.QuandoUsar` grava os dois e ainda reescreve a rota (`internal/okf/skills.go:40,277,292,307`). Resultado: a administradora lê texto de classificador ("Os campos… vêm do Nexus", "Usado quando o morador…"), e quem melhora o texto para a administradora mexe no roteamento sem querer. **Trazer de volta:** um campo `resumo` (uma linha, para pessoas) separado de "Quando usar" (para o modelo). Ver §5.3.
2. **Orientações por skill**: perderam o recolhimento, o marcador "tem orientação" e os exemplos por tema (`admin.js:252,283-285`; `index.html:78-79`). E o texto do Padrão nunca aparece na ficha do condomínio, embora some em tempo de execução. **Trazer:** placeholder com exemplo, orientação da skill dentro do drawer da skill (§5), e um bloco só leitura "Também vale o do Padrão: …".
3. **Ajustes**: perderam o vínculo com a skill ("(desligado aqui)"), o grupo "Gerais", os campos livres, o "Limpar deste condomínio" e, por bug, o valor herdado quando há descrição (§1 A11, A12, B8, B9, B11). **Trazer:** ajustes da skill no drawer da skill; "Gerais" só para o que não pertence a nenhuma skill; placeholder = valor herdado sempre; "Usar o padrão" em todo tipo; campos livres só no Avançado.
4. **Restrição por papel**: a explicação de onde vem a decisão (`admin.js:202`, "Sem entrada própria: vale o padrão. Salvar cria a entrada do condomínio.") sumiu. Clicar num chip de papel cria decisão própria em silêncio. **Trazer:** a frase de origem no drawer, e um chip "deste condomínio" na linha quando a decisão é própria.
5. **Identificação**: sem id, sem exemplos de formato (`index.html:92-94` tinha `cond-001`, `5982`, `69a5…`), sem o aviso "sem ids de integração ainda" (`admin.js:231`). **Trazer:** placeholders, validação de formato e o aviso como chip na lista e na ficha.
6. **Administradoras**: renomear, contagem de condomínios e "abrir como esta administradora" (§1 B1, B3, B5).
7. **Matriz skills × condomínios** (A8): para uma administradora com 40 condomínios, desligar uma skill em vários hoje exige abrir 40 fichas.
8. **Ferramentas**: headers, método MCP, tipo dos campos de saída, tipo e padrão das definições, assinatura na lista, origem do catálogo (§1 C16-C19).
9. **Identidade da skill**: modelo, tipo, ícone (`icon:` existe em todo `caso.yaml`, ex. `okf/agentes/concierge/agentes/agendar-mudanca/caso.yaml:5`, e nenhuma tela usa) e visibilidade do caso. Hoje só editando YAML cru no Avançado.
10. **Persona**: sumiu o aviso de persona vazia e o "use o modelo" (C8).
11. **Problemas da skill** (C11): o construtor só descobre um vínculo quebrado entrando no Avançado.
12. **Orquestração**: a ajuda por campo (`orquestracao.html:73-75`) virou só validação ao salvar.

---

## 3. Redundâncias

1. **A mesma descrição em quatro lugares**: `SkillCard.tsx:31-33`, `QuandoUsarAba.tsx:18-26`, `SkillsSecao.tsx:55-57` e, para a ferramenta, `FerramentasAba.tsx:104-108` + `FerramentasPage.tsx:194-196`. Na ficha do condomínio, o texto inteiro de classificador aparece em toda linha.
2. **A lista de skills aparece três vezes na ficha do condomínio**: `SkillsSecao` (10 linhas), `OrientacoesSecao` (10 textareas, `:50-60`) e `AjustesSecao` (por ferramenta, mas cada ferramenta é de uma skill: "Reserva de área comum" a ferramenta, `okf/nexus/cond-001.yaml:46-47`, e "Reserva de Área Comum" a skill). A mesma coisa com dois nomes e três lugares.
3. **"Ligado por padrão" + switch** (print, `admin.js:241`): o switch já diz que está ligado; o texto só deveria aparecer quando informa algo novo (decisão própria).
4. **"Sem papel marcado: vale para todos"** repetido em toda linha (`SkillsSecao.tsx:83-87`) + 4 chips por linha. O normal (todos os papéis) não precisa de nada na tela; só a exceção merece chip.
5. **"Descreva em uma frase / Sugerir"** repetido para cada ferramenta (`AjustesSecao.tsx:100-113`). Uma caixa por seção, ou atrás de um botão "Preencher com IA" (padrão `atende/components/panel/FillWithAiBox.tsx`).
6. **"Usar o padrão"** em três níveis (por skill, por campo, e ausente por ferramenta) sem regra clara.
7. **Dois jeitos de editar o mesmo arquivo**: Persona (`AssistentePage.tsx`) e `persona.md` no Avançado; Regras/Base de conhecimento (`skills/abas/*`) e `rules/`, `knowledge/` no Avançado; Fluxo visual e `caso.yaml` cru no Avançado (pode sobrescrever o fluxo com YAML inválido). Aceitável no Avançado, mas o Avançado deveria dizer "editado também em Skills > X" e, para `caso.yaml`, abrir só leitura com link para o Fluxo.
8. **Três jeitos de salvar** na mesma ficha: imediato (skills), autosave (orientações, ajustes), botão (identificação, `IdentificacaoSecao.tsx:51-57`).
9. **Seletor de administradora duas vezes** (`CondominiosPage.tsx:153-155` e `IdentificacaoSecao.tsx:37-42`): ok, mas o diálogo de criação podia ser a única entrada e a ficha mostrar só leitura com "Mover".

---

## 4. Facilidade de preencher, por tela

| Tela | Atrito (arquivo:linha) | Proposta | Referência no Atende |
|---|---|---|---|
| Condomínio > Skills | Cartão alto por skill, texto de classificador, chips de papel sempre abertos (`SkillsSecao.tsx:45-88`) | Lista densa + drawer (§5) | `atende/components/agents/ToolRow.tsx`, `ToolPicker.tsx:30-48` (grupos), `atende/components/editor/BlockDrawer.tsx` (drawer 380px) |
| Condomínio > Orientações | Textarea em branco sem exemplo nem explicação (`OrientacoesSecao.tsx:42-49`); 10 textareas abertas | Placeholder com o exemplo de `index.html:79`; ajuda "como você explicaria a um atendente novo"; orientação por skill vai para o drawer; mostrar o texto do Padrão só leitura | `atende/components/panel/widgets/FieldShell.tsx` (rótulo + ajuda) |
| Condomínio > Ajustes | Padrão escondido quando há descrição (`CampoAjuste.tsx:22`); sem unidade; "Sugerir" repetido; título técnico como fallback (`AjustesSecao.tsx:99`) | `placeholder` = valor herdado + `helperText` = descrição; sufixo "dias" via `InputAdornment` quando o rótulo tem unidade; "Usar o padrão" em todo tipo; um "Preencher com IA" recolhido | `atende/components/tenants/SlotsForm.tsx:25-26` (placeholder = padrão), `atende/components/panel/FillWithAiBox.tsx` |
| Condomínio > Identificação | Rótulos sem formato, nenhum marcado obrigatório, botão Salvar isolado (`IdentificacaoSecao.tsx:19-25,51-57`) | Placeholders (`cond-001`, `5982`, 24 hex), validação de formato, Nome com `required`, autosave como o resto | `atende/components/bots/CreateSkillDialog.tsx:55-58` (helper sob o campo) |
| Condomínios (lista) | Cartões com nome + administradora, sem busca, sem situação (`CondominiosPage.tsx:78-121`) | Tabela/lista com busca, coluna "skills ligadas N de M", chip "faltam ids", chip da administradora | `atende/pages/SkillListPage.tsx:52-103` (filtro + chips), `atende/components/bots/SummaryCards.tsx` |
| Administradoras | Sem renomear, sem contagem (`AdministradorasAba.tsx:46-69`) | Nome editável no lugar (`shared/ui/InlineTitle.tsx` já existe), contagem, link "ver condomínios" filtrando a lista | `atende/pages/TenantListPage.tsx` |
| Skill > Quando usar | O campo é classificador mas a ajuda diz "em uma frase" (`QuandoUsarAba.tsx:17`); palavras-chave e frases só aparecem com fluxo (`:28`), então FAQ e Regimento não têm | Dois campos: "Resumo" (1 linha, contador até 90, aparece para administradoras) e "Como reconhecer o pedido" (classificador, com exemplo); frases como chips com Enter | `atende/components/panel/widgets/StringListWidget.tsx:31-40` ("Enter adiciona") |
| Nova skill | Só título (`SkillsPage.tsx:93-101`) | Título + resumo + "Quando usar" no diálogo, com ajuda | `atende/components/bots/CreateSkillDialog.tsx` |
| Skills (lista) | Cartão sem problemas nem ícone; "N ferramentas" não diz nada a gestor (`SkillCard.tsx:34-37`) | Ícone do `caso.yaml`, resumo, chip de erro/aviso só para construtor, filtro | `atende/pages/SkillListPage.tsx` |
| Ferramenta (drawer) | ~15 campos em coluna, "Nome" sem dizer que é código, definição sem tipo/padrão (`FerramentaDrawer.tsx:121-247`) | Seções recolhíveis: Básico (título, descrição, tipo, grava?), Parâmetros, Saída, Conexão (recolhida), Amostra, Ajustes por condomínio; "Código" com padrão `grupo.acao` validado; definição com Tipo (Número/Texto/Sim-não/Escolha) e Padrão, opções só em Escolha (o mesmo que `fluxo/painel/InformacoesPainel.tsx:26-28` já faz) | `atende/components/panel/SchemaForm.tsx`, `ObjectFields.tsx` |
| Fluxo > Informações | Id técnico em monospace para todos (`InformacoesPainel.tsx:73-75`), Pergunta sem exemplo | Id só para construtor; placeholder "Ex.: Para que dia é a reserva?" | `atende/components/panel/widgets/TextWidget.tsx` |
| Assistente > Persona | Textarea vazia sem modelo (`AssistentePage.tsx:53-60`) | Aviso quando vazia + "Usar o modelo" (`GET /api/arquivo/modelo?caminho=persona.md`, já em `catalogo/contratos.ts:146-149`) | `atende/components/agents/AgentPersonaSection.tsx` |
| Avançado > Orquestração | Ajuda por campo não exibida (`OrquestracaoAba.tsx:133-141`) | `helperText` com o `texto` de `LIMITES_DE_ORQUESTRACAO` + a frase de `orquestracao.html:73-75` | — |
| Avançado > Importar | "Instalar em" é caminho digitado (`ImportarExportarAba.tsx:145-151`) | Select da árvore + nome, como `backdoor.html:227-228` | — |
| Geral | Gestor entra em `/skills` (`app/router.tsx:18`), mas o trabalho dele é Condomínios | Rota inicial por permissão: `condominios.habilitar` sem `skills.editar_textos` → `/condominios` | — |

---

## 5. Correção de "O que o concierge faz aqui"

### 5.1 Hoje

Print (Admin antigo): ~150-180 px por skill, 5 skills por tela. Studio (`SkillsSecao.tsx`): switch + título + descrição completa + linha de 4 chips + legenda, ~110-130 px por skill. Com 10 skills a seção ocupa mais de uma tela, e Orientações e Ajustes repetem a lista logo abaixo.

### 5.2 Proposta (uma linha por skill, ~44 px)

```
O que o concierge faz aqui   8 de 10 ligadas        [Buscar…]  [ ] Só o que muda do Padrão
────────────────────────────────────────────────────────────────────────────────────────────
REGISTRA PEDIDOS
 [caminhão] Agendamento de mudança   Agenda dia e período da mudança.      só síndico   (o)
 [martelo]  Comunicação de obra      Avisa a administração sobre uma obra. deste cond.  ( )
 …
CONSULTAS
 [pacote]   Minhas encomendas        Mostra as encomendas na portaria.                  (o)
DÚVIDAS
 [livro]    Regimento interno        Responde o que o regimento permite.                (o)
```

- **Cabeçalho**: título, "N de M ligadas", busca (filtra por título e resumo), toggle "Só o que muda do Padrão" (mostra só `propria === true`, `condominios/contratos.ts:150-159`). Ação em lote opcional no menu "⋯": "Voltar tudo ao Padrão".
- **Linha** (`ListItemButton` denso, uma linha, `noWrap` + `Tooltip` com o resumo inteiro): ícone 20 px, título, resumo de uma linha, **chips só para exceções**: "só síndico, proprietário" quando há papéis; "deste condomínio" quando a decisão é própria. Nada de "ligado por padrão" (o switch já diz) nem "vale para todos" (é o normal). Switch à direita, com `onClick` parando a propagação. Desligada: `opacity .6`.
- **Agrupamento** por categoria derivada, sem campo novo: `escrita: true` → "Registra pedidos"; tem caso e não escreve → "Consultas"; sem caso (faq, regimento) → "Dúvidas". Precisa expor `tem_caso`/`id` em `casoDeUso` (o `ID` já existe, `cmd/okf-admin/admin.go:84`, mas `CasoSchema` não lê, `condominios/contratos.ts:53-59`).
- **Drawer lateral** (clique na linha, 380-420 px, padrão `atende/components/editor/BlockDrawer.tsx`):
  1. Título, ícone, resumo, switch e a frase de origem (`admin.js:202`).
  2. "Quem pode usar": radio "Todos" / "Só alguns papéis" + checkboxes (morador, proprietário, síndico, público).
  3. "Orientação para esta skill" (sai de `OrientacoesSecao`), com o placeholder de `admin.js:283-285`.
  4. "Ajustes desta skill": os campos das ferramentas que a skill usa (`caso.recursos`, a mesma lógica de `admin.js:270-275`), com `CampoAjuste`.
  5. Rodapé: "Voltar ao Padrão" (limpar) e, para quem pode, "Ver fluxo" (`/skills/:nome?aba=fluxo`).
  6. O texto de classificador ("Quando usar") só aparece aqui, recolhido, para quem tem `skills.editar_textos`.
- Com isso a ficha fica: **Skills** (lista densa) → **Orientações gerais** (um textarea, com o texto do Padrão acima) → **Ajustes gerais** (só ferramentas que não pertencem a nenhuma skill) → **Identificação**. As listas repetidas somem (§3.2).

### 5.3 De onde vem o resumo de uma linha

A skill hoje só tem o texto de classificador. Proposta, em ordem:

1. **Campo novo `resumo` no `caso.yaml`** (uma linha, até ~90 caracteres, para pessoas). Para skills sem caso (faq, regimento), `resumo` no `agent.yaml`. Go: `casoDeUso.Resumo` em `cmd/okf-admin/admin.go:82-90` e `Textos.Resumo` em `internal/okf/skills.go:38-44`, gravado por `gravarTextosNoCaso` (`:270-286`) **sem** entrar em `atualizarRoteamento` (`:290-301`). Studio: `CasoSchema` e `TextosSchema` ganham `resumo`; `QuandoUsarAba` ganha o campo "Resumo".
2. **Enquanto o campo estiver vazio, derivar** no Go (não no front, para valer em toda tela): primeira frase de `caso.description`, sem prefixos de classificador ("Usado para o morador", "Usado quando o morador", "Usado para o dono ou morador da unidade"), cortada em 90 caracteres. Nunca usar `agent.yaml descricao` como fallback de resumo.
3. **Preencher os 10 resumos agora** no `okf/agentes`, à mão (são 10 linhas). Sugestões:

| Skill | resumo |
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

4. **Ícone**: ler `icon` do `caso.yaml` (lucide: `truck`, `file-pen-line`, `key-round`, `hammer`, `package`, `calendar-check`, `building-2`) e mapear para `@mui/icons-material` num `Record` com fallback.

---

## 6. Vocabulário que vaza para a administradora

| Termo / texto | Origem | Onde aparece | Correção |
|---|---|---|---|
| "vêm do **Nexus**" | `okf/agentes/concierge/agentes/atualizacao-cadastral/caso.yaml:6-8` | ficha do condomínio (descrição da skill), cartão da skill | reescrever a description; o resumo resolve na tela |
| URL crua `https://gruvi.link/guest` e "Usado para o dono ou morador…" | `liberacao-visitante/caso.yaml:6-9` | idem | fica no "Quando usar" (o modelo precisa); some da tela com o resumo |
| "caso de uso determinístico com confirmação e protocolo", "especialista no regimento interno…" | `agent.yaml descricao` de `liberacao-visitante`, `regimento`, `faq` | qualquer skill sem `caso.description` cai aqui (`cmd/okf-admin/admin.go:116`) | resumo obrigatório em toda skill roteada |
| "O **concierge** informa ao confirmar" vs Studio "a **assistente**" | `okf/nexus/cond-001.yaml:85` (descrição da definição) × `CondominiosPage.tsx:45`, `SkillsSecao.tsx:37-39`, `OrientacoesSecao.tsx:38` | Ajustes | decidir um termo; o PO usa "concierge" ("O que o concierge faz aqui"), a spec usa "Assistente" (§4). Recomendo "concierge" na tela de condomínio |
| Código da ferramenta (`gruvi.bff.building-operation…`) quando falta `titulo` | `AjustesSecao.tsx:99`, `FerramentasAba.tsx:99` | Ajustes, aba Ferramentas | fallback "Ferramenta sem nome" + aviso para o construtor; exigir título no drawer |
| Descrições de ferramenta escritas para o modelo ("Ação de escrita: só pelo caso de uso…", "Use antes de montar…") | `okf/nexus/cond-001.yaml:110-111,128-129` | `FerramentasPage.tsx:194-196` e `FerramentasAba.tsx:104-108`, visíveis ao gestor (`skills.ler`) | gestor vê só título; descrição completa só para `ferramentas.editar` |
| Id técnico das informações (`unidade_id`) | `InformacoesPainel.tsx:73-75` | painel do fluxo, também para gestor | esconder para quem não edita fluxo |
| "Conta de integrações", "Canal do Atende" | `IdentificacaoSecao.tsx:43-45` | só com `condominios.editar` | ok; acrescentar exemplo de formato |
| Old Admin: "account no Nexus", "recursos do Nexus não declaram definições" | `web/okf/index.html:92`, `admin.js:278` | Admin antigo (o print) | some quando o Admin for aposentado |

O teste de vocabulário que vem na st16 (`studio/src/vocabulario.test.ts`) olha só o código do front; vazamentos que vêm **dos dados** (YAML) passam. Proposta: um teste Go em `cmd/okf-admin` que monta `GET /api/admin/estado?administradora=…` com o `okf/` real e falha se `titulo`, `resumo` ou `definicoes[].label/description` contiverem `Nexus|orquestrador|subagente|caso.yaml|MCP`.

---

## 7. Tarefas priorizadas

Cada item cabe em uma pessoa e uma PR pequena. Skills do brief: `code-quality` + `react` (front) ou `go` (Go); `frontend-design` nos itens de UI.

### P0 (o que o PO reclamou e o que impede aposentar o antigo)

1. **Resumo separado do "Quando usar"** (Go + dados): `resumo` em `caso.yaml`/`agent.yaml`, `casoDeUso.Resumo` com fallback derivado (§5.3), `Textos.Resumo` gravado sem mexer na rota; preencher os 10 resumos. Arquivos: `internal/okf/skills.go`, `cmd/okf-admin/admin.go:82-140`, `okf/agentes/concierge/agentes/*/caso.yaml|agent.yaml`, testes em `internal/okf/skills_test.go`, `cmd/okf-admin/studio_test.go`.
2. **Contratos do front**: `resumo`, `id`, `icon` em `CasoSchema` e `SkillResumoSchema`, `resumo` em `TextosSchema`; mapa de ícones lucide → MUI. Arquivos: `studio/src/condominios/contratos.ts:53-59`, `studio/src/skills/contratos.ts:12-35`, novo `studio/src/skills/icones.ts`.
3. **SkillsSecao como lista densa** (§5.2): linha de 44 px, contagem, busca, "Só o que muda do Padrão", grupos por categoria, chips só de exceção. Arquivo: `studio/src/condominios/SkillsSecao.tsx` (+ teste em `CondominioPage.test.tsx`).
4. **Drawer da skill no condomínio**: origem, papéis, orientação da skill, ajustes da skill, "Voltar ao Padrão", "Ver fluxo". Arquivos: novo `studio/src/condominios/SkillDoCondominioDrawer.tsx`; tirar a lista por skill de `OrientacoesSecao.tsx:50-60`; `AjustesSecao.tsx` passa a mostrar só ferramentas fora de qualquer skill.
5. **CampoAjuste**: placeholder = herdado sempre, helper = descrição, "Usar o padrão" em todo tipo, sufixo de unidade; "Sugerir" recolhido em um botão por seção. Arquivos: `studio/src/condominios/CampoAjuste.tsx:19-82`, `AjustesSecao.tsx:73-131`.
6. **Orientações gerais legíveis**: ajuda, placeholder de `index.html:79`, bloco só leitura com o texto do Padrão. Arquivo: `studio/src/condominios/OrientacoesSecao.tsx`.
7. **Vazamentos de dados**: reescrever `atualizacao-cadastral/caso.yaml:6-8`; decidir "concierge" × "assistente" e aplicar nas telas de condomínio; teste Go de vocabulário sobre `/api/admin/estado` (§6). Arquivos: `okf/agentes/...`, `studio/src/condominios/*.tsx`, `cmd/okf-admin/admin_test.go` (novo).
8. **Sequência da st16**: mergear o serviço de `/studio` (`cmd/okf-admin/studio_web.go` da st16) mas **manter** `index.html`, `backdoor.*`, `orquestracao.*`, `payloads.html` até P0+P1 fecharem; redirecionar `/` para `/studio`. Arquivos: `cmd/okf-admin/main.go`, `admin.go:27-66`, `web/web.go`.

### P1 (o "faltando bastante coisa")

9. **Matriz skills × condomínios** (A8): aba "Visão por skill" em Condomínios, linhas = skills, colunas = Padrão + condomínios, célula com switch e ponto de "decisão própria"; clique abre o mesmo drawer do item 4. Arquivos: novo `studio/src/condominios/MatrizPage.tsx`, `app/router.tsx`, `CondominiosPage.tsx` (aba).
10. **Lista de condomínios útil**: busca, "N de M skills", chip "faltam ids", administradora; tabela em vez de cartões. Arquivo: `studio/src/condominios/CondominiosPage.tsx:78-121`.
11. **Administradoras**: renomear no lugar (`InlineTitle`), contagem de condomínios, link para a lista filtrada. Arquivo: `studio/src/condominios/AdministradorasAba.tsx`.
12. **Identificação**: placeholders, validação de formato, `required` no nome, autosave, mostrar o id. Arquivo: `studio/src/condominios/IdentificacaoSecao.tsx`.
13. **Ferramenta drawer completo**: headers (lista chave/valor com `ListaEditavel`), "MCP" em `METODOS`, tipo na saída, tipo + padrão nas definições (opções só em Escolha), seções recolhíveis, "Código" validado. Arquivos: `studio/src/ferramentas/FerramentaDrawer.tsx:41-247`, `ferramentas/contratos.ts`.
14. **Quando usar em dois campos** + palavras/frases também para skills sem fluxo + diálogo de nova skill com resumo e "Quando usar". Arquivos: `studio/src/skills/QuandoUsarAba.tsx`, `SkillsPage.tsx:93-101`, `shared/ui/NomeDialog.tsx`.
15. **Payload livre e "Limpar" no Avançado** (B9, B11): aba "Ajustes por condomínio (técnico)" com select de condomínio, tabela de recursos com chips próprio/herdado e drawer chave/valor, usando `PUT /api/admin/payload`. Arquivos: novo `studio/src/avancado/AjustesTecnicosAba.tsx`, `avancado/AvancadoPage.tsx:23-28`, `condominios/contratos.ts` (`salvarPayload`).
16. **Rota inicial por papel**: gestor cai em `/condominios`. Arquivo: `studio/src/app/router.tsx:18`.
17. **Skills (lista)**: ícone, resumo, chip de erros/avisos para construtor, filtro. Arquivos: `studio/src/skills/SkillCard.tsx`, `SkillsPage.tsx` (contagem vem de `GET /api/agentes`, `avancado/contratos.ts:36-40`).
18. **Persona**: aviso de vazia + "Usar o modelo". Arquivo: `studio/src/assistente/AssistentePage.tsx`.
19. **Orquestração**: `helperText` em cada limite e a linha "Quando usar" por modo. Arquivo: `studio/src/avancado/OrquestracaoAba.tsx:18-28,132-142`.
20. **Identidade técnica da skill no Avançado** (modelo, tipo, visibilidade) como formulário em vez de YAML. Arquivos: novo `studio/src/avancado/IdentidadeAba.tsx`, `PUT /api/agente/config`.
21. **Gestor não vê descrição técnica de ferramenta nem ids de informação**. Arquivos: `FerramentasPage.tsx:188-205`, `skills/abas/FerramentasAba.tsx:98-127`, `fluxo/painel/InformacoesPainel.tsx:73-75`.

### P2

22. Mover skill (POST /api/agentes/mover) no menu do cartão. `studio/src/skills/SkillCard.tsx`, `skills/contratos.ts`.
23. Markdown renderizado (somente leitura) em Avançado > Arquivos e nas Regras. `avancado/ArquivosAba.tsx`, `skills/abas/DocumentoEditavel.tsx`.
24. Origem do catálogo (ao vivo / instantâneo / indisponível) e "usada por N skills" na página de Ferramentas. `ferramentas/FerramentasPage.tsx`.
25. "Ver como esta administradora" para a Superlógica (precisa da Fundação; hoje `euDev` é fixo, `cmd/okf-admin/studio.go:47-70`).
26. Importar com select "Onde" em vez de caminho digitado. `avancado/ImportarExportarAba.tsx:145-151`.
27. Avançado avisa "editado também em Skills > X" para `persona.md`, `rules/`, `knowledge/`, e abre `caso.yaml` só leitura com link para o Fluxo. `avancado/ArquivosAba.tsx`.
28. Base de conhecimento: enviar arquivo e colar texto. `skills/abas/ConhecimentoAba.tsx` (ref.: `atende/components/knowledge/UploadDocumentButton.tsx`, `PasteTextDialog.tsx`).
29. Números no topo de Skills (ligadas, com problema, ferramentas). Ref.: `atende/components/bots/SummaryCards.tsx`.
