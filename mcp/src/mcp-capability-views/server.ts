import { AsyncLocalStorage } from 'node:async_hooks';
import { once } from 'node:events';
import { readFile } from 'node:fs/promises';
import { createServer as createHttpServer, type IncomingMessage, type Server as NodeHttpServer, type ServerResponse } from 'node:http';
import { createHash, randomUUID } from 'node:crypto';
import { AjvJsonSchemaValidator } from '@modelcontextprotocol/server/validators/ajv';
import { createOpenAPIServer, FastMCP, type TokenVerifier } from '@prefecthq/fastmcp-ts/server';
import { fastMcpConfigFromEnv } from '../mcp-building-operation-fastmcp-server.js';
import { isCredentialKey, tracePreview, type InspectorTrace } from '../inspector-trace.js';

export interface CapabilityHierarchy {
  domain: string;
  service?: string;
  module?: string;
}

export interface CapabilitySourceConfig {
  id: string;
  hierarchy: CapabilityHierarchy;
  openapiPath: string;
  upstreamBaseUrl: string;
  toolNamePrefix?: string;
  headerEnv?: Record<string, string>;
}

export interface CapabilitySourceIdentity {
  id: string;
  hierarchy: CapabilityHierarchy;
}

export interface CapabilitySkillResource {
  uri: string;
  name: string;
  description: string;
  mimeType: string;
  hierarchy: CapabilityHierarchy;
  metadata: Record<string, unknown>;
  read(): Promise<string>;
}

export interface CapabilityToolDefinition {
  name: string;
  description?: string;
  inputSchema: Record<string, unknown>;
  outputSchema?: Record<string, unknown>;
}

export interface Capability {
  source: CapabilitySourceIdentity;
  operationId: string;
  toolName: string;
  definition?: CapabilityToolDefinition;
  execution: { method: string; path: string };
}

export interface CapabilityRegistry {
  sources: CapabilitySourceIdentity[];
  capabilities: Capability[];
  excluded: number;
  excludedBySource: Record<string, number>;
}

interface LoadedSource {
  config: CapabilitySourceConfig;
  identity: CapabilitySourceIdentity;
  spec: Record<string, any>;
  baseUrl: URL;
  headers: Record<string, string>;
  exposure: Map<string, boolean>;
  capabilities: Capability[];
}

interface RouteMetadata {
  capability: Capability;
  pathArguments: string[];
  argumentAliases: Map<string, string>;
  parameters: Record<string, { name: string; in: string }>;
  upstreamHeaders: Record<string, string>;
  missingRequiredHeaders: string[];
  validate: (input: unknown) => { valid: boolean; errorMessage?: string };
}

interface RuntimeContext {
  accessToken: string;
  secrets: string[];
  signal: AbortSignal;
  viewPath: string;
  upstreamHeaders?: Record<string, string>;
}

interface ExecutionState {
  status?: number;
  errorCode?: string;
  kind: string;
  trace: InspectorTrace;
}

interface CapabilityExecution {
  id: string;
  timestamp: string;
  view: string;
  sourceId: string;
  toolName: string;
  durationMs: number;
  kind: string;
  status?: number;
  errorCode?: string;
}

export interface CapabilityViewsServerConfig {
  sources: CapabilitySourceConfig[];
  skillResources?: CapabilitySkillResource[];
  requestTokenVerifier: TokenVerifier;
  toolNameBuilder?: (input: { source: CapabilitySourceIdentity; operationId: string; method: string; path: string; duplicateOperationId: boolean }) => string;
  host?: string;
  port?: number;
  httpTimeoutMs?: number;
  httpMaxResponseBytes?: number;
  maxRequestBodyBytes?: number;
  resolvedSourceHeaders?: Record<string, Record<string, string>>;
}

function createRequestBearerVerifier(): TokenVerifier {
  return {
    async verify(token) {
      if (!/^[A-Za-z0-9._~+/-]+=*$/.test(token)) throw new Error('Bearer inválido');
      return { token, clientId: 'request-bearer', scopes: [], claims: {} };
    },
  };
}

export function capabilityViewsConfigFromEnv(env: NodeJS.ProcessEnv = process.env): CapabilityViewsServerConfig {
  const host = env.MCP_VIEWS_HOST ?? '127.0.0.1';
  const port = Number(env.MCP_VIEWS_PORT ?? 3336);
  const fastMcpConfig = fastMcpConfigFromEnv({
    ...env,
    UPSTREAM_AUTH_MODE: 'bearer',
    FASTMCP_HOST: host,
    FASTMCP_PORT: String(port),
  });
  let sources: CapabilitySourceConfig[];
  const sourcesJson = env.MCP_VIEWS_SOURCES_JSON;
  if (sourcesJson?.trim()) {
    let parsed: unknown;
    try { parsed = JSON.parse(sourcesJson); } catch { throw new Error('MCP_VIEWS_SOURCES_JSON deve ser um JSON válido'); }
    if (!Array.isArray(parsed)) throw new Error('MCP_VIEWS_SOURCES_JSON deve conter uma lista de sources');
    sources = parsed as CapabilitySourceConfig[];
  } else {
    const buildingOperationBaseUrl = env.MCP_VIEWS_UPSTREAM_BASE_URL?.trim() || fastMcpConfig.upstreamBaseUrl;
    sources = [
      {
        id: 'bff-building-operation',
        hierarchy: { domain: 'gruvi', service: 'bff', module: 'building-operation' },
        openapiPath: env.MCP_VIEWS_OPENAPI_PATH?.trim() || fastMcpConfig.openapiPath,
        upstreamBaseUrl: buildingOperationBaseUrl,
      },
      {
        id: 'bff-mobile',
        hierarchy: { domain: 'gruvi', service: 'bff', module: 'mobile' },
        openapiPath: env.MCP_VIEWS_MOBILE_OPENAPI_PATH?.trim() || 'mobile.json',
        upstreamBaseUrl: env.MCP_VIEWS_MOBILE_UPSTREAM_BASE_URL?.trim() || buildingOperationBaseUrl,
        headerEnv: { 'x-app-check-token': 'MCP_VIEWS_MOBILE_APP_CHECK_TOKEN' },
      },
    ];
  }
  const resolvedSourceHeaders = Object.fromEntries(sources.map(rawSource => {
    const source = assertSource(rawSource);
    return [source.id, Object.fromEntries(Object.entries(source.headerEnv ?? {}).flatMap(([header, envName]) => {
      const value = env[envName]?.trim();
      return value ? [[header.toLowerCase(), value]] : [];
    }))];
  }));
  return {
    sources,
    requestTokenVerifier: createRequestBearerVerifier(),
    resolvedSourceHeaders,
    host,
    port,
    httpTimeoutMs: fastMcpConfig.httpTimeoutMs,
    httpMaxResponseBytes: fastMcpConfig.httpMaxResponseBytes,
    maxRequestBodyBytes: Number(env.MCP_VIEWS_MAX_REQUEST_BYTES ?? 4194304),
  };
}

