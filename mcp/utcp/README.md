# Publicador do manual UTCP

Importado da POC `agent-utcp` em `/home/vanderlei.martins/superlogica-estudo-pocs/mcp-server-with-agent`. Este diretório contém a geração do manual a partir de OpenAPI e seu servidor HTTP. A execução dos recursos pelo cliente UTCP fica para uma etapa posterior.

Execute `npm run manual:utcp` a partir de `mcp/`. `GET http://127.0.0.1:3345/utcp` retorna o manual; `GET http://127.0.0.1:3345/` retorna o índice de fontes, contagens e operações não suportadas. O publicador aceita apenas GET e acesso local. No Compose, o serviço `utcp` compartilha a rede do Concierge e os endpoints externos são `/utcp` e `/utcp/`, com a mesma permissão de Superlógica usada no repasse MCP.

As definições são geradas no início do processo. Alterações nas OpenAPIs ou na configuração exigem reinício. O processo lê documentos JSON ou YAML de arquivo ou URL HTTP, sem executar operações das APIs.

| Configuração | Comportamento |
|---|---|
| `UPSTREAM_BASE_URL` ou `MCP_VIEWS_UPSTREAM_BASE_URL` | URL base das operações Building Operation; obrigatório |
| `MCP_VIEWS_OPENAPI_PATH` | OpenAPI Building Operation; padrão `building-operation.json` |
| `MCP_VIEWS_MOBILE_OPENAPI_PATH` | OpenAPI Mobile; padrão `mobile.json` |
| `MCP_VIEWS_MOBILE_UPSTREAM_BASE_URL` | URL base Mobile; usa a de Building Operation quando vazia |
| `MCP_VIEWS_MOBILE_APP_CHECK_TOKEN` | Quando configurado, permite publicar operações que exigem App Check; o valor não entra no manual |
| `MCP_VIEWS_COND_CORE_UPSTREAM_BASE_URL` | Habilita CondCore; exige disponibilizar a OpenAPI em `MCP_VIEWS_COND_CORE_OPENAPI_PATH` (padrão `cond-core.json`) |
| `AGENT_UTCP_PARECER_ENABLED` | Habilita Parecer Pasta; padrão `false` |
| `AGENT_UTCP_PARECER_OPENAPI_URL` | OpenAPI Parecer; padrão URL pública Nexus da POC |
| `AGENT_UTCP_PARECER_UPSTREAM_BASE_URL` | URL base Parecer; padrão gateway Nexus da POC |
| `AGENT_UTCP_MANUAL_HOST` / `AGENT_UTCP_MANUAL_PORT` | Endereço local do servidor; padrão `127.0.0.1:3345` |
| `AGENT_UTCP_OPENAPI_TIMEOUT_MS` | Prazo de download das OpenAPIs; padrão `10000` ms |
| `AGENT_UTCP_OPENAPI_MAX_BYTES` | Limite por documento OpenAPI; padrão `2097152` bytes |
| `AGENT_UTCP_HTTP_TIMEOUT_MS` | Timeout indicado nos templates de execução; padrão `15000` ms |

O conversor oficial `@utcp/http` transforma operações e schemas. A montagem preserva `summary` e `description`, normaliza UUID/nullable e referências locais, cria nomes únicos por fonte e respeita `x-mcp=false`. Operações não suportadas, como upload multipart, respostas binárias e API key sem suporte, são omitidas e aparecem no índice.

Os templates contêm referências às variáveis `B2C_TOKEN`, `MOBILE_APP_CHECK_TOKEN`, `PARECER_GATEWAY_TOKEN` e `PARECER_COMPANY_ID`, conforme a fonte. Os valores serão fornecidos pelo futuro cliente; o publicador não obtém nem renova tokens. Os schemas de entrada não pedem essas credenciais ao modelo.

Validação: `npm run typecheck` e `npm test`, a partir de `mcp/`.
