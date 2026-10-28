# Servidor MCP oficial da entrega (Capability Views): transforma as OpenAPIs do BFF do
# Gruvi (building-operation.json e mobile.json) em tools MCP em /mcp/gruvi/bff, com
# autenticação request_bearer (token B2C do Keycloak repassado ao BFF).
FROM node:24-alpine
WORKDIR /app
COPY package.json package-lock.json ./
# tsx e typescript são devDependencies, mas o servidor roda direto do .ts: instala tudo
RUN npm ci
COPY . .
ENV MCP_CATALOG_AGENT_MCP_HOST=127.0.0.1 \
    MCP_CATALOG_AGENT_MCP_PORT=3337 \
    NODE_ENV=production
EXPOSE 3337
CMD ["npx", "tsx", "mcp-catalog-agent/server.ts"]
