import SwaggerParser from '@apidevtools/swagger-parser';
import { openapiSchemaToJsonSchema } from '@openapi-contrib/openapi-schema-to-json-schema';
import { McpServer, fromJsonSchema, type JsonSchemaType } from '@modelcontextprotocol/server';
import { AjvJsonSchemaValidator } from '@modelcontextprotocol/server/validators/ajv';
import { NodeStreamableHTTPServerTransport } from '@modelcontextprotocol/node';
import { createServer } from 'node:http';
import { pathToFileURL } from 'node:url';
import { readFile } from 'node:fs/promises';
import { randomUUID } from 'node:crypto';
import { Client, StreamableHTTPClientTransport } from '@modelcontextprotocol/client';

export interface McpServerConfig {
  openapiPath: string;
  upstreamBaseUrl: string;
  upstreamBearerToken?: string;
  getUpstreamBearerToken?: () => Promise<string>;
  host?: string;
  port?: number;
  httpTimeoutMs?: number;
  httpMaxResponseBytes?: number;
  onExecution?: (execution: ToolExecution) => void;
}

export interface ToolExecution {
  id: string;
  timestamp: string;
  toolName: string;
  durationMs: number;
  kind: string;
  status?: number;
  errorCode?: string;
}

export interface ClientCredentialsConfig {
  keycloakUrl: string;
  clientId: string;
  clientSecret: string;
  now?: () => number;
}

export function createBffTokenProvider(config: ClientCredentialsConfig): () => Promise<string> {
  if (!config.keycloakUrl || !config.clientId || !config.clientSecret) throw new Error('Configure BFF_KEYCLOAK_URL, BFF_KEYCLOAK_CLIENT_ID e BFF_KEYCLOAK_CLIENT_SECRET');
  const base = new URL(config.keycloakUrl);
  if (!['http:', 'https:'].includes(base.protocol) || base.username || base.password || base.search || base.hash) throw new Error('BFF_KEYCLOAK_URL inválida');
  const tokenUrl = config.keycloakUrl.replace(/\/$/, '') + '/token';
  const now = config.now ?? Date.now;
  let cachedToken = '';
  let validUntil = 0;
  let pending: Promise<string> | undefined;
  return async () => {
    if (cachedToken && now() < validUntil) return cachedToken;
    if (!pending) {
      pending = (async () => {
        const response = await fetch(tokenUrl, {
          method: 'POST', redirect: 'error', signal: AbortSignal.timeout(10_000),
          headers: { 'content-type': 'application/x-www-form-urlencoded' },
          body: new URLSearchParams({ grant_type: 'client_credentials', client_id: config.clientId, client_secret: config.clientSecret }),
        });
        if (!response.ok) throw new Error('BFF_AUTH_HTTP_' + response.status);
        const body = await response.json() as { access_token?: unknown; expires_in?: unknown };
        if (typeof body.access_token !== 'string' || !body.access_token.trim()) throw new Error('BFF_AUTH_INVALID_RESPONSE');
        cachedToken = body.access_token;
        const ttl = typeof body.expires_in === 'number' && Number.isFinite(body.expires_in) ? body.expires_in : 0;
        validUntil = now() + Math.max(0, ttl * 1000 - 30_000);
        return cachedToken;
      })().finally(() => { pending = undefined; });
    }
    return pending;
  };
}

interface Schema {
  [key: string]: unknown;
  type?: string;
  description?: string;
  properties?: Record<string, Schema>;
  required?: string[];
}

interface Parameter {
  name: string;
  in: string;
  required?: boolean;
  description?: string;
  schema: Schema;
  style?: string;
  explode?: boolean;
  allowReserved?: boolean;
  content?: unknown;
}

interface OperationSource {
  operationId?: string;
  summary?: string;
  description?: string;
  parameters?: Parameter[];
  requestBody?: { required?: boolean; description?: string; content: Record<string, { schema?: Schema }> };
  security?: Record<string, string[]>[];
  responses?: unknown;
  'x-mcp'?: boolean;
}