function isLoopback(hostname: string): boolean {
  const host = hostname.toLowerCase().replace(/^\[|\]$/g, '');
  return host === 'localhost' || host === '::1' || /^127(?:\.\d{1,3}){3}$/.test(host);
}

function assertSource(source: CapabilitySourceConfig): CapabilitySourceConfig {
  if (!source || typeof source !== 'object' || Array.isArray(source)) throw new Error('Configuração de source inválida');
  if (!/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(source.id)) throw new Error('ID de source inválido');
  if (!source.hierarchy || typeof source.hierarchy !== 'object') throw new Error('Hierarquia ausente na source ' + source.id);
  const hierarchy = source.hierarchy;
  if (typeof hierarchy.domain !== 'string' || !/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(hierarchy.domain)) throw new Error('Hierarquia inválida na source ' + source.id);
  const segments = [hierarchy.domain, hierarchy.service, hierarchy.module].filter((value): value is string => value !== undefined);
  if (!segments.length || hierarchy.service === undefined && hierarchy.module !== undefined) throw new Error('Hierarquia inválida na source ' + source.id);
  if (segments.some(segment => typeof segment !== 'string' || !/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(segment))) throw new Error('Hierarquia inválida na source ' + source.id);
  if (typeof source.openapiPath !== 'string' || !source.openapiPath.trim()) throw new Error('OpenAPI ausente na source ' + source.id);
  const baseUrl = new URL(source.upstreamBaseUrl);
  if (!['http:', 'https:'].includes(baseUrl.protocol) || baseUrl.username || baseUrl.password || baseUrl.search || baseUrl.hash) throw new Error('UPSTREAM_BASE_URL inválida na source ' + source.id);
  if (!isLoopback(baseUrl.hostname) && baseUrl.protocol !== 'https:') throw new Error('UPSTREAM_BASE_URL deve usar HTTPS fora de loopback na source ' + source.id);
  if (source.toolNamePrefix !== undefined && !/^[a-zA-Z0-9_.-]*$/.test(source.toolNamePrefix)) throw new Error('Prefixo de tool inválido na source ' + source.id);
  if (source.headerEnv !== undefined) {
    if (!source.headerEnv || typeof source.headerEnv !== 'object' || Array.isArray(source.headerEnv)) throw new Error('headerEnv inválido na source ' + source.id);
    for (const [header, envName] of Object.entries(source.headerEnv)) {
      if (!/^[a-zA-Z0-9-]+$/.test(header) || /^(authorization|proxy-authorization|host|cookie|set-cookie|connection|content-length|transfer-encoding|upgrade|expect|forwarded|x-forwarded-.+)$/i.test(header)) throw new Error('Header injetado inválido na source ' + source.id);
      if (typeof envName !== 'string' || !/^[A-Z_][A-Z0-9_]*$/.test(envName)) throw new Error('Nome de variável inválido em headerEnv na source ' + source.id);
    }
  }
  return source;
}

function identityOf(source: CapabilitySourceConfig): CapabilitySourceIdentity {
  return {
    id: source.id,
    hierarchy: {
      domain: source.hierarchy.domain,
      ...(source.hierarchy.service === undefined ? {} : { service: source.hierarchy.service }),
      ...(source.hierarchy.module === undefined ? {} : { module: source.hierarchy.module }),
    },
  };
}

function operationMethods(item: Record<string, any>): Array<[string, Record<string, any>]> {
  return Object.entries(item).filter(([method, operation]) => ['get', 'post', 'put', 'patch', 'delete', 'head', 'options', 'trace'].includes(method.toLowerCase()) && operation && typeof operation === 'object') as Array<[string, Record<string, any>]>;
}

function enabledFor(spec: Record<string, any>, item: Record<string, any>, operation: Record<string, any>): boolean {
  const enabled = operation['x-mcp'] ?? item['x-mcp'] ?? spec['x-mcp'] ?? true;
  if (typeof enabled !== 'boolean') throw new Error('x-mcp deve ser boolean');
  return enabled;
}

function normalizeToolNamePart(value: string): string {
  return value.replace(/\[(\d+)\]/g, '_$1').replace(/[^a-zA-Z0-9_.-]/g, '_');
}

function routeNameSuffix(method: string, path: string): string {
  const normalizedMethod = method.toLowerCase();
  const hash = createHash('sha256').update(method.toUpperCase() + ' ' + path).digest('hex').slice(0, 8);
  const lastLiteral = path.split('/').filter(Boolean).reverse().find(segment => !/^\{[^{}]+\}$/.test(segment));
  const slug = normalizeToolNamePart(lastLiteral?.replace(/[{}]/g, '') || 'route');
  const maxSlugLength = 64 - normalizedMethod.length - hash.length - 2;
  return normalizedMethod + '.' + hash + '.' + slug.slice(-maxSlugLength);
}

function createToolName(prefix: string, operationId: string, suffix?: string): string {
  const base = normalizeToolNamePart(operationId) || 'tool';
  const tail = suffix ? '.' + normalizeToolNamePart(suffix).slice(0, 64) : '';
  const baseLength = 128 - prefix.length - tail.length;
  if (baseLength < 1) throw new Error('Prefixo de tool excede o limite MCP');
  const name = prefix + base.slice(0, baseLength) + tail;
  if (!/^[a-zA-Z0-9_.-]{1,128}$/.test(name)) throw new Error('Nome de tool MCP inválido: ' + name);
  return name;
}

function normalizeSchemaTypes(schema: unknown): void {
  if (!schema || typeof schema !== 'object') return;
  if (Array.isArray(schema)) {
    for (const item of schema) normalizeSchemaTypes(item);
    return;
  }
  const record = schema as Record<string, any>;
  if (record.type === 'uuid') {
    record.type = 'string';
    record.format = 'uuid';
  }
  for (const key of ['properties', 'patternProperties', '$defs', 'definitions', 'dependencies', 'dependentSchemas']) {
    const children = record[key];
    if (children && typeof children === 'object') for (const child of Object.values(children)) normalizeSchemaTypes(child);
  }
  for (const key of ['items', 'prefixItems', 'additionalProperties', 'allOf', 'anyOf', 'oneOf', 'not', 'if', 'then', 'else']) {
    if (record[key] && typeof record[key] === 'object') normalizeSchemaTypes(record[key]);
  }
}

