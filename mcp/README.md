# Servidor MCP

Este diretório contém o servidor que transforma as OpenAPIs Building Operation e Mobile em tools MCP. O concierge Go inicia o servidor pelo Compose da raiz e envia o token B2C em cada requisição. O servidor valida o formato do Bearer e o repassa ao BFF, que realiza a autorização.

No Compose, o MCP escuta na porta 3337 somente na rede interna do concierge. O endereço acessível pelo host é `http://127.0.0.1:8090/mcp/gruvi/bff`.

O arquivo `mcp/.env` configura as URLs dos BFFs e é ignorado pelo Git. O exemplo aponta para um mock em `127.0.0.1:3344`; para integrações reais, configure `MCP_VIEWS_UPSTREAM_BASE_URL` e, se necessário, `MCP_VIEWS_MOBILE_UPSTREAM_BASE_URL` e `MCP_VIEWS_MOBILE_APP_CHECK_TOKEN`. A autenticação do concierge fica no `.env` da raiz.

Para executar o MCP diretamente, use Node.js 24 e:

```sh
npm ci
npm run start:mcp:catalog-agent
```

Nesse caso o servidor e o Explorer ficam em `http://127.0.0.1:3337`. Para verificar o código, execute `npm run typecheck` e `npm test`. Consulte o [README da raiz](../README.md) para configurar e subir o conjunto completo.