interface OpenApiDocument {
  openapi: string;
  paths: Record<string, Record<string, unknown> & OperationSource>;
  security?: Record<string, string[]>[];
  components?: { securitySchemes?: Record<string, { type?: string; scheme?: string }> };
  'x-mcp'?: boolean;
}

export interface OpenApiOperation {
  toolName: string;
  operationId: string;
  method: string;
  pathTemplate: string;
  description: string;
  parameters: Parameter[];
  inputSchema: Schema;
  requiresBearer: boolean;
  responses?: unknown;
}

export interface OpenApiCatalog {
  operations: OpenApiOperation[];
  discovered: number;
  excluded: number;
}

export interface ToolArguments {
  path?: Record<string, unknown>;
  query?: Record<string, unknown>;
  headers?: Record<string, unknown>;
  body?: unknown;
}

const methods = new Set(['get', 'post', 'put', 'patch', 'delete', 'head', 'options', 'trace']);
const reservedHeaders = /^(authorization|proxy-authorization|host|cookie|set-cookie|connection|content-length|transfer-encoding|upgrade|expect|forwarded|x-forwarded-.+)$/i;

function convertSchema(schema: Schema): Schema {
  const converted = openapiSchemaToJsonSchema(schema, { cloneSchema: true }) as Schema;
  delete converted.$schema;
  return converted;
}

function inputSchema(parameters: Parameter[], source: OperationSource): Schema {
  const properties: Record<string, Schema> = {};
  const required: string[] = [];
  for (const parameter of parameters) {
    if (!['path', 'query', 'header'].includes(parameter.in)) throw new Error('Localização de parâmetro não suportada: ' + parameter.in);
    if (!parameter.schema || parameter.content || !['string', 'number', 'integer', 'boolean'].includes(parameter.schema.type ?? '')) {
      throw new Error('Somente parâmetros escalares com schema são suportados: ' + parameter.name);
    }
    const expectedStyle = parameter.in === 'query' ? 'form' : 'simple';
    if ((parameter.style && parameter.style !== expectedStyle) || parameter.allowReserved) throw new Error('Serialização não suportada para ' + parameter.name);
    if (parameter.in === 'header' && reservedHeaders.test(parameter.name)) throw new Error('Header reservado não pode ser argumento de tool: ' + parameter.name);
    const groupName = parameter.in === 'header' ? 'headers' : parameter.in;
    const group = properties[groupName] ??= { type: 'object', properties: {}, required: [], additionalProperties: false };
    group.properties![parameter.name] = { ...convertSchema(parameter.schema), ...(parameter.description ? { description: parameter.description } : {}) };
    if (parameter.required || parameter.in === 'path') group.required!.push(parameter.name);
  }
  for (const [name, group] of Object.entries(properties)) {
    if (group.required!.length) required.push(name);
  }
  if (source.requestBody) {
    const content = source.requestBody.content;
    if (!content['application/json']?.schema || Object.keys(content).some(type => type !== 'application/json')) throw new Error('Somente requestBody application/json com schema é suportado');
    properties.body = { ...convertSchema(content['application/json'].schema), ...(source.requestBody.description ? { description: source.requestBody.description } : {}) };
    if (source.requestBody.required) required.push('body');
  }
  return { $schema: 'http://json-schema.org/draft-07/schema#', type: 'object', properties, required, additionalProperties: false };
}

function requiresBearer(source: OperationSource, document: OpenApiDocument): boolean {
  const security = source.security ?? document.security ?? [];
  if (!security.length || security.some(requirement => Object.keys(requirement).length === 0)) return false;
  for (const requirement of security) {
    const names = Object.keys(requirement);
    if (names.length === 1) {
      const scheme = document.components?.securitySchemes?.[names[0]!];
      if (scheme?.type === 'http' && scheme.scheme?.toLowerCase() === 'bearer') return true;
    }
  }
  throw new Error('Esquema de autenticação não suportado: ' + source.operationId);
}