function configuredHeaderEnv(source: CapabilitySourceConfig, headerName: string): string | undefined {
  return Object.entries(source.headerEnv ?? {}).find(([header]) => header.toLowerCase() === headerName.toLowerCase())?.[1];
}

async function loadRegistry(sourceConfigs: CapabilitySourceConfig[], resolvedSourceHeaders: Record<string, Record<string, string>> = {}, toolNameBuilder?: CapabilityViewsServerConfig['toolNameBuilder']): Promise<{ registry: CapabilityRegistry; loaded: LoadedSource[] }> {
  if (!sourceConfigs.length) throw new Error('Registre ao menos uma OpenAPI source');
  const identities = new Set<string>();
  const globalNames = new Map<string, string>();
  const loaded: LoadedSource[] = [];
  const registry: CapabilityRegistry = { sources: [], capabilities: [], excluded: 0, excludedBySource: {} };
  for (const rawConfig of sourceConfigs) {
    const config = assertSource(rawConfig);
    if (identities.has(config.id)) throw new Error('ID de source duplicado: ' + config.id);
    identities.add(config.id);
    const identity = identityOf(config);
    const spec = JSON.parse(await readFile(config.openapiPath, 'utf8')) as Record<string, any>;
    if (!spec.paths || typeof spec.paths !== 'object') throw new Error('OpenAPI sem paths na source ' + config.id);
    const exposure = new Map<string, boolean>();
    const candidates: Array<{ operationId: string; method: string; path: string }> = [];
    const capabilities: Capability[] = [];
    for (const [path, rawItem] of Object.entries(spec.paths as Record<string, Record<string, any>>)) {
      const item = rawItem as Record<string, any>;
      for (const [method, operation] of operationMethods(item)) {
        const enabled = enabledFor(spec, item, operation);
        const key = method.toUpperCase() + ' ' + path;
        exposure.set(key, enabled);
        if (!enabled) {
          registry.excluded++;
          registry.excludedBySource[config.id] = (registry.excludedBySource[config.id] ?? 0) + 1;
          continue;
        }
        const operationId = operation.operationId;
        if (typeof operationId !== 'string' || !operationId.trim()) throw new Error('operationId ausente na source ' + config.id);
        candidates.push({ operationId, method: method.toUpperCase(), path });
      }
    }
    const prefix = config.toolNamePrefix ?? hierarchySegments(identity.hierarchy).join('.') + '.';
    const groupedCandidates = new Map<string, Array<{ operationId: string; method: string; path: string }>>();
    for (const candidate of candidates) {
      const baseName = toolNameBuilder ? candidate.operationId : normalizeToolNamePart(candidate.operationId) || 'tool';
      const group = groupedCandidates.get(baseName) ?? [];
      group.push(candidate);
      groupedCandidates.set(baseName, group);
    }
    for (const candidate of candidates) {
        const baseName = toolNameBuilder ? candidate.operationId : normalizeToolNamePart(candidate.operationId) || 'tool';
        const group = groupedCandidates.get(baseName)!;
        const suffix = group.length > 1 ? routeNameSuffix(candidate.method, candidate.path) : undefined;
        const toolName = toolNameBuilder
          ? toolNameBuilder({ source: identity, ...candidate, duplicateOperationId: group.length > 1 })
          : createToolName(prefix, candidate.operationId, suffix);
        const previousSource = globalNames.get(toolName);
        if (globalNames.has(toolName)) throw new Error('Colisão de tool MCP "' + toolName + '" entre as sources ' + previousSource + ' e ' + config.id + '; configure toolNamePrefix explicitamente');
        globalNames.set(toolName, config.id);
        const capability: Capability = {
          source: identity,
          operationId: candidate.operationId,
          toolName,
          execution: { method: candidate.method, path: candidate.path },
        };
        capabilities.push(capability);
        registry.capabilities.push(capability);
    }
    registry.sources.push(identity);
    const headers = Object.fromEntries(Object.entries(resolvedSourceHeaders[config.id] ?? {}).map(([name, value]) => [name.toLowerCase(), value]));
    loaded.push({ config, identity, spec, baseUrl: new URL(config.upstreamBaseUrl), headers, exposure, capabilities });
  }
  return { registry, loaded };
}

function aliasesFor(inputSchema: Record<string, any>): { inputSchema: Record<string, any>; aliases: Map<string, string>; renamed: Map<string, string> } {
  const properties = inputSchema.properties as Record<string, unknown> ?? {};
  const used = new Set(Object.keys(properties).filter(name => /^[a-zA-Z0-9_.-]{1,64}$/.test(name)));
  const aliases = new Map<string, string>();
  const renamed = new Map<string, string>();
  const normalized = Object.entries(properties).map(([name, schema]) => {
    if (/^[a-zA-Z0-9_.-]{1,64}$/.test(name)) return [name, schema];
    const base = name.replace(/[^a-zA-Z0-9_]+/g, '_').replace(/^_+|_+$/g, '').slice(0, 56) || 'argument';
    let alias = base;
    for (let suffix = 2; used.has(alias); suffix++) alias = base + '_' + suffix;
    used.add(alias);
    aliases.set(alias, name);
    renamed.set(name, alias);
    return [alias, schema];
  });
  return {
    inputSchema: {
      ...inputSchema,
      properties: Object.fromEntries(normalized),
      additionalProperties: false,
      ...(Array.isArray(inputSchema.required) ? { required: inputSchema.required.map((name: unknown) => renamed.get(String(name)) ?? name) } : {}),
    },
    aliases,
    renamed,
  };
}

function redactSecrets(value: string, secrets: string[]): string {
  return [...secrets].filter(Boolean).sort((left, right) => right.length - left.length).reduce((text, secret) => text.replaceAll(secret, '[OCULTO]'), value);
}

function targetUrl(input: Parameters<typeof fetch>[0], baseUrl: URL): URL {
  const target = new URL(input instanceof Request ? input.url : String(input), baseUrl);
  const basePath = baseUrl.pathname.replace(/\/$/, '');
  if (target.origin !== baseUrl.origin || !(target.pathname === basePath || target.pathname.startsWith(basePath + '/'))) throw new Error('UPSTREAM_TARGET_INVALID');
  return target;
}

