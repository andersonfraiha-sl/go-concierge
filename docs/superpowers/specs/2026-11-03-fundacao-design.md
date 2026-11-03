# Fundação: Postgres, acesso por papéis e o padrão funcional

Data: 2026-11-03 · Projeto 1 de 5 · Constituição: `docs/superpowers/constitution.md`

## 1. Por que

O concierge vai ganhar memória classificada (projeto 2), um Studio novo usado por Superlógica e administradoras (projeto 3) e um copiloto (projeto 4). Nenhum deles se sustenta na base atual:

- **Tudo persiste em arquivo JSON**: o mapeamento das administradoras (`okf/var/admin/mapeamento.json`), a memória por telefone (`okf/var/memorias/`) e os pedidos (`internal/pedidos`, uma carteira JSON por telefone). As sessões do ATENDE vivem só na memória do processo. Com duas réplicas, uma sobrescreve a outra; um restart perde as sessões; nada pode ser consultado.
- **Não existe autenticação.** Toda rota `/api/*` é aberta. O "entrar como administradora" do Admin é um seletor que emula o login.
- **O código mistura decisão com I/O** e tem erros engolidos (`memoria.go:180`, `:202`; `harness/memoria.go:92`).

A Fundação resolve as três coisas e deixa um padrão que os projetos seguintes copiam.

## 2. Objetivos, não objetivos e anti objetivos

**Objetivos**
1. Postgres como único armazenamento, com sqlc + pgx e migrations goose.
2. Os quatro armazenamentos atuais (mapeamento, memória, pedidos, sessões) no Postgres, **com o mesmo comportamento visto de fora**, mais um comando que importa os JSON existentes.
3. Autenticação e autorização em toda rota `/api/*`, com papéis em herança e escopo.
4. O padrão funcional estabelecido e demonstrado nos pacotes que esta fase toca.

5. **Um concierge para todas as administradoras** (seção 4b). Hoje cada processo atende uma conta só (`DEFAULT_ACCOUNT_ID`).

**Não objetivos** (cada um tem seu projeto)
- Novo modelo de memória com tipos, Jev e frecência (projeto 2). Aqui a memória muda de lugar, não de forma.
- Studio novo e nomes comuns (projeto 3). As telas atuais continuam funcionando.
- Catálogo de skills (`okf/agentes/`) no banco (projeto 3 decide). Continua em arquivo.
- Refatorar harness e motor de pedidos inteiros (projeto 5). Só o que esta fase toca.

**Anti objetivos**
- Uma administradora conseguir ler ou alterar dado de outra, por qualquer rota.
- Perder dado na migração dos JSON.
- Quebrar o atendimento em andamento (webhook do ATENDE e Console).

## 3. Banco e acesso a dados

- **Postgres 16** no `docker-compose.yml` como serviço `postgres`, com volume. O concierge recebe `DATABASE_URL`.
- **Sem SQLite** (minha decisão, diverge do DEFINICOES 4.6): com sqlc, dois dialetos dobram todas as queries. Desenvolvimento local usa o Postgres do compose; testes de integração usam testcontainers.
- **Estrutura**:
  ```
  db/migrations/   NNNN_<nome>.sql  (goose, up e down)
  db/queries/      <área>.sql       (sqlc)
  internal/db/     código gerado pelo sqlc (não editado à mão)
  sqlc.yaml
  ```
- As migrations rodam no boot (`goose.Up`) atrás de flag `-migrar` (padrão ligado em dev, desligado em prod, onde rodam como job).
- Memória e pedidos já têm interface `Store` (`memoria.Store`, `pedidos.Store`): troca-se a implementação. O mapeamento (`admin.Store` é uma struct concreta) e as sessões (hoje um `map[string]*Sessao` com mutex em `atende/sessao.go`, sem abstração) ganham a interface do zero. A implementação em arquivo **sai**: fica só a Postgres e um fake em memória para testes unitários.

### Tabelas (forma inicial)

| Área | Tabelas | Observação |
|---|---|---|
| Mapeamento | `administradoras`, `condominios`, `habilitacoes`, `payloads`, `orientacoes` | `"*"` (padrão global) vira `condominio_id NULL` |
| Memória | `memoria_fatos (administradora_id, condominio_id, telefone, chave, texto, fonte, quando, confianca)` | `UNIQUE (administradora_id, telefone, chave)` reproduz o "mesma chave substitui"; o fato é da pessoa na administradora, `condominio_id` registra onde foi aprendido |
| Pedidos | `pedidos`, `pedido_eventos` | estado atual + histórico de transições; com `administradora_id` e `condominio_id` |
| Sessões | `sessoes`, `sessao_turnos` | o histórico do Eino em `jsonb`; com `administradora_id` e `condominio_id` |
| Acesso | ver seção 4 | |

