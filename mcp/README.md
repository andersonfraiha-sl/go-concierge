# MCP Catalog Agent — início rápido

Esta entrega contém o agente, o servidor MCP que transforma as OpenAPIs Building Operation e Mobile em tools e o Explorer local. O `.env` foi copiado integralmente da origem. O ZIP contém credenciais: compartilhe-o somente por canal privado com a pessoa autorizada e não publique nem adicione o `.env` ao Git.

## Pré-requisitos

- Node.js 24 LTS e npm. A entrega foi preparada com Node.js 24.11.1 e npm 11.6.2.
- Acesso à rede dos serviços configurados no `.env`, incluindo o gateway de LLM e os BFFs. Se a rede corporativa exigir VPN ou outras permissões, configure-as antes de executar.
- Credenciais válidas para os serviços. O `B2C_CLIENT_KEYCLOAK_TOKEN` incluído expirou em 28/10/2026 às 11h43 (Brasília): obtenha um token B2C válido pelo processo autorizado da sua equipe e substitua apenas o valor dessa variável no `.env`. Ela autentica a conexão do agente com o MCP e é repassada ao BFF. Reinicie o servidor e o agente após alterar o token. Os tokens do gateway de LLM também expiram; renove `LLM_LUNA_GATEWAY_TOKEN` ou `LLM_GATEWAY_TOKEN` quando necessário. A API Mobile pode exigir `MCP_VIEWS_MOBILE_APP_CHECK_TOKEN`, conforme o ambiente e a operação.
- Confira `CONCIERGE_COMPANY_ID` e `CONCIERGE_PHONE` no `.env`: são o contexto enviado ao agente. As URLs já estão configuradas no arquivo entregue; use os endpoints e as permissões corretos para o ambiente desejado.

## Instalação

Extraia o ZIP, abra um terminal na pasta que contém este README e execute:

```sh
npm ci
```

A instalação precisa de acesso ao registro npm. Não é necessário copiar `node_modules` nem compilar o projeto para rodar os comandos abaixo.

## Execução

No primeiro terminal, a partir da pasta deste README:

```sh
npm run start:mcp:catalog-agent
```

Aguarde a mensagem de inicialização. O MCP usa por padrão `http://127.0.0.1:3337/mcp/gruvi/bff`; o Explorer local fica em `http://127.0.0.1:3337/`.

Abra outro terminal na mesma pasta e execute:

```sh
npm run agent:mcp-catalog
```

Ao aparecer `Você:`, converse com o agente. Use `/tools` para listar o catálogo e `/sair` para encerrar o agente. Encerre o servidor no primeiro terminal com Ctrl+C. Mantenha o MCP em execução durante a conversa. As ferramentas podem acessar e alterar dados reais de acordo com as permissões do token e a solicitação feita ao agente.

Para reinício automático durante desenvolvimento, use `npm run dev:mcp:catalog-agent` e `npm run dev:agent:mcp-catalog` em terminais separados. Caso a porta 3337 esteja ocupada, configure `MCP_CATALOG_AGENT_MCP_PORT` e `MCP_CATALOG_AGENT_MCP_URL` de forma consistente no `.env`.

## Registro e verificações

Cada inicialização substitui `mcp-catalog-agent/session-mcp-catalog-agent.json` por uma nova sessão. Esse arquivo pode conter mensagens e resultados das consultas; trate-o como dado sensível. O viewer e as instruções de uso estão em `mcp-catalog-agent/`. Mais detalhes técnicos estão em `mcp-catalog-agent/README.md`.

Para verificar o pacote localmente, execute `npm run typecheck` e `npm test`. Os testes incluídos usam mocks locais, sem chamadas reais ao gateway ou BFF. Para erros de autenticação, verifique expiração e permissões dos tokens; para erros de conexão, confirme as URLs e o acesso à rede. O código-fonte e as duas OpenAPIs estão incluídos; não há banco ou Docker para iniciar nesta POC.
