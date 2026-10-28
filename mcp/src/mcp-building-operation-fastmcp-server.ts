import { AsyncLocalStorage } from 'node:async_hooks';
import { createHash, randomUUID, timingSafeEqual } from 'node:crypto';
import { readFile } from 'node:fs/promises';
import { pathToFileURL } from 'node:url';
import { Client, StreamableHTTPClientTransport } from '@modelcontextprotocol/client';
import { AjvJsonSchemaValidator } from '@modelcontextprotocol/server/validators/ajv';
import { createOpenAPIServer, type AccessToken, type FastMCP, type TokenVerifier } from '@prefecthq/fastmcp-ts/server';
import { configFromEnv, type McpServerConfig, type ToolExecution } from './mcp-buildingi-operation-server.js';
import { isCredentialKey, tracePreview, type InspectorTrace } from './inspector-trace.js';

interface RouteMetadata {
  operationId: string;
  method: string;
  path: string;
  pathArguments: string[];
  argumentAliases: Map<string, string>;
  parameters: Record<string, { name: string; in: string }>;
  validate: (input: unknown) => { valid: boolean; errorMessage?: string };
}

interface ExecutionState {
  status?: number;
  errorCode?: string;
  kind: string;
  trace: InspectorTrace;
}

interface RuntimeContext {
  accessToken?: string;
  secrets: string[];
}

export type FastMcpAuthMode = 'bearer' | 'client_credentials' | 'request_bearer';

export interface FastMcpServerConfig extends McpServerConfig {
  upstreamAuthMode?: FastMcpAuthMode;
  requestTokenVerifier?: TokenVerifier;
  getCatalogBearerToken?: () => string | undefined;
}

function jwtExpiration(token: string): number | undefined {
  const parts = token.split('.');
  if (parts.length !== 3) return undefined;
  try {
    const payload = JSON.parse(Buffer.from(parts[1]!, 'base64url').toString('utf8')) as Record<string, unknown>;
    if (!Object.hasOwn(payload, 'exp')) return undefined;
    if (typeof payload.exp !== 'number' || !Number.isFinite(payload.exp)) throw new Error('Token inválido');
    return payload.exp;
  } catch {
    throw new Error('Token de teste inválido');
  }
}

export function createPinnedTokenVerifier(pinnedToken: string, now: () => number = Date.now): TokenVerifier {
  const normalized = pinnedToken.trim();
  if (!normalized) throw new Error('Configure B2C_CLIENT_KEYCLOAK_TOKEN para request_bearer');
  const pinnedDigest = createHash('sha256').update(normalized).digest();
  const expiresAt = jwtExpiration(normalized);
  return {
    async verify(token: string): Promise<AccessToken> {
      const suppliedDigest = createHash('sha256').update(token).digest();
      if (!timingSafeEqual(pinnedDigest, suppliedDigest) || (expiresAt !== undefined && expiresAt <= now() / 1000)) throw new Error('Unauthorized');
      return { token, clientId: 'poc-b2c-configured-token', scopes: [], ...(expiresAt === undefined ? {} : { expiresAt }), claims: {} };
    },
  };
}

function isLoopback(hostname: string): boolean {
  const host = hostname.toLowerCase().replace(/^\[|\]$/g, '');
  return host === 'localhost' || host === '::1' || /^127(?:\.\d{1,3}){3}$/.test(host);
}

function assertHttpsOutsideLoopback(url: URL, label: string): void {
  if (!isLoopback(url.hostname) && url.protocol !== 'https:') throw new Error(label + ' deve usar HTTPS fora de loopback');
}

function redactSecrets(value: string, secrets: string[]): string {
  let redacted = value;
  for (const secret of [...secrets].filter(Boolean).sort((left, right) => right.length - left.length)) redacted = redacted.replaceAll(secret, '[OCULTO]');
  return redacted;
}