Payloads do Nexus e histórico do Eino ficam em `jsonb`, validados na borda (exceção prevista na constituição).

### Importação

`okf-admin importar-json -dir okf/var` lê os JSON atuais e grava no Postgres numa transação por área. É idempotente (`ON CONFLICT DO NOTHING`) e imprime a contagem lida e gravada por área; diferença entre as duas é erro.

## 4b. Um concierge para todas as administradoras

- `DEFAULT_ACCOUNT_ID` deixa de definir o processo. A cada mensagem, o condomínio é resolvido como já acontece em `atende/servico.go:819` (`Mapeamento.Resolver` por company do Gruvi, tenant do ATENDE e account); do condomínio saem a administradora e a account do Nexus usada nas tools.
- Mensagem cujo condomínio não resolve não é atendida pelo agente: vai para transbordo com log de aviso (hoje cai no account padrão).
- Toda linha de memória, pedido e sessão grava `administradora_id` e `condominio_id` resolvidos na conversa; é isso que as rotas `/api/atende/*`, `/api/memoria*` e `/api/pedidos*` usam como alvo na checagem de acesso. Essas rotas passam a receber `condominio_id` (ou derivam da sessão pelo `id`).
- O Console continua emulando um telefone; o seletor de condomínio dele define company/tenant da mensagem forjada.
- `DEFAULT_ACCOUNT_ID` só sobrevive como valor do emulador em dev.

## 4. Acesso: papéis com herança e escopo

### Conceitos

- **Usuário**: quem entra no Studio/Admin.
- **Papel de acesso**: conjunto de permissões, que pode herdar de outro papel. Nome diferente do "papel do morador" que já existe em `Habilitacao.Papeis` (proprietário, inquilino), que continua igual e não tem relação com isto.
- **Escopo**: onde a atribuição vale: `superlogica` (tudo), `administradora:<id>` (ela e seus condomínios) ou `condominio:<id>`.
- **Permissão**: verbo sobre área, ex. `skills.ler`, `skills.editar_textos`, `skills.editar_fluxo`, `condominios.habilitar`, `studio.avancado`.
- **Concessão**: permissão dada a uma administradora inteira, que os papéis dela passam a ter. É assim que o Avançado é liberado: a Superlógica concede `studio.avancado` à administradora X.

### Papéis iniciais (herança de baixo para cima)

```
operador_administradora  → ler skills, ver conversas e pedidos dos seus condomínios
  └ gestor_administradora → + habilitar skills por condomínio, editar textos, regras, persona e base de conhecimento
      └ construtor_superlogica → + criar/excluir skills, editar fluxo, ferramentas, studio.avancado
          └ admin_superlogica   → + administradoras, usuários, papéis, concessões
```

`studio.avancado` chega ao gestor só por concessão à administradora dele.

### Resolução (função pura)

```go
func Permitido(acesso Acesso, perm Permissao, alvo Escopo) bool
```

`Acesso` é montado na borda (uma query) com: atribuições do usuário, grafo de herança dos papéis e concessões das administradoras envolvidas. A função expande a herança, aplica concessões e confere se algum escopo atribuído **contém** o alvo (`superlogica ⊃ administradora ⊃ condomínio`). Testes em tabela cobrem herança, concessão, escopo vizinho negado e ciclo na herança (rejeitado na gravação, não na leitura).

### Tabelas

`usuarios`, `papeis (id, nome, herda_de NULL)`, `papel_permissoes`, `atribuicoes (usuario_id, papel_id, escopo_tipo, escopo_id)`, `concessoes (administradora_id, permissao)`.

### Onde é aplicado

- Um middleware resolve o usuário, monta `Acesso` e põe no `context`.
- Cada handler declara a permissão e o alvo; sem permissão → `403` com corpo `{codigo, mensagem}`.
- As rotas `/api/admin/*` que hoje recebem a administradora como parâmetro passam a usar o escopo do usuário; o seletor "entrar como" fica só para quem tem escopo `superlogica`.
- **Fora do middleware**: `POST /webhook/atende`, `/healthz` e os arquivos estáticos.
- **Webhook do ATENDE passa a exigir HMAC** com `ATENDE_API_SECRET` (o código já existe em `atende/assinatura.go`, hoje desligado). Assinatura inválida ou ausente → `401`, sem processar. Sem `ATENDE_API_SECRET` o boot falha fora de dev. A rota do emulador continua com `ATENDE_WEBHOOK_TOKEN`.