export async function loadOpenApiOperations(specPath: string): Promise<OpenApiCatalog> {
  const document = await SwaggerParser.dereference(specPath, { resolve: { http: false }, dereference: { circular: false } }) as unknown as OpenApiDocument;
  if (!document.openapi?.startsWith('3.0.')) throw new Error('A POC suporta OpenAPI 3.0');
  const catalog: OpenApiCatalog = { operations: [], discovered: 0, excluded: 0 };
  const operationIds = new Set<string>();
  const toolNames = new Set<string>();
  for (const [pathTemplate, pathItem] of Object.entries(document.paths)) {
    for (const [method, value] of Object.entries(pathItem)) {
      if (!methods.has(method)) continue;
      catalog.discovered += 1;
      const source = value as OperationSource;
      const exposure = source['x-mcp'] ?? pathItem['x-mcp'] ?? document['x-mcp'] ?? true;
      if (typeof exposure !== 'boolean') throw new Error('x-mcp deve ser boolean em ' + method + ' ' + pathTemplate);
      if (!exposure) { catalog.excluded += 1; continue; }
      const operationId = source.operationId;
      if (!operationId || operationIds.has(operationId)) throw new Error('operationId ausente ou duplicado: ' + (operationId ?? method + ' ' + pathTemplate));
      operationIds.add(operationId);
      const toolName = operationId.replace(/[^a-zA-Z0-9_.-]/g, '_');
      if (!toolName || toolName.length > 128 || toolNames.has(toolName)) throw new Error('Nome de tool inválido ou colidente: ' + toolName);
      toolNames.add(toolName);
      if (!pathTemplate.startsWith('/') || pathTemplate.startsWith('//') || /[\\?#]/.test(pathTemplate) || pathTemplate.split('/').some(segment => segment === '.' || segment === '..')) throw new Error('Path de operation inválido: ' + pathTemplate);
      const mergedParameters = new Map<string, Parameter>();
      for (const parameter of [...(pathItem.parameters ?? []), ...(source.parameters ?? [])]) mergedParameters.set(parameter.in + ':' + parameter.name, parameter);
      const parameters = [...mergedParameters.values()];
      const placeholders = [...pathTemplate.matchAll(/\{([^}]+)\}/g)].map(match => match[1]);
      if (placeholders.some(name => !parameters.some(parameter => parameter.in === 'path' && parameter.name === name))) throw new Error('Path sem declaração de parâmetro em ' + operationId);
      catalog.operations.push({
        toolName, operationId, method: method.toUpperCase(), pathTemplate,
        description: [source.summary, source.description].filter(Boolean).join('\n\n') || operationId + ' — ' + method.toUpperCase() + ' ' + pathTemplate,
        parameters, inputSchema: inputSchema(parameters, source),
        requiresBearer: requiresBearer(source, document), responses: source.responses,
      });
    }
  }
  return catalog;
}

function validateConfig(config: McpServerConfig, catalog: OpenApiCatalog): URL {
  if (!config.upstreamBaseUrl) throw new Error('UPSTREAM_BASE_URL é obrigatório');
  const baseUrl = new URL(config.upstreamBaseUrl);
  if (!['http:', 'https:'].includes(baseUrl.protocol) || baseUrl.username || baseUrl.password || baseUrl.search || baseUrl.hash) throw new Error('UPSTREAM_BASE_URL deve ser HTTP(S), sem credenciais, query ou fragmento');
  if (catalog.operations.some(operation => operation.requiresBearer) && !config.upstreamBearerToken && !config.getUpstreamBearerToken) throw new Error('UPSTREAM_BEARER_TOKEN ou client credentials são obrigatórios para as operações autenticadas');
  for (const [name, value] of Object.entries({ httpTimeoutMs: config.httpTimeoutMs ?? 10_000, httpMaxResponseBytes: config.httpMaxResponseBytes ?? 1_048_576 })) {
    if (!Number.isSafeInteger(value) || value <= 0) throw new Error(name + ' deve ser um inteiro positivo');
  }
  if (config.port !== undefined && (!Number.isInteger(config.port) || config.port < 0 || config.port > 65535)) throw new Error('MCP_PORT inválida');
  return baseUrl;
}

function toolResult(payload: Record<string, unknown>, isError = false) {
  return { content: [{ type: 'text' as const, text: JSON.stringify(payload) }], structuredContent: payload, isError };
}

export async function executeOpenApiOperation(operation: OpenApiOperation, args: ToolArguments, config: McpServerConfig) {
  let renderedPath: string;
  try {
    renderedPath = operation.pathTemplate.replace(/\{([^}]+)\}/g, (_, name: string) => {
      const value = String(args.path?.[name]);
      if (args.path?.[name] === undefined || value === '.' || value === '..') throw new Error('Parâmetro de path inválido');
      return encodeURIComponent(value);
    });
  } catch {
    return toolResult({ kind: 'argument_error', message: 'Parâmetro de path ausente ou inseguro' }, true);
  }
  const baseUrl = new URL(config.upstreamBaseUrl);
  const url = new URL(baseUrl);
  url.pathname = baseUrl.pathname.replace(/\/$/, '') + renderedPath;
  const headers = new Headers({ accept: 'application/json, text/plain;q=0.9, */*;q=0.1' });
  for (const parameter of operation.parameters) {
    if (parameter.in === 'query' && args.query?.[parameter.name] !== undefined) url.searchParams.append(parameter.name, String(args.query[parameter.name]));
    if (parameter.in === 'header' && args.headers?.[parameter.name] !== undefined) headers.set(parameter.name, String(args.headers[parameter.name]));
  }
  if (operation.requiresBearer) {
    try {
      const token = config.getUpstreamBearerToken ? await config.getUpstreamBearerToken() : config.upstreamBearerToken;
      if (!token) throw new Error('BFF_AUTH_TOKEN_MISSING');
      headers.set('authorization', 'Bearer ' + token);
    } catch (error) {
      const code = error instanceof Error && /^BFF_AUTH_[A-Z0-9_]+$/.test(error.message) ? error.message : 'BFF_AUTH_UNAVAILABLE';
      return toolResult({ kind: 'auth_error', code, message: 'Não foi possível autenticar na API upstream: ' + code }, true);
    }
  }
  if (args.body !== undefined) headers.set('content-type', 'application/json');
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), config.httpTimeoutMs ?? 10_000);
  let responseTooLarge = false;
  try {
    const response = await fetch(url, {
      method: operation.method, headers, redirect: 'manual', signal: controller.signal,
      ...(args.body !== undefined ? { body: JSON.stringify(args.body) } : {}),
    });
    const chunks: Uint8Array[] = [];
    let bytes = 0;
    const maxBytes = config.httpMaxResponseBytes ?? 1_048_576;
    if (Number(response.headers.get('content-length')) > maxBytes) {
      responseTooLarge = true;
      controller.abort();
      return toolResult({ kind: 'transport_error', message: 'Resposta excede o limite configurado' }, true);
    }
    if (response.body) {
      for await (const chunk of response.body) {
        bytes += chunk.byteLength;
        if (bytes > maxBytes) {
          responseTooLarge = true;
          controller.abort();
          return toolResult({ kind: 'transport_error', message: 'Resposta excede o limite configurado' }, true);
        }
        chunks.push(chunk);
      }
    }
    const text = Buffer.concat(chunks).toString('utf8');
    const contentType = response.headers.get('content-type') ?? '';
    let body: unknown = text;
    if (/\bapplication\/(?:[\w.-]+\+)?json\b/i.test(contentType) && text) {
      try { body = JSON.parse(text); } catch { body = text; }
    }
    return toolResult({ kind: response.ok ? 'http_response' : 'http_error', status: response.status, contentType, body }, !response.ok);
  } catch {
    return toolResult({ kind: 'transport_error', message: responseTooLarge ? 'Resposta excede o limite configurado' : controller.signal.aborted ? 'Timeout na chamada upstream' : 'Falha de conexão com a API upstream' }, true);
  } finally {
    clearTimeout(timer);
  }
}