export function fastMcpConfigFromEnv(env: NodeJS.ProcessEnv = process.env): FastMcpServerConfig {
  const authMode = env.UPSTREAM_AUTH_MODE ?? 'bearer';
  if (!['bearer', 'client_credentials', 'request_bearer'].includes(authMode)) throw new Error('UPSTREAM_AUTH_MODE deve ser bearer, client_credentials ou request_bearer');
  const requestBearer = authMode === 'request_bearer';
  const legacyConfig = configFromEnv({ ...env, UPSTREAM_AUTH_MODE: requestBearer ? 'bearer' : authMode, ...(requestBearer ? { UPSTREAM_BEARER_TOKEN: undefined } : {}) });
  if (!requestBearer) return { ...legacyConfig, upstreamAuthMode: authMode as FastMcpAuthMode, host: env.FASTMCP_HOST ?? env.MCP_HOST ?? '127.0.0.1', port: Number(env.FASTMCP_PORT ?? 3335) };
  const token = env.B2C_CLIENT_KEYCLOAK_TOKEN?.trim() ?? '';
  if (!token) throw new Error('Configure B2C_CLIENT_KEYCLOAK_TOKEN para request_bearer');
  return {
    ...legacyConfig,
    upstreamBearerToken: undefined,
    getUpstreamBearerToken: undefined,
    upstreamAuthMode: 'request_bearer',
    requestTokenVerifier: createPinnedTokenVerifier(token),
    getCatalogBearerToken: () => token,
    host: env.FASTMCP_HOST ?? env.MCP_HOST ?? '127.0.0.1',
    port: Number(env.FASTMCP_PORT ?? 3335),
  };
}

export class BuildingOperationFastMcpServer {
  private server?: FastMCP;
  private readonly executions: ToolExecution[] = [];
  private readonly traces = new Map<string, InspectorTrace>();
  private readonly execution = new AsyncLocalStorage<ExecutionState>();
  private readonly runtime = new AsyncLocalStorage<RuntimeContext>();
  private readonly routes = new Map<string, RouteMetadata>();
  private discovered = 0;
  private excluded = 0;

  constructor(private readonly config: FastMcpServerConfig) {}

  get url(): string {
    const address = this.server?.address;
    if (!address) throw new Error('O servidor FastMCP não está em execução');
    return 'http://' + (address.host.includes(':') ? '[' + address.host + ']' : address.host) + ':' + address.port + '/mcp';
  }