async function fetchUpstream(input: Parameters<typeof fetch>[0], init: RequestInit | undefined, source: LoadedSource, runtime: AsyncLocalStorage<RuntimeContext>, execution: AsyncLocalStorage<ExecutionState>, timeoutMs: number, maxBytes: number): Promise<Response> {
  const request = runtime.getStore();
  if (!request?.accessToken) throw new Error('REQUEST_AUTH_CONTEXT_MISSING');
  const url = targetUrl(input, source.baseUrl);
  const timeoutSignal = AbortSignal.timeout(timeoutMs);
  const signals = [timeoutSignal, request.signal, ...(init?.signal ? [init.signal] : [])];
  const state = execution.getStore();
  if (state) {
    const safeUrl = new URL(url);
    for (const key of safeUrl.searchParams.keys()) if (isCredentialKey(key)) safeUrl.searchParams.set(key, '[OCULTO]');
    const headers = new Headers(init?.headers ?? (input instanceof Request ? input.headers : undefined));
    state.trace.request = {
      method: init?.method ?? (input instanceof Request ? input.method : 'GET'),
      url: tracePreview(safeUrl.href, request.secrets).value as string,
      path: tracePreview(url.pathname, request.secrets).value as string,
      query: tracePreview(Object.fromEntries(url.searchParams), request.secrets),
      headers: tracePreview(Object.fromEntries(['content-type', 'accept'].filter(name => headers.has(name)).map(name => [name, headers.get(name)!])), request.secrets).value as Record<string, string>,
      body: tracePreview(typeof init?.body === 'string' ? init.body : null, request.secrets),
    };
  }
  try {
    const upstream = await fetch(url, { ...init, signal: AbortSignal.any(signals), redirect: 'manual' });
    if (state) {
      state.status = upstream.status;
      state.kind = upstream.ok ? 'http_response' : 'http_error';
    }
    const contentType = redactSecrets(upstream.headers.get('content-type') ?? 'text/plain; charset=utf-8', request.secrets);
    const statusText = redactSecrets(upstream.statusText, request.secrets);
    if (state) state.trace.response = { status: upstream.status, headers: tracePreview({ 'content-type': contentType }, request.secrets).value as Record<string, string> };
    const declaredSize = Number(upstream.headers.get('content-length'));
    if (Number.isFinite(declaredSize) && declaredSize > maxBytes) {
      await upstream.body?.cancel();
      throw new Error('UPSTREAM_RESPONSE_TOO_LARGE');
    }
    const chunks: Uint8Array[] = [];
    let size = 0;
    if (upstream.body) {
      const reader = upstream.body.getReader();
      try {
        while (true) {
          const chunk = await reader.read();
          if (chunk.done) break;
          size += chunk.value.byteLength;
          if (size > maxBytes) {
            await reader.cancel();
            throw new Error('UPSTREAM_RESPONSE_TOO_LARGE');
          }
          chunks.push(chunk.value);
        }
      } finally { reader.releaseLock(); }
    }
    const rawBody = Buffer.concat(chunks).toString('utf8');
    const body = Buffer.from(redactSecrets(rawBody, request.secrets));
    if (state?.trace.response) state.trace.response.body = tracePreview(body.toString('utf8'), request.secrets);
    if (!upstream.ok && state) {
      try {
        const payload = JSON.parse(body.toString('utf8'));
        const code = payload?.error?.code;
        if (typeof code === 'string' && /^[A-Z0-9_]{1,80}$/.test(code)) state.errorCode = code;
      } catch {}
    }
    return new Response([204, 205, 304].includes(upstream.status) ? null : body, {
      status: upstream.status,
      statusText,
      headers: { 'content-type': contentType },
    });
  } catch (error) {
    if (state) {
      state.kind = 'transport_error';
      state.errorCode = error instanceof Error && error.message === 'UPSTREAM_RESPONSE_TOO_LARGE' ? error.message : error instanceof Error && error.message === 'UPSTREAM_TARGET_INVALID' ? error.message : 'UPSTREAM_CONNECTION_ERROR';
      state.trace.error = state.errorCode;
    }
    if (request.signal.aborted) throw new Error('UPSTREAM_REQUEST_ABORTED');
    if (error instanceof Error && ['UPSTREAM_RESPONSE_TOO_LARGE', 'UPSTREAM_TARGET_INVALID'].includes(error.message)) throw error;
    throw new Error('UPSTREAM_CONNECTION_ERROR');
  }
}

function schemaHasBinaryFormat(schema: unknown): boolean {
  if (!schema || typeof schema !== 'object') return false;
  if (Array.isArray(schema)) return schema.some(schemaHasBinaryFormat);
  const record = schema as Record<string, any>;
  if (record.type === 'string' && record.format === 'binary') return true;
  const mappedChildren = ['properties', 'patternProperties', '$defs', 'definitions', 'dependencies', 'dependentSchemas']
    .flatMap(key => record[key] && typeof record[key] === 'object' ? Object.values(record[key]) : []);
  if (mappedChildren.some(schemaHasBinaryFormat)) return true;
  return ['items', 'prefixItems', 'additionalProperties', 'allOf', 'anyOf', 'oneOf', 'not', 'if', 'then', 'else']
    .some(key => schemaHasBinaryFormat(record[key]));
}