export function createMcpServer(config: McpServerConfig, catalog: OpenApiCatalog): McpServer {
  validateConfig(config, catalog);
  const server = new McpServer({ name: 'building-operation', version: '0.1.0' });
  const validator = new AjvJsonSchemaValidator();
  for (const operation of catalog.operations) {
    const schema = fromJsonSchema<ToolArguments>(operation.inputSchema as JsonSchemaType, validator);
    server.registerTool(operation.toolName, {
      description: operation.description,
      inputSchema: schema,
      annotations: {
        ...(operation.method === 'GET' || operation.method === 'HEAD' ? { readOnlyHint: true } : {}),
        ...(operation.method === 'DELETE' ? { destructiveHint: true } : {}),
      },
      _meta: { operationId: operation.operationId, method: operation.method, path: operation.pathTemplate },
    }, async args => {
      const start = performance.now();
      const result = await executeOpenApiOperation(operation, args, config);
      const payload = result.structuredContent;
      const body = payload.body as { error?: { code?: unknown } } | undefined;
      const code = body?.error?.code ?? payload.code;
      config.onExecution?.({ id: randomUUID(), timestamp: new Date().toISOString(), toolName: operation.toolName, durationMs: Math.round(performance.now() - start), kind: String(payload.kind), ...(typeof payload.status === 'number' ? { status: payload.status } : {}), ...(typeof code === 'string' && /^[A-Z0-9_]{1,80}$/.test(code) ? { errorCode: code } : {}) });
      return result;
    });
  }
  return server;
}