  async start(): Promise<this> {
    if (this.server) throw new Error('O servidor FastMCP já foi iniciado');
    this.routes.clear();
    this.discovered = 0;
    this.excluded = 0;
    const base = new URL(this.config.upstreamBaseUrl);
    if (!['http:', 'https:'].includes(base.protocol) || base.username || base.password || base.search || base.hash) throw new Error('UPSTREAM_BASE_URL inválida');
    if (this.config.upstreamAuthMode === 'request_bearer') {
      if (!this.config.requestTokenVerifier) throw new Error('Configure o verifier de request_bearer');
      assertHttpsOutsideLoopback(base, 'UPSTREAM_BASE_URL');
    } else if (!this.config.getUpstreamBearerToken && !this.config.upstreamBearerToken?.trim()) throw new Error('Configure a autenticação upstream');
    const host = this.config.host ?? '127.0.0.1';
    if (!['127.0.0.1', 'localhost', '::1'].includes(host)) throw new Error('FASTMCP_HOST deve ser loopback nesta POC local');
    const port = this.config.port ?? 3335;
    if (!Number.isInteger(port) || port < 0 || port > 65535) throw new Error('FASTMCP_PORT inválida');
    for (const value of [this.config.httpTimeoutMs ?? 10000, this.config.httpMaxResponseBytes ?? 1048576]) {
      if (!Number.isSafeInteger(value) || value <= 0) throw new Error('Limites HTTP devem ser inteiros positivos');
    }
    const spec = JSON.parse(await readFile(this.config.openapiPath, 'utf8'));
    const exposure = new Map<string, boolean>();
    const names = new Set<string>();
    for (const [path, item] of Object.entries(spec.paths as Record<string, Record<string, any>>)) {
      for (const [method, operation] of Object.entries(item)) {
        if (!['get', 'post', 'put', 'patch', 'delete', 'head', 'options', 'trace'].includes(method)) continue;
        const enabled = operation['x-mcp'] ?? item['x-mcp'] ?? spec['x-mcp'] ?? true;
        if (typeof enabled !== 'boolean') throw new Error('x-mcp deve ser boolean');
        exposure.set(method.toUpperCase() + ' ' + path, enabled);
        this.discovered++;
        if (!enabled) { this.excluded++; continue; }
        if (typeof operation.operationId !== 'string' || !/^[a-zA-Z0-9_.-]{1,128}$/.test(operation.operationId) || names.has(operation.operationId)) throw new Error('operationId inválido ou duplicado');
        names.add(operation.operationId);
        for (const parameter of [...(item.parameters ?? []), ...(operation.parameters ?? [])]) {
          if (parameter.in === 'header' && /^(authorization|proxy-authorization|host|cookie|set-cookie|connection|content-length|transfer-encoding|upgrade|expect|forwarded|x-forwarded-.+)$/i.test(parameter.name)) throw new Error('Header reservado não pode ser argumento de tool');
        }
      }
    }
    const validator = new AjvJsonSchemaValidator();
    const server = createOpenAPIServer({
      spec,
      name: 'building-operation-fastmcp',
      version: '0.0.1',
      validateOutput: false,
      client: {
        baseUrl: base.href.replace(/\/$/, ''),
        timeoutMs: this.config.httpTimeoutMs ?? 10000,
        auth: { getHeaders: () => this.authHeaders() },
        fetch: (input, init) => this.fetchUpstream(input, init),
      },
      routeMapFn: route => exposure.get(route.method + ' ' + route.path) === false ? 'exclude' : 'tool',
      componentFn: (route, component) => {
        if (component.kind !== 'tool') return;
        component.name = route.operationId!;
        const properties = component.inputSchema.properties as Record<string, unknown> ?? {};
        const usedNames = new Set(Object.keys(properties).filter(name => /^[a-zA-Z0-9_.-]{1,64}$/.test(name)));
        const aliases = new Map<string, string>();
        const renamed = new Map<string, string>();
        const normalizedProperties = Object.entries(properties).map(([name, schema]) => {
          if (/^[a-zA-Z0-9_.-]{1,64}$/.test(name)) return [name, schema];
          const baseName = name.replace(/[^a-zA-Z0-9_]+/g, '_').replace(/^_+|_+$/g, '').slice(0, 56) || 'argument';
          let alias = baseName;
          for (let suffix = 2; usedNames.has(alias); suffix++) alias = baseName + '_' + suffix;
          usedNames.add(alias);
          aliases.set(alias, name);
          renamed.set(name, alias);
          return [alias, schema];
        });
        component.inputSchema = { ...component.inputSchema, properties: Object.fromEntries(normalizedProperties), additionalProperties: false, ...(Array.isArray(component.inputSchema.required) ? { required: component.inputSchema.required.map(name => renamed.get(String(name)) ?? name) } : {}) };
        this.routes.set(component.name, {
          operationId: component.name, method: route.method, path: route.path,
          pathArguments: Object.entries(route.parameterMap).filter(([, mapping]) => mapping.location === 'path').map(([name]) => renamed.get(name) ?? name),
          argumentAliases: aliases,
          parameters: Object.fromEntries(Object.entries(route.parameterMap).map(([name, mapping]) => [renamed.get(name) ?? name, { name: mapping.openapiName, in: mapping.location }])),
          validate: validator.getValidator(component.inputSchema),
        });
      },
      serverOptions: { http: { cors: false }, dnsRebinding: { enabled: true }, ...(this.config.upstreamAuthMode === 'request_bearer' ? { auth: this.config.requestTokenVerifier } : {}) },
    });
    if (this.routes.size !== names.size) { await server.close(); throw new Error('O catálogo gerado pelo FastMCP está incompleto'); }
    server.use({
      onListTools: async (_context, next) => {
        const result = await next() as { tools: Array<{ name: string; _meta?: Record<string, unknown>; annotations?: Record<string, unknown> }> };
        for (const tool of result.tools) {
          const route = this.routes.get(tool.name);
          if (!route) continue;
          tool._meta = { ...tool._meta, operationId: route.operationId, method: route.method, path: route.path, parameters: route.parameters, implementation: 'fastmcp-ts' };
          tool.annotations = { ...tool.annotations, ...(['GET', 'HEAD'].includes(route.method) ? { readOnlyHint: true } : {}), ...(route.method === 'DELETE' ? { destructiveHint: true } : {}) };
        }
        return result;
      },
      onCallTool: async (context, next) => {
        const request = context.request as { name: string; arguments?: Record<string, unknown> };
        const accessToken = this.config.upstreamAuthMode === 'request_bearer' ? context.mcpContext.auth?.token : undefined;
        const runtime: RuntimeContext = { accessToken, secrets: accessToken ? [accessToken] : [] };
        const state: ExecutionState = { kind: 'tool_error', trace: {} };
        const started = performance.now();
        return this.runtime.run(runtime, () => this.execution.run(state, async () => {
          try {
            const route = this.routes.get(request.name);
            const args = request.arguments ?? {};
            state.trace.arguments = tracePreview(args, runtime.secrets);
            if (route) state.trace.route = { method: route.method, path: route.path };
            const validation = route?.validate(args);
            if (!route || !validation?.valid || route.pathArguments.some(name => ['.', '..'].includes(String(args[name])))) {
              state.kind = 'argument_error';
              const result = { isError: true, content: [{ type: 'text', text: 'Argumentos inválidos: ' + (validation?.errorMessage ?? 'parâmetro de path inseguro') }] };
              state.trace.result = tracePreview(result, runtime.secrets);
              return result;
            }
            for (const [alias, original] of route.argumentAliases) {
              if (!Object.hasOwn(args, alias)) continue;
              args[original] = args[alias];
              delete args[alias];
            }
            const result = await next() as { isError?: boolean };
            state.trace.result = tracePreview(result, runtime.secrets);
            if (result.isError && state.kind === 'http_response') state.kind = 'tool_error';
            return result;
          } catch (error) {
            state.trace.error = state.errorCode ?? 'TOOL_EXECUTION_ERROR';
            throw error;
          } finally {
            const entry: ToolExecution = { id: randomUUID(), timestamp: new Date().toISOString(), toolName: request.name, durationMs: Math.round(performance.now() - started), kind: state.kind, ...(typeof state.status === 'number' ? { status: state.status } : {}), ...(state.errorCode ? { errorCode: state.errorCode } : {}) };
            this.executions.push(entry);
            this.traces.set(entry.id, state.trace);
            if (this.executions.length > 50) this.traces.delete(this.executions.shift()!.id);
            console.info('[FastMCP] ' + entry.toolName + ' → ' + (entry.status ?? entry.kind) + (entry.errorCode ? ' ' + entry.errorCode : '') + ' (' + entry.durationMs + ' ms)');
            this.config.onExecution?.(entry);
          }
        }));
      },
    });
    const html = await readFile(new URL('./mcp-inspector.html', import.meta.url), 'utf8');
    for (const path of ['/', '/api/tools', '/api/executions', '/api/execution']) {
      server.customRoute({ path, methods: ['GET'] }, async request => {
        const allowed = ['127.0.0.1', 'localhost', '[::1]'].map(name => 'http://' + name + ':' + server.address!.port);
        const origin = new URL(request.url).origin;
        if (!allowed.includes(origin) || (request.headers.has('origin') && !allowed.includes(request.headers.get('origin')!))) return new Response('Host ou Origin não permitido', { status: 403 });
        const headers = { 'cache-control': 'no-store' };
        if (path === '/') return new Response(html, { headers: { ...headers, 'content-type': 'text/html; charset=utf-8' } });
        if (path === '/api/executions') return Response.json({ executions: this.executions }, { headers });
        if (path === '/api/execution') {
          const id = new URL(request.url).searchParams.get('id') ?? '';
          const trace = this.traces.get(id);
          return trace ? Response.json({ execution: this.executions.find(entry => entry.id === id), trace }, { headers }) : Response.json({ error: 'Chamada não encontrada ou removida do histórico.' }, { status: 404, headers });
        }
        const client = new Client({ name: 'fastmcp-local-inspector', version: '1' });
        try {
          const catalogToken = this.config.upstreamAuthMode === 'request_bearer' ? this.config.getCatalogBearerToken?.() : undefined;
          const transport = new StreamableHTTPClientTransport(new URL(this.url), catalogToken ? { requestInit: { headers: { authorization: 'Bearer ' + catalogToken } } } : undefined);
          await client.connect(transport);
          const tools = [];
          let cursor: string | undefined;
          do {
            const page = await client.listTools(cursor ? { cursor } : undefined);
            tools.push(...page.tools);
            cursor = page.nextCursor;
          } while (cursor);
          return Response.json({ tools, mcpUrl: this.url, upstreamBaseUrl: base.origin + base.pathname, discovered: this.discovered, excluded: this.excluded, implementation: 'fastmcp-ts' }, { headers });
        } catch {
          return Response.json({ error: 'Não foi possível consultar tools/list do FastMCP' }, { status: 502, headers });
        } finally { await client.close(); }
      });
    }
    this.server = server;
    try { await server.run({ transport: 'http', host, port, path: '/mcp', stateless: true }); }
    catch (error) { await this.close(); throw error; }
    return this;
  }

