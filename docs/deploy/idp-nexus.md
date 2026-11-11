# Deploy no IDP Nexus

Como o go-concierge sobe na plataforma interna (IDP Nexus), no mesmo padrão do `report-manager` e do `workspaces-registry`.

## tltr

- Deploy por GitOps feito pelo próprio repositório: os workflows constroem a imagem, publicam no ECR e gravam manifestos e tag no `gitops-apps`. O ArgoCD aplica.
- Exposição no modelo do `report-manager`: Ingress ALB (`idp-shared`) para um Oathkeeper dedicado, `go-concierge-auth`. Ele valida o JWT do Auth0 e injeta `X-Authenticated-User-Email`.
- O serviço roda com `AUTH_MODE=proxy`: confia nesse cabeçalho. Por isso a NetworkPolicy só deixa o proxy chegar ao pod, e toda regra anônima do proxy zera o cabeçalho.
- Uma réplica, `Recreate`: o serviço tem cache em processo.
- Só o ambiente `dev` está pronto. Staging, prod, login do Studio no navegador e métricas ficam para depois (seção "Fica para depois").
- O repositório da org já existe: `idp-nexusapps/go-concierge` (privado). Falta a quem opera: variáveis e secret, banco, segredo no Secrets Manager, DNS e o webhook do ATENDE (seção "Passo a passo").

## O que este PR entrega

| Caminho | O que é |
|---|---|
| `cmd/okf-admin` | `AUTH_MODE=proxy` e `ACESSO_ADMIN_INICIAL` (detalhes no README, tabela "Acesso e papéis") |
| `.github/k8s/base/` | manifestos planos: app, ConfigMap, ExternalSecret, NetworkPolicy, Ingress e o proxy de autenticação |
| `.github/workflows/` | `setup`, `deploy`, `rollback`, `deploy-branch` e `destroy`, com guarda `github.repository_owner == 'idp-nexusapps'` em todo job |
| `.dockerignore` | `.github` e `docs` fora do contexto de build |

O `ci.yml` não mudou. Os manifestos ficam sem subpasta porque o `deploy.yml` copia `.github/k8s/base/*` arquivo a arquivo para `apps/go-concierge/base/` do `gitops-apps`.

Diferenças em relação aos repositórios de referência:

- O `workspaces-registry` tem um patch de overlay que aponta para um `Rule` do Oathkeeper central, com nome diferente do real, e por isso não pega nada fora de dev. Aqui o proxy é dedicado e não há `Rule`, então o patch saiu. O `report-manager` não tem patch por ambiente. As regras do proxy não fixam o host, então um host novo só exige patch no Ingress. O `deploy-branch.yml` faz esse patch para o preview.
- Sem o passo de `pnpm-lock` e sem o build-arg do Sonatype: não há `package.json` na raiz e o `Dockerfile` não usa o argumento.
- Os workflows só oferecem o ambiente `dev`.

## Passo a passo de quem opera

1. Usar o repositório `idp-nexusapps/go-concierge`. Fora da org toda a pipeline é ignorada pela guarda de dono.
2. No repositório, criar o ambiente `dev` (Settings > Environments) e preencher:
   - variável `AWS_ROLE_ARN`: role de OIDC com permissão no ECR (a plataforma informa);
   - variáveis `AWS_REGION`, `GITOPS_REPO` e `GITOPS_OWNER`, se os padrões (`us-east-1`, `gitops-apps` e o dono do repositório) não servirem;
   - secret `GITOPS_TOKEN`: token com escrita no `gitops-apps`.
3. Banco: Postgres 18 com database e usuário próprios. Anotar a `DATABASE_URL`, com `sslmode`. As migrations rodam sozinhas no boot (`-migrar`).
4. Segredo no Secrets Manager, chave `idp-go-concierge`, JSON com as chaves listadas em `.github/k8s/base/external-secret.yaml`. As obrigatórias são `DATABASE_URL`, `ATENDE_API_SECRET` e `GANCHOS_SEGREDO_EXPORTACAO`. Defina também `ACESSO_ADMIN_INICIAL` (e-mail do primeiro admin) e `INTEGRACAO_CHAVE` (`openssl rand -hex 32`).
5. Rodar o workflow `Setup Environment` com `dev`. Ele cria o ECR, publica `1.0.0`, grava base e overlay no `gitops-apps` e cria a Application do ArgoCD. Depois, criar a variável de **repositório** `DEPLOY_ENABLED=true`: só com ela o `Deploy` passa a rodar a cada push na `main`. Sem ela o `Deploy` é pulado, para não falhar a cada commit enquanto a role e o token não existem.
6. Confirmar o rollout: pods `go-concierge` e `go-concierge-auth` prontos no namespace `go-concierge`. O `/readyz` pode ser testado por `kubectl port-forward`.
7. DNS: `go-concierge.idp.nexus.superlogica.net` apontando para o ALB `idp-shared` (a confirmar se o registro já existe por curinga).
8. Cadastrar o webhook no ATENDE: `POST https://go-concierge.idp.nexus.superlogica.net/webhook/atende`, assinado com o mesmo valor de `ATENDE_API_SECRET`. Defina também `ATENDE_API_BASE_URL` no ConfigMap quando a resposta real ao morador for ligada.
9. Primeiro login: com um JWT do Auth0 cujo e-mail seja o de `ACESSO_ADMIN_INICIAL`, `GET /api/eu` deve responder o papel `admin_superlogica`. A partir daí o admin cadastra administradoras e condomínios pela API. Fora de dev o mapeamento nasce vazio e o boot registra um aviso.