export async function startMcpServer(config: McpServerConfig) {
  const catalog = await loadOpenApiOperations(config.openapiPath);
  validateConfig(config, catalog);
  const host = config.host ?? '127.0.0.1';
  if (!['127.0.0.1', 'localhost', '::1'].includes(host)) throw new Error('MCP_HOST deve ser loopback nesta POC local');
  const active = new Set<McpServer>();
  const executions: ToolExecution[] = [];
  let endpointUrl = '';
  const runtimeConfig: McpServerConfig = { ...config, onExecution: execution => {
    executions.push(execution);
    if (executions.length > 50) executions.shift();
    console.info('[MCP] ' + execution.toolName + ' → ' + (execution.status ?? execution.kind) + (execution.errorCode ? ' ' + execution.errorCode : '') + ' (' + execution.durationMs + ' ms)');
    config.onExecution?.(execution);
  } };
  const inspectorHtml = await readFile(new URL('./mcp-inspector.html', import.meta.url), 'utf8');
  const httpServer = createServer(async (request, response) => {
    const port = (httpServer.address() as { port: number }).port;
    const allowedHosts = ['127.0.0.1', 'localhost', '[::1]'].map(name => name + ':' + port);
    if (!allowedHosts.includes(request.headers.host ?? '') || (request.headers.origin && !allowedHosts.map(name => 'http://' + name).includes(request.headers.origin))) {
      response.writeHead(403).end('Host ou Origin não permitido');
      return;
    }
    response.setHeader('cache-control', 'no-store');
    if (request.method === 'GET' && request.url === '/') {
      response.writeHead(200, { 'content-type': 'text/html; charset=utf-8' }).end(inspectorHtml);
      return;
    }
    if (request.method === 'GET' && request.url === '/api/executions') {
      response.writeHead(200, { 'content-type': 'application/json' }).end(JSON.stringify({ executions }));
      return;
    }
    if (request.method === 'GET' && request.url === '/api/tools') {
      const client = new Client({ name: 'local-mcp-inspector', version: '1.0.0' });
      try {
        await client.connect(new StreamableHTTPClientTransport(new URL(endpointUrl)));
        const tools = [];
        let cursor: string | undefined;
        do {
          const page = await client.listTools(cursor ? { cursor } : undefined);
          tools.push(...page.tools);
          cursor = page.nextCursor;
        } while (cursor);
        response.writeHead(200, { 'content-type': 'application/json' }).end(JSON.stringify({ tools, mcpUrl: endpointUrl, upstreamBaseUrl: new URL(config.upstreamBaseUrl).origin + new URL(config.upstreamBaseUrl).pathname, discovered: catalog.discovered, excluded: catalog.excluded }));
      } catch {
        response.writeHead(502, { 'content-type': 'application/json' }).end(JSON.stringify({ error: 'Não foi possível consultar tools/list do MCP' }));
      } finally {
        await client.close();
      }
      return;
    }
    if (request.url !== '/mcp') {
      response.writeHead(404).end();
      return;
    }
    const server = createMcpServer(runtimeConfig, catalog);
    active.add(server);
    const transport = new NodeStreamableHTTPServerTransport({
      sessionIdGenerator: undefined,
      enableJsonResponse: true,
      enableDnsRebindingProtection: true,
      allowedHosts: ['127.0.0.1', 'localhost', '[::1]'].map(name => name + ':' + (httpServer.address() as { port: number }).port),
      allowedOrigins: ['127.0.0.1', 'localhost', '[::1]'].map(name => 'http://' + name + ':' + (httpServer.address() as { port: number }).port),
    });
    response.once('close', () => {
      active.delete(server);
      void server.close();
    });
    void server.connect(transport).then(() => transport.handleRequest(request, response)).catch(() => {
      if (!response.headersSent) response.writeHead(500, { 'content-type': 'application/json' }).end(JSON.stringify({ error: 'Falha no transporte MCP' }));
      else response.end();
      active.delete(server);
      void server.close();
    });
  });
  await new Promise<void>((resolve, reject) => {
    httpServer.once('error', reject);
    httpServer.listen(config.port ?? 3333, host, () => { httpServer.off('error', reject); resolve(); });
  });
  const address = httpServer.address();
  if (!address || typeof address === 'string') throw new Error('Endereço HTTP indisponível');
  const url = 'http://' + (host.includes(':') ? '[' + host + ']' : host) + ':' + address.port + '/mcp';
  endpointUrl = url;
  return {
    url,
    catalog,
    close: async () => {
      await Promise.all([...active].map(server => server.close()));
      await new Promise<void>((resolve, reject) => {
        httpServer.close(error => error ? reject(error) : resolve());
        httpServer.closeAllConnections();
      });
    },
  };
}