function createSourceProvider(source: LoadedSource, runtime: AsyncLocalStorage<RuntimeContext>, execution: AsyncLocalStorage<ExecutionState>, timeoutMs: number, maxBytes: number, recordExecution: (entry: CapabilityExecution, trace: InspectorTrace) => void): { server: FastMCP; routes: Map<string, RouteMetadata> } {
  const routes = new Map<string, RouteMetadata>();
  const errors: string[] = [];
  const validator = new AjvJsonSchemaValidator();
  const byRoute = new Map(source.capabilities.map(capability => [capability.execution.method + ' ' + capability.execution.path, capability]));
  const baseUrl = source.baseUrl.href.replace(/\/$/, '');
  const server = createOpenAPIServer({
    spec: source.spec,
    name: 'capability-source-' + source.identity.id,
    version: '0.0.1',
    validateOutput: false,
    client: {
      baseUrl,
      timeoutMs,
      auth: {
        async getHeaders() {
          const current = runtime.getStore();
          if (!current?.accessToken) throw new Error('REQUEST_AUTH_CONTEXT_MISSING');
          return { ...current.upstreamHeaders, authorization: 'Bearer ' + current.accessToken };
        },
      },
      fetch: (input, init) => fetchUpstream(input, init, source, runtime, execution, timeoutMs, maxBytes),
    },
    routeMapFn: route => source.exposure.get(route.method.toUpperCase() + ' ' + route.path) === true ? 'tool' : 'exclude',
    componentFn: (route, component) => {
      if (component.kind !== 'tool') return;
      const capability = byRoute.get(route.method.toUpperCase() + ' ' + route.path);
      if (!capability) { errors.push('Tool sem capability registrada na source ' + source.identity.id); return; }
      try {
        for (const mapping of Object.values(route.parameterMap)) {
          if (mapping.location === 'header' && /^(authorization|proxy-authorization|host|cookie|set-cookie|connection|content-length|transfer-encoding|upgrade|expect|forwarded|x-forwarded-.+)$/i.test(mapping.openapiName)) {
            errors.push('Header reservado não pode ser argumento de tool: ' + mapping.openapiName + ' na source ' + source.identity.id);
            return;
          }
          if (mapping.location === 'header' && isCredentialKey(mapping.openapiName) && !configuredHeaderEnv(source.config, mapping.openapiName)) {
            errors.push('Header de credencial requer configuração headerEnv na source ' + source.identity.id + ': ' + mapping.openapiName);
            return;
          }
        }
        normalizeSchemaTypes(component.inputSchema);
        component.name = capability.toolName;
        const normalized = aliasesFor(component.inputSchema as Record<string, any>);
        const configuredHeaders = Object.entries(route.parameterMap).flatMap(([name, mapping]) => {
          if (mapping.location !== 'header') return [];
          const envName = configuredHeaderEnv(source.config, mapping.openapiName);
          if (!envName) return [];
          const parameter = route.parameters.find(item => item.location === 'header' && item.name.toLowerCase() === mapping.openapiName.toLowerCase());
          return [{ argumentName: name, headerName: mapping.openapiName.toLowerCase(), envName, required: parameter?.required ?? false }];
        });
        const properties = normalized.inputSchema.properties as Record<string, unknown>;
        for (const configured of configuredHeaders) {
          delete properties[normalized.renamed.get(configured.argumentName) ?? configured.argumentName];
          delete route.parameterMap[configured.argumentName];
        }
        if (Array.isArray(normalized.inputSchema.required)) {
          const hiddenNames = new Set(configuredHeaders.map(item => normalized.renamed.get(item.argumentName) ?? item.argumentName));
          normalized.inputSchema.required = normalized.inputSchema.required.filter((name: unknown) => !hiddenNames.has(String(name)));
        }
        component.inputSchema = normalized.inputSchema;
        const parameters = Object.fromEntries(Object.entries(route.parameterMap).map(([name, mapping]) => [normalized.renamed.get(name) ?? name, { name: mapping.openapiName, in: mapping.location }]));
        const upstreamHeaders = Object.fromEntries(configuredHeaders.flatMap(item => {
          const value = source.headers[item.headerName];
          return value ? [[item.headerName, value]] : [];
        }));
        const missingRequiredHeaders = configuredHeaders.filter(item => item.required && !source.headers[item.headerName]).map(item => item.envName);
        if (schemaHasBinaryFormat(normalized.inputSchema)) component.description = (component.description ?? '') + ' Limitação: nesta interface MCP baseada em JSON, campos OpenAPI format=binary são enviados como texto no multipart; o envio de arquivo binário não é suportado.';
        routes.set(capability.toolName, {
          capability,
          pathArguments: Object.entries(route.parameterMap).filter(([, mapping]) => mapping.location === 'path').map(([name]) => normalized.renamed.get(name) ?? name),
          argumentAliases: normalized.aliases,
          parameters,
          upstreamHeaders,
          missingRequiredHeaders,
          validate: validator.getValidator(component.inputSchema),
        });
        capability.definition = {
          name: capability.toolName,
          ...(typeof component.description === 'string' ? { description: component.description } : {}),
          inputSchema: component.inputSchema as Record<string, unknown>,
          ...(component.outputSchema && typeof component.outputSchema === 'object' ? { outputSchema: component.outputSchema as Record<string, unknown> } : {}),
        };
      } catch { errors.push('Falha ao normalizar a Tool ' + capability.toolName + ' na source ' + source.identity.id); }
    },
    serverOptions: { http: { cors: false }, dnsRebinding: { enabled: true } },
  });
  if (errors.length || routes.size !== source.capabilities.length || source.capabilities.some(capability => !capability.definition)) {
    void server.close();
    throw new Error(errors[0] ?? 'O catálogo gerado está incompleto na source ' + source.identity.id);
  }
  server.use({
    onCallTool: async (context, next) => {
      const request = context.request as { name: string; arguments?: Record<string, unknown> };
      const route = routes.get(request.name);
      const args = request.arguments ?? {};
      const runtimeContext = runtime.getStore();
      const routeSecrets = Object.values(route?.upstreamHeaders ?? {});
      const secrets = [...new Set([...(runtimeContext?.secrets ?? []), ...routeSecrets])];
      const state: ExecutionState = { kind: 'tool_error', trace: {} };
      const started = performance.now();
      if (route) state.trace.route = { method: route.capability.execution.method, path: route.capability.execution.path };
      state.trace.arguments = tracePreview(args, secrets);
      const finish = () => {
        if (!route) return;
        recordExecution({
          id: randomUUID(),
          timestamp: new Date().toISOString(),
          view: runtimeContext?.viewPath ?? '/mcp',
          sourceId: route.capability.source.id,
          toolName: request.name,
          durationMs: Math.round(performance.now() - started),
          kind: state.kind,
          ...(typeof state.status === 'number' ? { status: state.status } : {}),
          ...(state.errorCode ? { errorCode: state.errorCode } : {}),
        }, state.trace);
      };
      const invoke = () => execution.run(state, async () => {
        try {
          const validation = route?.validate(args);
          if (!route || !validation?.valid || route.pathArguments.some(name => ['.', '..'].includes(String(args[name])))) {
            state.kind = 'argument_error';
            const result = { isError: true, content: [{ type: 'text' as const, text: 'Argumentos inválidos: ' + (validation?.errorMessage ?? 'parâmetro de path inseguro') }] };
            state.trace.result = tracePreview(result, secrets);
            return result;
          }
          if (route.missingRequiredHeaders.length) {
            state.kind = 'auth_error';
            state.errorCode = 'SOURCE_HEADER_NOT_CONFIGURED';
            state.trace.error = state.errorCode;
            const result = { isError: true, content: [{ type: 'text' as const, text: 'Configure a variável ' + route.missingRequiredHeaders.join(', ') + ' no servidor MCP para executar esta tool.' }] };
            state.trace.result = tracePreview(result, secrets);
            return result;
          }
          for (const [alias, original] of route.argumentAliases) {
            if (!Object.hasOwn(args, alias)) continue;
            args[original] = args[alias];
            delete args[alias];
          }
          const result = await next() as { isError?: boolean };
          state.trace.result = tracePreview(result, secrets);
          if (result.isError && state.kind === 'http_response') state.kind = 'tool_error';
          return result;
        } catch (error) {
          state.trace.error = state.errorCode ?? 'TOOL_EXECUTION_ERROR';
          throw error;
        } finally { finish(); }
      });
      const callContext = runtimeContext && route
        ? { ...runtimeContext, secrets, upstreamHeaders: route.upstreamHeaders }
        : runtimeContext;
      return callContext ? runtime.run(callContext, invoke) : invoke();
    },
  });
  return { server, routes };
}