Para trocar um valor do ConfigMap ou do segredo é preciso reiniciar o pod: o serviço lê o ambiente só no boot.

## Rotas expostas

| Rota | Proteção |
|---|---|
| `/api/*`, todos os métodos | JWT do Auth0 validado no proxy (mesmos emissores, JWKS e audience do `report-manager`). A claim `email` vira `X-Authenticated-User-Email`; sem ela a requisição falha no proxy. O serviço confere cadastro e permissão |
| `POST /webhook/atende` | anônimo no proxy. O serviço confere o HMAC com `ATENDE_API_SECRET` |
| `/integracao/v1/*` | anônimo no proxy. O serviço confere a chave de serviço no `Authorization`, que passa intacto |
| `GET /` e `GET /studio/*` | anônimo, só arquivos estáticos do Studio. Os dados vêm de `/api` |
| qualquer outra (`/healthz`, `/readyz`, `/console`, `/admin`, `/backdoor*`, `/emulador/*`, `/mcp/*`) | sem regra: 404 do proxy, nunca chega ao serviço |

Em toda regra anônima o proxy zera `X-Authenticated-User-Email` (mutator `header` com valor vazio). Assim um cliente não forja a identidade por uma rota sem JWT, e o serviço trata cabeçalho vazio como não autenticado. A NetworkPolicy do pod fecha o outro caminho: sem ela, qualquer pod do cluster alcançaria a 8080 direto.

Como foi conferido: a config e as regras foram carregadas em um Oathkeeper `v25.4.0` real, com um JWT de teste e um upstream que ecoa os cabeçalhos. Resultado: JWT válido injeta o e-mail e sobrescreve um cabeçalho forjado; JWT sem `email` dá 500 no proxy e não chega ao serviço; rotas anônimas entregam o cabeçalho vazio; `Authorization` passa intacto em `/integracao/v1`; as demais rotas dão 404.

## Fica para depois

| Item | Motivo |
|---|---|
| Login do Studio no navegador | o Studio ainda não obtém o JWT do Auth0 sozinho. Ele abre (arquivos estáticos), mas as chamadas a `/api` levam 401 do proxy. Falta um fluxo de login no front |
| `/metrics` e ServiceMonitor | o serviço ainda não expõe `/metrics`. Sem HPA, PDB do app e dashboard pelo mesmo motivo e porque roda com uma réplica |
| Mais de uma réplica | cache em processo (regras dos ganchos, memória, pedidos, mapeamento): uma segunda réplica serviria dado velho após escrita na outra. Exige invalidação compartilhada |
| Servidor MCP legado em Node (`mcp/`) | fora da imagem. `MCP_URL` fica vazio; os recursos vêm do Nexus ou do instantâneo `okf/nexus/<account>.yaml`. Sem `MCP_URL` o serviço também não monta o token B2C que o Gruvi usaria |
| Staging e prod | os hosts, namespaces e o patch de host do Ingress por ambiente ainda não foram definidos. Dois Ingress com o mesmo host no `idp-shared` disputariam o tráfego |
| Preview por branch | o `deploy-branch.yml` existe, mas o preview usa o mesmo segredo e banco do dev e aplica as migrations da branch nele. Use com cuidado |

## A confirmar com a plataforma

- Nome do segredo no Secrets Manager (`idp-go-concierge` é um palpite no padrão `idp-<app>`).
- Se existe um Postgres compartilhado e onde ele mora. A NetworkPolicy libera a 5432 para qualquer destino privado até isso ser definido.
- DNS de `go-concierge.idp.nexus.superlogica.net`.
- Se o ambiente `dev` do IDP atende tráfego real. O ConfigMap usa `AMBIENTE=prd`, que exige `ATENDE_API_SECRET` e `GANCHOS_SEGREDO_EXPORTACAO` e desliga os recursos de desenvolvimento.
- Provedor de modelo (`LLM_MODE`) e URL da API do ATENDE (`ATENDE_API_BASE_URL`): ficam vazios no ConfigMap.
- Se o egress HTTPS para fora das faixas privadas basta (o gateway de modelos `kong-ai-gtw` pode estar numa faixa privada).
- A imagem herda o `HEALTHCHECK` do Dockerfile, que consulta a porta 8090. O Kubernetes o ignora e usa as probes HTTP; só importa se alguém rodar a imagem no Docker.
