# Smoke do Studio na Fundação, por papel

- Data: 2026-11-04
- Commit testado: 265efd7 (branch `feat/studio-react`; contém as Tasks 1 a 9, inclusive o merge da Task 9 em 0c69a66)
- Ambiente: `AUTH_MODE=dev`, `AMBIENTE=dev`, Postgres do compose (porta 5433), `okf-admin` em `127.0.0.1:8090`, navegador real (Playwright). Usuário trocado pelo "Ver como".
- Prints (fora do repositório): `C:\Users\felip\AppData\Local\Temp\claude\47fc841402b12a07\e954a488-9fae-483d-876f-ce224c8f178f\scratchpad\smoke\`

## Subida

| Passo | Resultado |
|---|---|
| `./build.sh --test` | ok. Studio: lint, typecheck e 814 testes verdes (59 arquivos). Go: gofmt, vet e `go test -p 1 ./...` verdes |
| `./build.sh --run` | ok. Front compilado, Postgres do compose saudável, `okf-admin` ouvindo em `127.0.0.1:8090`; porta estava livre antes |
| `importar-json -dir okf/var` | ok, com ressalva: `mapeamento lidos=2 gravados=2`, `memorias 0/0`, `pedidos 0/0`. O `okf/var` deste checkout só tem `admin/mapeamento.json`, então não há dado real de memórias/pedidos para comparar. O `bin/okf-admin.exe` antigo não conhecia o subcomando (subiu um servidor); a importação rodou com o binário gerado pelo `--run`, depois de o servidor subir |
| `/` | ok. Redireciona 302 para `/studio/` |

## Por papel

### admin-superlogica

| Item | Resultado | O que se viu |
|---|---|---|
| `/` vai para `/studio/` | ok | Cai em `/studio/concierge`, menu com Concierge, Skills, Ferramentas, Condomínios, Avançado, Atendente |
| Cria condomínio | ok | `POST /api/admin/condominio` 201, vai para o detalhe do condomínio novo e a frase "Condomínio criado." aparece (print `admin-03-criado.png`) |
| Criar de novo com o mesmo id | ok | O diálogo fica aberto com "Já existe um condomínio com esse nome." no campo Nome (print `admin-02-criar-409.png`) |
| Habilita uma skill | ok | No detalhe do condomínio, desligar "Agendamento de Mudança" (`PUT /api/admin/habilitacao` 200, contador vai a "9 de 10 ligadas") e religar (200) |
| Edita e salva um fluxo | ok | Em Skills > Agendamento de Mudança > Fluxo, trocar o título da informação `data_mudanca` salva sozinho: `PUT /api/skills/agendar-mudanca/fluxo` 200 e o `caso.yaml` mudou em disco. Não há botão Salvar. O arquivo foi restaurado depois. Ver observação 1 |
| Copiloto responde | nao testado | Copiloto: chave configurada (`LLM_MODE=luna`) mas o gateway recusou com HTTP 401. `POST /api/copiloto` voltou 500 com "Não deu para concluir. Tente de novo."; o log do servidor mostra `chamar modelo: responses: gateway retornou HTTP 401`. A rota, a permissão e o contexto chegaram até a chamada ao modelo; não é falha da integração, é credencial. Print `admin-07-copiloto.png` |
| `/admin` e `/backdoor` abrem | ok | `/admin` 200, `/backdoor/administradoras` 200, `/backdoor/payloads` 200, `/console` 200 (prints `admin-08` e `admin-09`) |

### construtor-superlogica

| Item | Resultado | O que se viu |
|---|---|---|
| Edita fluxo | ok | Mesmo fluxo acima: `PUT .../fluxo` 200, campos editáveis. Arquivo restaurado depois |
| Não vê "Administradoras" para editar | ok | Em Condomínios só aparecem as abas "Condomínios" e "Visão por skill" (sem `administradoras.editar`). Print `construtor-01-condominios.png` |
| (extra) Criar condomínio | ok | O botão "Novo condomínio" aparece e `POST` dá 201: o papel tem `condominios.editar`, coerente com as permissões de `/api/eu` |

### gestor-exemplo

| Item | Resultado | O que se viu |
|---|---|---|
| Abre em Condomínios | ok | `/` termina em `/studio/condominios` |
| Só vê condomínios de `adm-exemplo` | ok | Tela: Padrão (todos), Condomínio exemplo e (durante o teste) Smoke Teste. `Smoke Dois`, sem administradora, não aparece. `GET /api/admin/estado` devolve só `cond-001` e `smoke-teste` e só a administradora `adm-exemplo` |
| Fluxo só leitura | ok | Selo "Só leitura", todos os campos readonly/disabled, nenhum botão "Adicionar". `PUT .../fluxo` direto na API dá 403; `POST /api/admin/condominio` dá 403 |
| Sem Avançado | ok | Menu sem Avançado; abrir `/studio/avancado` direto mostra "Você não tem acesso a esta área." |
| Copiloto não cita condomínio de outra administradora | nao testado | Copiloto: chave recusada (401), sem resposta do modelo para avaliar. O recorte por administradora do estado é o mesmo do `/api/admin/estado`, que está correto (linha acima) |

### gestor-avancado

| Item | Resultado | O que se viu |
|---|---|---|
| Vê Avançado | ok | Menu com Avançado; abas Arquivos, Identidade, Prompt montado, Orquestração, Ajustes por condomínio (técnico), Importar e exportar |
| Orquestração lê (200) | ok | `GET /api/admin/orquestracao` 200; a aba mostra Router, Planning (em uso) e ReAct com os campos todos readonly e sem botão de salvar (print `gestor-avancado-02-orquestracao.png`) |
| Salvar recusa (403 legível) | ok, com ressalva | Como a tela já vem só leitura, não há botão para clicar; o teste foi direto na API: `PUT /api/admin/orquestracao` como `gestor-avancado` dá 403 `{"codigo":"SEM_PERMISSAO","error":"sem permissão para esta operação"}` |

### usuário inexistente (`localStorage['studio.usuarioDev'] = 'fantasma'`)

| Item | Resultado | O que se viu |
|---|---|---|
| Tela "Não reconhecemos seu usuário." | ok, com ressalvas | A frase aparece ("Não reconhecemos seu usuário. Entre de novo para continuar."), `/api/eu` dá 401. Ver observação 2 |

### Troca de usuário sem dado do anterior piscando

| Item | Resultado | O que se viu |
|---|---|---|
| Admin para gestor-exemplo na lista de Condomínios | ok | Medido quadro a quadro depois do clique: o conteúdo do admin (linha `Smoke Dois`, botão "Novo condomínio") aparece só no quadro do próprio clique (cerca de 65 ms, antes de o React reagir) e some no quadro seguinte; os dados do gestor entram depois. Nenhum quadro posterior mostra dado do admin |

## Observações (não bloqueiam, mas valem olhar antes do PR)

1. O fluxo salva sozinho ao editar e regrava o `caso.yaml` por inteiro: o diff do teste mostrou aspas removidas em strings (`"Manhã (8h às 12h)"` virou sem aspas), `with` com aspas simples, o comentário `# a unidade vale para a conversa toda...` perdido e a chave `reuse` movida para o fim do bloco `choose`. Semântica igual, mas o diff de uma edição de uma palavra vira 18 linhas.
2. Usuário inexistente: a mensagem aparece pequena no topo do menu lateral; a área principal continua mostrando a página anterior (`/studio/skills`) com o erro cru "usuário não autenticado" e o botão "Tentar de novo", o Copiloto continua aberto com o campo de mensagem, e o "Ver como" some, então não há como voltar pela interface (só editando o `localStorage`).
3. O Copiloto ficou sem teste de ponta a ponta por causa do 401 do gateway. Para fechar essas duas linhas (admin responde, gestor não cita outra administradora) é preciso uma chave válida no `.env` e repetir.

## Falhas

Nenhuma falha de integração encontrada. As duas linhas do Copiloto ficam "nao testado" por credencial recusada, e as observações 1 e 2 são polimento.

## Estado deixado

- Condomínios de teste (`smoke-teste`, `smoke-dois`, `smoke-tres`, `smoke-cons`) removidos pela própria API ao final; o `caso.yaml` editado voltou ao conteúdo do commit.
- Servidor `okf-admin` e Postgres do compose parados; o volume do Postgres foi mantido.