function hierarchySegments(hierarchy: CapabilityHierarchy): string[] {
  return [hierarchy.domain, hierarchy.service, hierarchy.module].filter((segment): segment is string => segment !== undefined);
}

type CapabilityPublishedTool = CapabilityToolDefinition & {
  _meta?: Record<string, unknown>;
  annotations?: Record<string, unknown>;
};

function addCapabilityMetadata(tool: CapabilityPublishedTool, route: RouteMetadata): CapabilityPublishedTool {
  const capability = route.capability;
  return {
    ...tool,
    _meta: {
      ...tool._meta,
      source: capability.source.id,
      hierarchy: capability.source.hierarchy,
      operationId: capability.operationId,
      method: capability.execution.method,
      path: capability.execution.path,
      parameters: route.parameters,
    },
    annotations: {
      ...tool.annotations,
      ...(['GET', 'HEAD'].includes(capability.execution.method) ? { readOnlyHint: true } : {}),
      ...(capability.execution.method === 'DELETE' ? { destructiveHint: true } : {}),
    },
  };
}

function serializeCapabilityTool(capability: Capability, route: RouteMetadata): CapabilityPublishedTool | undefined {
  if (!capability.definition) return undefined;
  return addCapabilityMetadata({ ...capability.definition }, route);
}

function buildViews(registry: CapabilityRegistry, sourceServers: Map<string, FastMCP>, skillResources: CapabilitySkillResource[] = []): Map<string, { server: FastMCP; sourceIds: string[] }> {
  const activeSourceIds = new Set(registry.capabilities.map(capability => capability.source.id));
  const activeSources = registry.sources.filter(source => activeSourceIds.has(source.id));
  const selectors = new Map<string, Set<string>>([['/mcp', new Set(activeSources.map(source => source.id))]]);
  for (const source of activeSources) {
    const segments = hierarchySegments(source.hierarchy);
    for (let index = 1; index <= segments.length; index++) {
      const path = '/mcp/' + segments.slice(0, index).join('/');
      const ids = selectors.get(path) ?? new Set<string>();
      ids.add(source.id);
      selectors.set(path, ids);
    }
  }
  const views = new Map<string, { server: FastMCP; sourceIds: string[] }>();
  for (const [path, ids] of selectors) {
    const view = new FastMCP({ name: path === '/mcp' ? 'capability-view-full' : 'capability-view-' + path.slice(5).replaceAll('/', '-'), version: '0.0.1', http: { cors: false }, dnsRebinding: { enabled: true } });
    for (const id of ids) {
      const source = sourceServers.get(id);
      if (source) view.addProvider(source);
    }
    const viewSegments = path === '/mcp' ? [] : path.slice('/mcp/'.length).split('/');
    const skillMetadataByUri = new Map<string, Record<string, unknown>>();
    for (const skill of skillResources) {
      const hierarchy = hierarchySegments(skill.hierarchy);
      if (hierarchy.length < viewSegments.length || viewSegments.some((segment, index) => hierarchy[index] !== segment)) continue;
      if (skillMetadataByUri.has(skill.uri)) throw new Error('URI de Skill duplicada: ' + skill.uri);
      skillMetadataByUri.set(skill.uri, skill.metadata);
      view.resource({ uri: skill.uri, name: skill.name, description: skill.description, mimeType: skill.mimeType }, async () => skill.read());
    }
    if (skillMetadataByUri.size) {
      view.use({
        onListResources: async (_context, next) => {
          const raw = await next();
          if (!raw || typeof raw !== 'object' || !('resources' in raw) || !Array.isArray(raw.resources)) return raw;
          const result = raw as { resources: Array<Record<string, unknown>>; [key: string]: unknown };
          return {
            ...result,
            resources: result.resources.map(resource => {
              const uri = resource.uri;
              const metadata = typeof uri === 'string' ? skillMetadataByUri.get(uri) : undefined;
              if (!metadata) return resource;
              const existingMeta = resource._meta && typeof resource._meta === 'object' && !Array.isArray(resource._meta)
                ? resource._meta as Record<string, unknown>
                : {};
              return { ...resource, _meta: { ...existingMeta, 'com.gruvi.concierge': metadata } };
            }),
          };
        },
      });
    }
    views.set(path, { server: view, sourceIds: [...ids] });
  }
  return views;
}

function send(response: ServerResponse, status: number, message: string, headers: Record<string, string> = {}): void {
  response.writeHead(status, { 'cache-control': 'no-store', 'content-type': 'text/plain; charset=utf-8', ...headers });
  response.end(message);
}

function sendJson(response: ServerResponse, status: number, value: unknown): void {
  response.writeHead(status, { 'cache-control': 'no-store', 'content-type': 'application/json; charset=utf-8' });
  response.end(JSON.stringify(value));
}

async function readBody(request: IncomingMessage, maxBytes: number): Promise<Buffer> {
  const declaredSize = Number(request.headers['content-length']);
  if (Number.isFinite(declaredSize) && declaredSize > maxBytes) throw new Error('REQUEST_TOO_LARGE');
  const chunks: Buffer[] = [];
  let size = 0;
  for await (const chunk of request) {
    const buffer = Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk);
    size += buffer.byteLength;
    if (size > maxBytes) throw new Error('REQUEST_TOO_LARGE');
    chunks.push(buffer);
  }
  return Buffer.concat(chunks);
}

export class CapabilityViewsServer {
  private httpServer?: NodeHttpServer;
  private readonly runtime = new AsyncLocalStorage<RuntimeContext>();
  private readonly execution = new AsyncLocalStorage<ExecutionState>();
  private readonly sourceServers = new Map<string, FastMCP>();
  private readonly routes = new Map<string, RouteMetadata>();
  private views = new Map<string, { server: FastMCP; sourceIds: string[] }>();
  private controllers = new Set<AbortController>();
  private executions: CapabilityExecution[] = [];
  private traces = new Map<string, InspectorTrace>();
  private inspectorHtml = '';
  registry?: CapabilityRegistry;