export function configFromEnv(env: NodeJS.ProcessEnv = process.env): McpServerConfig {
  const authMode = env.UPSTREAM_AUTH_MODE ?? 'bearer';
  if (!['bearer', 'client_credentials'].includes(authMode)) throw new Error('UPSTREAM_AUTH_MODE deve ser bearer ou client_credentials');
  return {
    openapiPath: env.OPENAPI_PATH ?? 'building-operation.json',
    upstreamBaseUrl: env.UPSTREAM_BASE_URL ?? '',
    upstreamBearerToken: env.UPSTREAM_BEARER_TOKEN,
    ...(authMode === 'client_credentials' ? { getUpstreamBearerToken: createBffTokenProvider({ keycloakUrl: env.BFF_KEYCLOAK_URL ?? '', clientId: env.BFF_KEYCLOAK_CLIENT_ID ?? '', clientSecret: env.BFF_KEYCLOAK_CLIENT_SECRET ?? '' }) } : {}),
    host: env.MCP_HOST ?? '127.0.0.1',
    port: Number(env.MCP_PORT ?? 3333),
    httpTimeoutMs: Number(env.HTTP_TIMEOUT_MS ?? 10_000),
    httpMaxResponseBytes: Number(env.HTTP_MAX_RESPONSE_BYTES ?? 1_048_576),
  };
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const config = configFromEnv();
  startMcpServer(config).then(server => {
    console.info('OpenAPI carregada: ' + config.openapiPath);
    console.info('Operations descobertas: ' + server.catalog.discovered);
    console.info('Operations excluídas por x-mcp: ' + server.catalog.excluded);
    console.info('MCP tools registradas: ' + server.catalog.operations.length);
    console.info('MCP endpoint: ' + server.url);
    console.info('Inspector visual: ' + new URL('/', server.url).href);
    console.info('Upstream base URL: ' + new URL(config.upstreamBaseUrl).origin + new URL(config.upstreamBaseUrl).pathname);
    const shutdown = () => { void server.close().then(() => process.exit(0)); };
    process.once('SIGINT', shutdown);
    process.once('SIGTERM', shutdown);
  }).catch(error => {
    console.error(error instanceof Error ? error.message : 'Falha ao iniciar MCP');
    process.exitCode = 1;
  });
}
