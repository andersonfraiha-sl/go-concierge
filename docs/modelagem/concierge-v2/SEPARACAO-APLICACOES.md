# Separação das aplicações

Base anterior à extração: `efbcc2f`. Marco atual: três aplicações independentes, preservando o runtime e a persistência v2. Sem mudanças nas regras de negócio ou nos dados existentes.

As aplicações ficam em `apps/backdoor`, `apps/console` e `apps/concierge`, com as convenções Go já existentes: `cmd` para executáveis, `internal` para implementação e `web` para interfaces. O monorepo mantém um único `go.mod` e os pacotes de domínio em `internal` na raiz. Cada aplicação tem Dockerfile, Compose e exemplo de configuração próprios. O Compose da raiz orquestra o conjunto e preserva o volume do PostgreSQL.

Backdoor e Console consomem HTTP do Concierge, sem conexão com o PostgreSQL. O recebimento e a exibição das respostas emuladas pertencem ao Console; o Concierge valida a autorização e a sessão antes de aceitar uma entrega. As operações de domínio permanecem nos endpoints existentes do Concierge neste marco.

O marco seguinte extrairá a API de dados. Seu destino inclui o próprio runtime como consumidor HTTP, além das APIs do Backdoor e Console. Os nomes e camadas dessa implementação ainda serão definidos a partir dos padrões de persistência existentes, sem impor `controllers/services/repositories` nesta extração.

Condição de término: três builds e configurações independentes, fronteiras verificadas, rotas e autorização preservadas, teste de envio/recebimento emulado entre processos e instruções de execução atualizadas. Validar pacotes afetados e registrar comandos e resultados. Não gerar reservas nem outras operações reais para testar a infraestrutura.

## Organização

| Local | Responsabilidade |
|---|---|
| `apps/backdoor/cmd/backdoor` | Executável HTTP do Backdoor |
| `apps/backdoor/internal/api` | Rotas da aplicação, repasse HTTP e entrega do Studio |
| `apps/backdoor/web` | Studio e telas administrativas; assets embutidos no Backdoor |
| `apps/console/cmd/console` | Executável HTTP do Console |
| `apps/console/internal/api` | Rotas da aplicação, validação HTTP e recebimento emulado |
| `apps/console/web` | Interface de atendimento e assets embutidos no Console |
| `apps/concierge/cmd/concierge` | Runtime, webhook e APIs existentes de domínio |
| `apps/concierge/cmd/okf-run` | CLI de execução e inspeção de agentes |
| `internal` | Domínio e persistência atuais; sem importação das aplicações visuais pelo núcleo |
| `internal/httpapp` | Infraestrutura HTTP comum; sem banco ou harness |

As APIs visuais encaminham identidade, filtros e erros do backend. O Concierge continua validando autorização e escopo de empresa/canal em cada operação. As aplicações visuais não recebem credenciais de PostgreSQL, Gruvi ou LLM.

## Executar o conjunto

Na raiz, manter o `.env` existente e executar:

```bash
docker compose up -d --build
```

- Backdoor/Studio: <http://127.0.0.1:8091/studio/>
- Console: <http://127.0.0.1:8092/console>
- Concierge/API e webhook real: <http://127.0.0.1:8090>

`./build.sh --run` executa o mesmo Compose. A raiz compõe o arquivo do Concierge e estende os serviços dos Composes visuais. Rede e volume existentes continuam `go-concierge_default` e `go-concierge_pgdata_v2`; nenhuma migração de banco faz parte desta separação.

Cada processo expõe `/healthz` e `/readyz`. A readiness visual consulta a readiness do Concierge. O webhook real permanece no Concierge; as rotas de navegação entre as aplicações fazem redirecionamento para as URLs públicas configuradas. O runtime funciona sem hospedar as interfaces.

## Executar cada aplicação

A partir da raiz, para o núcleo e sua infraestrutura:

```bash
docker compose --env-file .env -f apps/concierge/docker-compose.yml up -d --build
```

Depois, cada aplicação visual pode ser iniciada separadamente, usando a rede do núcleo:

```bash
docker compose --env-file .env -f apps/backdoor/docker-compose.yml up -d --build
docker compose --env-file .env -f apps/console/docker-compose.yml up -d --build
```

Os exemplos `.env.example` locais documentam as variáveis de cada aplicação. No Compose, `CONCIERGE_INTERNAL_URL` determina o destino HTTP interno, normalmente `http://concierge:8090`. `BACKDOOR_URL` e `CONSOLE_URL` são URLs acessíveis pelo navegador. Ao personalizar portas, ajustar também essas URLs. Para produção, preservar o mecanismo de autenticação existente e definir as credenciais de serviço para o retorno emulado antes de habilitá-lo fora de DEV.

No núcleo, `CONSOLE_REPLY_URL` aponta ao receptor emulado, normalmente `http://console:8092/emulador/atende/messages`. O remetente inclui empresa e canal na query. O Console consulta `POST /api/atende/entrega-emulada/validar` antes de aceitar a mensagem e só adiciona as entregas à resposta de uma consulta de sessão autorizada.

`CONCIERGE_GOOGLE_CREDENTIALS_FILE` identifica o JSON montado no Compose. Preferir caminho absoluto para um arquivo personalizado; o padrão relativo é resolvido a partir de `apps/concierge` e aponta a `.temp/gcp/google_service_account.json` na raiz. No ambiente atual, essa variável mantém o arquivo já utilizado. `GOOGLE_APPLICATION_CREDENTIALS` continua sendo a variável do cliente Google, ajustada para o caminho interno do container.

Para executar os binários Go no host, partir da raiz e fornecer as URLs locais:

```bash
CONSOLE_REPLY_URL=http://127.0.0.1:8092/emulador/atende/messages go run ./apps/concierge/cmd/concierge
CONCIERGE_URL=http://127.0.0.1:8090 go run ./apps/backdoor/cmd/backdoor
CONCIERGE_URL=http://127.0.0.1:8090 go run ./apps/console/cmd/console
```

Executar cada comando em um terminal. O banco e as integrações do núcleo usam o `.env` da raiz. As aplicações visuais usam os próprios `.env`. O Studio compila em `apps/backdoor/web/studio/dist`; seu servidor Vite encaminha `/api` à API do Backdoor em 8091. Recompilar a imagem correspondente após editar assets embutidos.

## Validação

Os testes de arquitetura impedem dependências das aplicações visuais no banco ou no runtime. A integração `TestConsoleSeparadoProcessaConversa` inicia um processo real do Console, utiliza um canal v2 e LLM simulado, e verifica evento, processamento e entrega HTTP sem chamadas externas. Testes focados cobrem recusas de autorização e respostas de ações de pedido com e sem sessão.

Evidências desta rodada ficam em `/tmp/concierge-separacao-20261123/`. As duas falhas em fixtures antigas de `TestSessoesFicamNaImplantacao` e `TestE2EGanchosRegrasDeExemplo` foram reproduzidas separadamente na base `efbcc2f`; não são regressões da separação e não foram alteradas neste marco. Não foi executada a suíte global.

As três imagens finais foram construídas. Passaram os testes Go focados nos pacotes afetados, a integração entre processos, `go vet`, golangci-lint v2.15.0 (zero achados), typecheck/build do Studio e seis arquivos/90 testes do frontend. O Biome passou nos arquivos alterados.

O smoke das imagens finais do Backdoor e Console confirmou páginas, assets, `/readyz`, `/api/eu` e `/api/atende/config` com HTTP 200. Foram utilizados containers temporários ligados ao núcleo já ativo, somente com GET, sem chamadas reais LLM/B2C ou operações de negócio. Os containers temporários foram removidos e os serviços preexistentes permaneceram sem reinício. Status e IDs das imagens estão em `smoke-imagens.json`; builds em `build-docker-final.log` e testes HTTP finais em `testes-http-encerramento.log`, no diretório de evidências.

A stack com a separação ainda precisa ser aplicada pelo usuário com o comando indicado acima; não houve reinício dos serviços durante esta implementação.