  constructor(private readonly config: CapabilityViewsServerConfig) {}

  get url(): string {
    const address = this.httpServer?.address();
    if (!address || typeof address === 'string') throw new Error('O servidor Capability Views não está em execução');
    return 'http://' + (address.address.includes(':') ? '[' + address.address + ']' : address.address) + ':' + address.port + '/mcp';
  }

  get viewPaths(): string[] {
    return [...this.views.keys()].sort();
  }

  get inspectorUrl(): string {
    return this.url.replace(/\/mcp$/, '/');
  }

  get viewUrls(): string[] {
    return this.viewPaths.map(path => new URL(path, this.url).href);
  }

  async start(): Promise<this> {
    if (this.httpServer) throw new Error('O servidor Capability Views já foi iniciado');
    const host = this.config.host ?? '127.0.0.1';
    const port = this.config.port ?? 3336;
    const timeoutMs = this.config.httpTimeoutMs ?? 10000;
    const maxResponseBytes = this.config.httpMaxResponseBytes ?? 1048576;
    const maxRequestBytes = this.config.maxRequestBodyBytes ?? 4194304;
    if (!isLoopback(host)) throw new Error('MCP_VIEWS_HOST deve ser loopback nesta POC local');
    if (!Number.isInteger(port) || port < 0 || port > 65535) throw new Error('MCP_VIEWS_PORT inválida');
    if (![timeoutMs, maxResponseBytes, maxRequestBytes].every(value => Number.isSafeInteger(value) && value > 0)) throw new Error('Limites HTTP devem ser inteiros positivos');
    const { registry, loaded } = await loadRegistry(this.config.sources, this.config.resolvedSourceHeaders, this.config.toolNameBuilder);
    this.registry = registry;
    try {
      this.inspectorHtml = await readFile(new URL('./inspector.html', import.meta.url), 'utf8');
      this.routes.clear();
      for (const source of loaded) {
        if (!source.capabilities.length) continue;
        const provider = createSourceProvider(source, this.runtime, this.execution, timeoutMs, maxResponseBytes, (entry, trace) => this.recordExecution(entry, trace));
        this.sourceServers.set(source.identity.id, provider.server);
        for (const [name, metadata] of provider.routes) this.routes.set(name, metadata);
      }
      this.views = buildViews(registry, this.sourceServers, this.config.skillResources ?? []);
      for (const { server } of this.views.values()) {
        server.use({
          onListTools: async (_context, next) => {
            const result = await next() as { tools: CapabilityPublishedTool[] };
            result.tools = result.tools.map(tool => {
              const route = this.routes.get(tool.name);
              return route ? addCapabilityMetadata(tool, route) : tool;
            });
            return result;
          },
        });
      }
      const httpServer = createHttpServer((request, response) => { void this.handle(request, response); });
      this.httpServer = httpServer;
      await new Promise<void>((resolve, reject) => {
        const onError = (error: Error) => { httpServer.off('listening', onListening); reject(error); };
        const onListening = () => { httpServer.off('error', onError); resolve(); };
        httpServer.once('error', onError);
        httpServer.once('listening', onListening);
        httpServer.listen(port, host);
      });
      return this;
    } catch (error) {
      await this.close();
      throw error;
    }
  }

  async close(): Promise<void> {
    for (const controller of this.controllers) controller.abort();
    this.controllers.clear();
    const server = this.httpServer;
    this.httpServer = undefined;
    if (server?.listening) {
      const closed = new Promise<void>(resolve => server.close(() => resolve()));
      server.closeAllConnections();
      await closed;
    }
    const all = new Set<FastMCP>([...this.views.values()].map(view => view.server));
    for (const source of this.sourceServers.values()) all.add(source);
    await Promise.all([...all].map(mcp => mcp.close().catch(() => undefined)));
    this.sourceServers.clear();
    this.routes.clear();
    this.views.clear();
    this.registry = undefined;
    this.executions = [];
    this.traces.clear();
    this.inspectorHtml = '';
  }

  private allowedOrigins(): Set<string> {
    const address = this.httpServer?.address();
    if (!address || typeof address === 'string') return new Set();
    return new Set(['127.0.0.1', 'localhost', '[::1]'].map(host => 'http://' + host + ':' + address.port));
  }

  private validateHostAndOrigin(request: IncomingMessage): boolean {
    const allowed = this.allowedOrigins();
    const host = request.headers.host;
    if (typeof host !== 'string') return false;
    const address = this.httpServer?.address();
    if (!address || typeof address === 'string') return false;
    const authorities = new Set(['127.0.0.1', 'localhost', '[::1]'].map(name => name + ':' + address.port));
    if (!authorities.has(host.toLowerCase())) return false;
    const requestOrigin = request.headers.origin;
    if (requestOrigin === undefined) return true;
    if (typeof requestOrigin !== 'string') return false;
    try { return allowed.has(new URL(requestOrigin).origin) && new URL(requestOrigin).pathname === '/' && !new URL(requestOrigin).search && !new URL(requestOrigin).hash; } catch { return false; }
  }

  private viewDetails() {
    return this.viewPaths.map(path => {
      const view = this.views.get(path)!;
      const sources = this.registry?.sources.filter(source => view.sourceIds.includes(source.id)) ?? [];
      return {
        path,
        label: path === '/mcp' ? 'Catálogo completo' : path.slice(5).split('/').join(' / '),
        sources,
      };
    });
  }

  private recordExecution(entry: CapabilityExecution, trace: InspectorTrace): void {
    this.executions.push(entry);
    this.traces.set(entry.id, trace);
    if (this.executions.length > 50) this.traces.delete(this.executions.shift()!.id);
  }

  private executionsForView(viewPath: string): CapabilityExecution[] | undefined {
    const view = this.views.get(viewPath);
    if (!view) return undefined;
    if (viewPath === '/mcp') return [...this.executions];
    const sourceIds = new Set(view.sourceIds);
    return this.executions.filter(entry => sourceIds.has(entry.sourceId));
  }