  async close(): Promise<void> {
    await this.server?.close();
    this.server = undefined;
  }

  private async authHeaders(): Promise<Record<string, string>> {
    try {
      const runtime = this.runtime.getStore();
      const token = this.config.upstreamAuthMode === 'request_bearer'
        ? runtime?.accessToken
        : this.config.getUpstreamBearerToken ? await this.config.getUpstreamBearerToken() : this.config.upstreamBearerToken;
      if (!token?.trim()) throw new Error(this.config.upstreamAuthMode === 'request_bearer' ? 'REQUEST_AUTH_CONTEXT_MISSING' : 'BFF_AUTH_TOKEN_MISSING');
      runtime?.secrets.push(token);
      return { authorization: 'Bearer ' + token };
    } catch (error) {
      const code = error instanceof Error && /^(?:BFF|REQUEST)_AUTH_[A-Z0-9_]+$/.test(error.message) ? error.message : 'BFF_AUTH_UNAVAILABLE';
      const state = this.execution.getStore();
      if (state) { state.kind = 'auth_error'; state.errorCode = code; }
      throw new Error(code);
    }
  }

  private async fetchUpstream(input: Parameters<typeof fetch>[0], init?: RequestInit): Promise<Response> {
    const state = this.execution.getStore();
    const secrets = this.runtime.getStore()?.secrets ?? [];
    try {
      if (state) {
        const url = new URL(input instanceof Request ? input.url : String(input));
        const safeUrl = new URL(url);
        for (const key of safeUrl.searchParams.keys()) if (isCredentialKey(key)) safeUrl.searchParams.set(key, '[OCULTO]');
        const headers = new Headers(init?.headers ?? (input instanceof Request ? input.headers : undefined));
        state.trace.request = {
          method: init?.method ?? (input instanceof Request ? input.method : 'GET'),
          url: tracePreview(safeUrl.href, secrets).value as string,
          path: tracePreview(url.pathname, secrets).value as string,
          query: tracePreview(Object.fromEntries(url.searchParams), secrets),
          headers: tracePreview(Object.fromEntries(['content-type', 'accept'].filter(name => headers.has(name)).map(name => [name, headers.get(name)!])), secrets).value as Record<string, string>,
          body: tracePreview(typeof init?.body === 'string' ? init.body : null, secrets),
        };
      }
      const response = await fetch(input, { ...init, redirect: 'manual' });
      if (state) { state.status = response.status; state.kind = response.ok ? 'http_response' : 'http_error'; }
      const contentType = redactSecrets(response.headers.get('content-type') ?? 'text/plain', secrets);
      const statusText = redactSecrets(response.statusText, secrets);
      if (state) state.trace.response = { status: response.status, headers: tracePreview({ 'content-type': contentType }, secrets).value as Record<string, string> };
      const maxBytes = this.config.httpMaxResponseBytes ?? 1048576;
      const chunks: Uint8Array[] = [];
      let size = 0;
      if (Number(response.headers.get('content-length')) > maxBytes) { await response.body?.cancel(); throw new Error('UPSTREAM_RESPONSE_TOO_LARGE'); }
      if (response.body) {
        for await (const chunk of response.body) {
          size += chunk.byteLength;
          if (size > maxBytes) throw new Error('UPSTREAM_RESPONSE_TOO_LARGE');
          chunks.push(chunk);
        }
      }
      const rawBody = Buffer.concat(chunks).toString('utf8');
      const body = Buffer.from(redactSecrets(rawBody, secrets));
      if (state?.trace.response) state.trace.response.body = tracePreview(body.toString('utf8'), secrets);
      if (!response.ok && state) {
        try {
          const payload = JSON.parse(body.toString('utf8'));
          const code = payload?.error?.code;
          if (typeof code === 'string' && /^[A-Z0-9_]{1,80}$/.test(code)) state.errorCode = code;
        } catch {}
      }
      return new Response([204, 205, 304].includes(response.status) ? null : body, { status: response.status, statusText, headers: { 'content-type': contentType } });
    } catch (error) {
      if (state) {
        state.kind = 'transport_error';
        state.errorCode = error instanceof Error && error.message === 'UPSTREAM_RESPONSE_TOO_LARGE' ? error.message : 'UPSTREAM_CONNECTION_ERROR';
        state.trace.error = state.errorCode;
      }
      throw new Error(state?.errorCode ?? 'UPSTREAM_CONNECTION_ERROR');
    }
  }
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const server = new BuildingOperationFastMcpServer(fastMcpConfigFromEnv());
  server.start().then(() => {
    console.info('FastMCP endpoint: ' + server.url);
    console.info('Inspector visual: ' + new URL('/', server.url).href);
    const shutdown = () => { void server.close().then(() => process.exit(0)); };
    process.once('SIGINT', shutdown);
    process.once('SIGTERM', shutdown);
  }).catch(error => {
    console.error(error instanceof Error ? error.message : 'Falha ao iniciar FastMCP');
    process.exitCode = 1;
  });
}