### Autenticação

O provedor de login fica para depois. A Fundação entrega a identidade atrás de uma interface (`Autenticador: requisição → usuário`) com uma só implementação: `AUTH_MODE=dev`, que aceita o cabeçalho `X-Dev-Usuario`. O boot **recusa** `AUTH_MODE=dev` quando `AMBIENTE=prd`, então nada vai a produção sem o provedor real.

## 5. Padrão funcional em Go

Vale para todo código novo e para o que esta fase tocar:

- **Pacote de domínio sem I/O**: tipos e funções puras (`acesso.Permitido`, `admin.HabilitacaoEfetiva`, transições de pedido). Testes em tabela, sem banco.
- **Store na borda**: traduz entre linhas do sqlc e tipos de domínio. Teste de integração com testcontainers.
- **Handler fino**: decodifica, valida, chama domínio e store, codifica. Erro de negócio mapeado para status por tabela, não por `if` espalhado.
- **Valores, não mutação**: funções devolvem a struct nova (`memoria.Upsert` passa a devolver `(Memoria, bool)` em vez de alterar o receptor).
- **Erros**: sentinelas por pacote (`ErrNaoEncontrado`, `ErrSemPermissao`), contexto sem "failed to" (`"gravar pedido %s: %w"`); os três erros engolidos atuais são corrigidos.
- **Dependências injetadas no `main`**; nada de estado global; toda chamada externa com `context` e timeout.
- **Logs** com `log/slog` em JSON, com `request_id`, `usuario_id`, `operacao`, `duracao_ms`; nunca telefone completo, CPF ou conteúdo da mensagem.

## 6. Ordem de entrega (cada item é um PR que funciona sozinho)

1. Postgres no compose, goose, sqlc, `DATABASE_URL`, healthcheck que testa o banco.
2. Mapeamento das administradoras no Postgres + importação.
3. Memória (forma atual) no Postgres + importação + correção dos erros engolidos.
4. Pedidos no Postgres + importação.
5. Sessões do ATENDE no Postgres (sobrevivem a restart).
6. Multi-administradora: condomínio resolvido por mensagem, `administradora_id`/`condominio_id` em memória, pedidos e sessões, fim do `DEFAULT_ACCOUNT_ID` fora do emulador.
7. Acesso: tabelas, `Permitido`, middleware, `AUTH_MODE=dev`, aplicação em todas as rotas `/api/*`.
8. HMAC obrigatório no webhook do ATENDE.
9. Remoção das implementações em arquivo.

## 7. Como saber que ficou pronto

- `docker compose up` sobe concierge + Postgres + MCP e o Console atende uma conversa ponta a ponta com o modelo mock.
- Restart do concierge no meio de uma conversa: a sessão continua.
- Importação dos JSON de `okf/var` com contagem igual nos dois lados.
- Testes de tabela de `Permitido` cobrindo: herança, concessão do Avançado, administradora vizinha negada, condomínio fora do escopo negado.
- Um usuário `gestor_administradora` da administradora A recebe `403` em toda rota que aponta para dado da administradora B.
- Duas mensagens de condomínios de administradoras diferentes no mesmo processo: cada uma usa a account do seu condomínio e grava com a administradora certa.
- Webhook do ATENDE sem assinatura válida → `401`.
- `go vet` e `go test ./...` verdes.

## 8. Em aberto

1. **Provedor de login** (Auth0 ou Keycloak): decidido depois; até lá nada vai a produção (seção 4).

## 8b. Decisões do Felipe nesta conversa

- Um concierge para todas as administradoras.
- Login decidido depois; Fundação só com modo dev.
- HMAC do webhook ligado nesta fase (reverte a decisão anterior do Anderson; avisar o Anderson).

## 9. Decisões que tomei

- Sem SQLite em dev (diverge do DEFINICOES 4.6).
- Catálogo de skills continua em arquivo até o projeto 3.
- Quatro papéis iniciais e a lista de permissões acima.
- "Papel de acesso" com nome distinto do "papel do morador".
- Migrations no boot só em dev.