  private async handleInspector(request: IncomingMessage, response: ServerResponse, url: URL): Promise<boolean> {
    const inspectorPath = url.pathname === '/' || ['/api/views', '/api/tools', '/api/executions', '/api/execution'].includes(url.pathname);
    if (!inspectorPath) return false;
    if (request.method !== 'GET') {
      send(response, 405, 'Método não permitido', { allow: 'GET' });
      return true;
    }
    if (url.pathname === '/') {
      response.writeHead(200, { 'cache-control': 'no-store', 'content-type': 'text/html; charset=utf-8' });
      response.end(this.inspectorHtml);
      return true;
    }
    if (url.pathname === '/api/views') {
      sendJson(response, 200, { views: this.viewDetails() });
      return true;
    }
    if (url.pathname === '/api/tools') {
      const viewPath = url.searchParams.get('view') ?? '/mcp';
      const view = this.views.get(viewPath);
      if (!view) { sendJson(response, 404, { error: 'View MCP não encontrada.' }); return true; }
      const capabilities = this.registry?.capabilities.filter(capability => view.sourceIds.includes(capability.source.id)) ?? [];
      const tools = capabilities.flatMap(capability => {
        const route = this.routes.get(capability.toolName);
        const tool = route ? serializeCapabilityTool(capability, route) : undefined;
        return tool ? [tool] : [];
      });
      const mcpUrl = new URL(viewPath, this.url);
      const excluded = view.sourceIds.reduce((total, sourceId) => total + (this.registry?.excludedBySource[sourceId] ?? 0), 0);
      sendJson(response, 200, { tools, mcpUrl: mcpUrl.href, excluded, view: viewPath, sources: this.viewDetails().find(item => item.path === viewPath)?.sources ?? [], implementation: 'fastmcp-ts' });
      return true;
    }
    if (url.pathname === '/api/executions') {
      const viewPath = url.searchParams.get('view') ?? '/mcp';
      const executions = this.executionsForView(viewPath);
      if (!executions) { sendJson(response, 404, { error: 'View MCP não encontrada.' }); return true; }
      sendJson(response, 200, { executions, view: viewPath });
      return true;
    }
    const id = url.searchParams.get('id') ?? '';
    const viewPath = url.searchParams.get('view') ?? '/mcp';
    const allowedExecutions = this.executionsForView(viewPath);
    const execution = allowedExecutions?.find(entry => entry.id === id);
    const trace = execution ? this.traces.get(id) : undefined;
    if (execution && trace) sendJson(response, 200, { execution, trace });
    else sendJson(response, 404, { error: 'Chamada não encontrada, removida ou fora da view selecionada.' });
    return true;
  }

  private async handle(request: IncomingMessage, response: ServerResponse): Promise<void> {
    const controller = new AbortController();
    this.controllers.add(controller);
    const onAborted = () => controller.abort();
    const onClosed = () => { if (!response.writableEnded) controller.abort(); };
    request.once('aborted', onAborted);
    response.once('close', onClosed);
    try {
      if (!this.validateHostAndOrigin(request)) { send(response, 403, 'Host ou Origin não permitido'); return; }
      const host = request.headers.host!;
      const rawTarget = request.url ?? '/';
      if (!rawTarget.startsWith('/') || rawTarget.startsWith('//')) { send(response, 400, 'Request inválida'); return; }
      let url: URL;
      try { url = new URL(rawTarget, 'http://' + host); } catch { send(response, 400, 'Request inválida'); return; }
      if (url.pathname !== rawTarget.split('?', 1)[0]) { send(response, 400, 'Request inválida'); return; }
      if (await this.handleInspector(request, response, url)) return;
      const view = this.views.get(url.pathname);
      if (!view) { send(response, 404, 'View MCP não encontrada'); return; }
      if (!['POST', 'GET', 'DELETE'].includes(request.method ?? '')) { send(response, 405, 'Método não permitido', { allow: 'POST, GET, DELETE' }); return; }
      if (request.method === 'POST') {
        const contentType = request.headers['content-type'];
        if (typeof contentType !== 'string' || contentType.split(';', 1)[0]!.trim().toLowerCase() !== 'application/json') { send(response, 415, 'Content-Type deve ser application/json'); return; }
      }
      const authorization = request.headers.authorization;
      const match = typeof authorization === 'string' ? /^Bearer ([A-Za-z0-9._~+/-]+=*)$/i.exec(authorization) : null;
      if (!match) { send(response, 401, 'Unauthorized', { 'www-authenticate': 'Bearer' }); return; }
      let verified;
      try { verified = await this.config.requestTokenVerifier.verify(match[1]!); } catch { send(response, 401, 'Unauthorized', { 'www-authenticate': 'Bearer' }); return; }
      const body = request.method === 'POST' ? await readBody(request, this.config.maxRequestBodyBytes ?? 4194304) : Buffer.alloc(0);
      if (controller.signal.aborted) return;
      const bodyText = body.toString('utf8');
      const init: RequestInit & { duplex?: 'half' } = {
        method: request.method,
        headers: request.headers as HeadersInit,
        signal: controller.signal,
        ...(request.method === 'POST' ? { body: bodyText, duplex: 'half' as const } : {}),
      };
      const webRequest = new Request(url, init);
      const authInfo = {
        token: verified.token,
        clientId: verified.clientId ?? 'authenticated-client',
        scopes: verified.scopes,
        expiresAt: verified.expiresAt,
        extra: verified.claims,
      };
      const runtime: RuntimeContext = { accessToken: verified.token, secrets: [verified.token], signal: controller.signal, viewPath: url.pathname };
      const mcpResponse = await this.runtime.run(runtime, () => view.server.fetch(webRequest, { authInfo }));
      if (controller.signal.aborted || response.destroyed) { await mcpResponse.body?.cancel(); return; }
      response.statusCode = mcpResponse.status;
      response.statusMessage = redactSecrets(mcpResponse.statusText, runtime.secrets);
      for (const [name, value] of mcpResponse.headers) response.setHeader(name, redactSecrets(value, runtime.secrets));
      if (!mcpResponse.body) { response.end(); return; }
      const reader = mcpResponse.body.getReader();
      const cancelReader = () => { void reader.cancel().catch(() => undefined); };
      controller.signal.addEventListener('abort', cancelReader, { once: true });
      try {
        while (!controller.signal.aborted) {
          const chunk = await reader.read();
          if (chunk.done) break;
          if (!response.write(Buffer.from(chunk.value))) await once(response, 'drain', { signal: controller.signal });
        }
      } finally {
        controller.signal.removeEventListener('abort', cancelReader);
        reader.releaseLock();
      }
      if (!controller.signal.aborted && !response.writableEnded) response.end();
    } catch (error) {
      if (error instanceof Error && error.message === 'REQUEST_TOO_LARGE') {
        if (!response.headersSent) send(response, 413, 'Request muito grande');
        else response.destroy();
      } else if (controller.signal.aborted || response.destroyed) {
        return;
      } else if (!response.headersSent) {
        send(response, 500, 'Falha ao processar request MCP');
      } else response.destroy();
    } finally {
      request.off('aborted', onAborted);
      response.off('close', onClosed);
      this.controllers.delete(controller);
    }
  }
}
