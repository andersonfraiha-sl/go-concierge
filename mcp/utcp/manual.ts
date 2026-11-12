import { createHash } from 'node:crypto';
import { readFile } from 'node:fs/promises';
import { resolve } from 'node:path';
import { OpenApiConverter } from '@utcp/http';
import { UtcpManualSerializer, type Tool, type UtcpManual } from '@utcp/sdk';
import { parse as parseYaml } from 'yaml';

export interface UtcpSourceConfig {
  id: string;
  openapiPath: string;
  upstreamBaseUrl: string;
  kind: 'bff' | 'parecer-pasta';
  appCheckToken?: string;
}

export interface SkippedUtcpOperation {
  source: string;
  operationId: string;
  method: string;
  path: string;
  reason: string;
}

export interface UtcpManualSnapshot {
  manual: UtcpManual;
  sources: Array<{ id: string; baseUrl: string; title: string }>;
  skipped: SkippedUtcpOperation[];
  excludedByPolicy: number;
  discovered: number;
}

export interface UtcpManualBuildOptions {
  sources?: UtcpSourceConfig[];
  env?: NodeJS.ProcessEnv;
  fetch?: typeof fetch;
  maxBytes?: number;
  timeoutMs?: number;
  requestTimeoutMs?: number;
}

export const MANUAL_NAME = 'agent_utcp';
const OPERATIONS = new Set(['get', 'post', 'put', 'patch', 'delete']);
const OPENAPI_OPERATIONS = new Set(['get', 'post', 'put', 'patch', 'delete', 'head', 'options', 'trace']);

function isLoopback(hostname: string): boolean {
  const host = hostname.toLowerCase().replace(/^\[|\]$/g, '');
  return host === 'localhost' || host === '::1' || /^127(?:\.\d{1,3}){3}$/.test(host);
}

function checkedUrl(value: string, label: string): URL {
  let url: URL;
  try { url = new URL(value); } catch { throw new Error(`${label} inválida.`); }
  if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password || url.search || url.hash) throw new Error(`${label} inválida.`);
  if (!isLoopback(url.hostname) && url.protocol !== 'https:') throw new Error(`${label} deve usar HTTPS fora de loopback.`);
  return url;
}

function requiredText(value: string | undefined, name: string): string {
  const normalized = value?.trim();
  if (!normalized) throw new Error(`Configure ${name}.`);
  return normalized;
}

export function utcpSourcesFromEnv(env: NodeJS.ProcessEnv = process.env): UtcpSourceConfig[] {
  const buildingBaseUrl = requiredText(env.MCP_VIEWS_UPSTREAM_BASE_URL?.trim() || env.UPSTREAM_BASE_URL, 'UPSTREAM_BASE_URL ou MCP_VIEWS_UPSTREAM_BASE_URL');
  const sources: UtcpSourceConfig[] = [
    {
      id: 'building-operation',
      kind: 'bff',
      openapiPath: env.MCP_VIEWS_OPENAPI_PATH?.trim() || 'building-operation.json',
      upstreamBaseUrl: buildingBaseUrl,
    },
    {
      id: 'mobile',
      kind: 'bff',
      openapiPath: env.MCP_VIEWS_MOBILE_OPENAPI_PATH?.trim() || 'mobile.json',
      upstreamBaseUrl: env.MCP_VIEWS_MOBILE_UPSTREAM_BASE_URL?.trim() || buildingBaseUrl,
      appCheckToken: env.MCP_VIEWS_MOBILE_APP_CHECK_TOKEN?.trim(),
    },
  ];
  const condCoreBaseUrl = env.MCP_VIEWS_COND_CORE_UPSTREAM_BASE_URL?.trim();
  if (condCoreBaseUrl) sources.push({
    id: 'cond-core',
    kind: 'bff',
    openapiPath: env.MCP_VIEWS_COND_CORE_OPENAPI_PATH?.trim() || 'cond-core.json',
    upstreamBaseUrl: condCoreBaseUrl,
  });
  const parecerEnabled = (env.AGENT_UTCP_PARECER_ENABLED ?? env.MCP_HELP_PARECER_PASTA_ENABLED)?.trim().toLowerCase() === 'true';
  if (parecerEnabled) sources.push({
    id: 'parecer-pasta',
    kind: 'parecer-pasta',
    openapiPath: env.AGENT_UTCP_PARECER_OPENAPI_URL?.trim() || env.MCP_HELP_PARECER_PASTA_OPENAPI_URL?.trim() || 'https://gateway-contas-pagar.idp.nexus.superlogica.net/swagger/openapi/pagamentos/parecer-pasta.yaml',
    upstreamBaseUrl: env.AGENT_UTCP_PARECER_UPSTREAM_BASE_URL?.trim() || env.MCP_HELP_PARECER_PASTA_UPSTREAM_BASE_URL?.trim() || 'https://gateway-contas-pagar.idp.nexus.superlogica.net/parecer-pasta',
  });
  return sources;
}

async function readBounded(response: Response, maxBytes: number): Promise<Uint8Array> {
  const declared = Number(response.headers.get('content-length'));
  if (Number.isFinite(declared) && declared > maxBytes) {
    await response.body?.cancel();
    throw new Error('OpenAPI excede o limite configurado.');
  }
  const reader = response.body?.getReader();
  if (!reader) return new Uint8Array();
  const chunks: Uint8Array[] = [];
  let size = 0;
  try {
    while (true) {
      const chunk = await reader.read();
      if (chunk.done) break;
      size += chunk.value.byteLength;
      if (size > maxBytes) {
        await reader.cancel();
        throw new Error('OpenAPI excede o limite configurado.');
      }
      chunks.push(chunk.value);
    }
  } finally {
    reader.releaseLock();
  }
  return Buffer.concat(chunks);
}

async function loadDocument(location: string, fetchImpl: typeof fetch, maxBytes: number, timeoutMs: number): Promise<Record<string, any>> {
  let content: string;
  let contentType = '';
  if (/^https?:\/\//i.test(location)) {
    const url = checkedUrl(location, 'URL da OpenAPI');
    const response = await fetchImpl(url, { signal: AbortSignal.timeout(timeoutMs), redirect: 'error' });
    if (!response.ok) throw new Error(`Falha ao carregar OpenAPI (HTTP ${response.status}).`);
    contentType = response.headers.get('content-type') ?? '';
    content = Buffer.from(await readBounded(response, maxBytes)).toString('utf8');
  } else {
    const bytes = await readFile(resolve(location));
    if (bytes.byteLength > maxBytes) throw new Error('OpenAPI excede o limite configurado.');
    content = bytes.toString('utf8');
  }
  const isYaml = /\.ya?ml(?:$|[?#])/i.test(location) || /ya?ml/i.test(contentType);
  let parsed: unknown;
  try { parsed = isYaml ? parseYaml(content) : JSON.parse(content); }
  catch {
    try { parsed = isYaml ? JSON.parse(content) : parseYaml(content); }
    catch { throw new Error('Documento OpenAPI JSON/YAML inválido.'); }
  }
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed) || !('paths' in parsed)) throw new Error('Documento OpenAPI sem paths.');
  return parsed as Record<string, any>;
}

function exposure(spec: Record<string, any>, pathItem: Record<string, any>, operation: Record<string, any>): boolean {
  const value = operation['x-mcp'] ?? pathItem['x-mcp'] ?? spec['x-mcp'] ?? true;
  if (typeof value !== 'boolean') throw new Error('x-mcp precisa ser boolean.');
  return value;
}

function resolvePointer(root: Record<string, any>, reference: string): unknown {
  if (!reference.startsWith('#/')) return undefined;
  let value: any = root;
  for (const part of reference.slice(2).split('/')) {
    const key = part.replace(/~1/g, '/').replace(/~0/g, '~');
    if (!value || typeof value !== 'object' || !Object.hasOwn(value, key)) return undefined;
    value = value[key];
  }
  return value;
}

function resolveSchemaReferences(input: Record<string, any>, spec: Record<string, any>): Record<string, any> {
  const definitions: Record<string, any> = {};
  const names = new Map<string, string>();
  const resolving = new Set<string>();
  const definitionName = (reference: string) => {
    const existing = names.get(reference);
    if (existing) return existing;
    const name = `ref_${createHash('sha256').update(reference).digest('hex').slice(0, 12)}`;
    names.set(reference, name);
    return name;
  };
  const walk = (value: any): any => {
    if (Array.isArray(value)) return value.map(walk);
    if (!value || typeof value !== 'object') return value;
    if (typeof value.$ref === 'string') {
      const reference = value.$ref;
      const target = resolvePointer(spec, reference);
      if (!target) return Object.fromEntries(Object.entries(value).map(([key, child]) => [key, walk(child)]));
      const name = definitionName(reference);
      if (!Object.hasOwn(definitions, name) && !resolving.has(reference)) {
        resolving.add(reference);
        definitions[name] = walk(target);
        resolving.delete(reference);
      }
      const siblings = Object.fromEntries(Object.entries(value).filter(([key]) => key !== '$ref').map(([key, child]) => [key, walk(child)]));
      return { ...siblings, $ref: `#/$defs/${name}` };
    }
    const result = Object.fromEntries(Object.entries(value).map(([key, child]) => [key, walk(child)]));
    if (result.type === 'uuid') {
      result.type = 'string';
      result.format = result.format ?? 'uuid';
    }
    return result;
  };
  const result = walk(input) as Record<string, any>;
  if (Object.keys(definitions).length) result.$defs = { ...(result.$defs ?? {}), ...definitions };
  return result;
}

function containsBinary(value: unknown, seen = new WeakSet<object>()): boolean {
  if (!value || typeof value !== 'object') return false;
  if (seen.has(value)) return false;
  seen.add(value);
  if (Array.isArray(value)) return value.some(child => containsBinary(child, seen));
  const record = value as Record<string, unknown>;
  if (record.type === 'file' || record.format === 'binary' || record.format === 'base64') return true;
  return Object.values(record).some(child => containsBinary(child, seen));
}

function requestBodyReason(operation: Record<string, any>): string | undefined {
  const content = operation.requestBody?.content;
  if (!content || typeof content !== 'object') return undefined;
  if (Object.keys(content).some(type => /multipart\/form-data|application\/octet-stream/i.test(type))) return 'upload binário ou multipart não suportado';
  if (containsBinary(content)) return 'campo binário não suportado';
  if (!Object.keys(content).some(type => type === 'application/json' || type === 'application/x-www-form-urlencoded')) return 'tipo de body não suportado pelo conversor UTCP';
  return undefined;
}

function responseReason(operation: Record<string, any>): string | undefined {
  for (const response of Object.values(operation.responses ?? {}) as Array<Record<string, any>>) {
    for (const [type, media] of Object.entries(response?.content ?? {})) {
      if (/octet-stream|image\/|audio\/|video\/|application\/pdf|application\/zip/i.test(type) || containsBinary(media)) return 'resposta binária não suportada';
    }
  }
  return undefined;
}

function securityIsSupported(spec: Record<string, any>, operation: Record<string, any>, source: UtcpSourceConfig): boolean {
  const requirements = operation.security === undefined ? spec.security ?? [] : operation.security;
  if (!Array.isArray(requirements) || requirements.length === 0) return true;
  const schemes = spec.components?.securitySchemes ?? spec.securityDefinitions ?? {};
  return requirements.some((requirement: unknown) => {
    if (!requirement || typeof requirement !== 'object' || Array.isArray(requirement)) return false;
    return Object.keys(requirement).every(name => {
      const scheme = schemes[name] as Record<string, unknown> | undefined;
      if (!scheme) return false;
      const type = String(scheme.type ?? '').toLowerCase();
      if (type === 'http' && String(scheme.scheme ?? '').toLowerCase() === 'bearer') return true;
      if (type === 'oauth2') return true;
      if (source.kind === 'parecer-pasta' && type === 'apikey' && String(scheme.in ?? '').toLowerCase() === 'header' && String(scheme.name ?? '').toLowerCase() === 'x-company-id') return true;
      return false;
    });
  });
}

function operationId(operation: Record<string, any>, method: string, path: string): string {
  const raw = typeof operation.operationId === 'string' && operation.operationId.trim() ? operation.operationId.trim() : '';
  if (!raw) throw new Error(`operationId ausente em ${method.toUpperCase()} ${path}.`);
  return raw;
}

function operationLabel(operation: Record<string, any>, method: string, path: string): string {
  return typeof operation.operationId === 'string' && operation.operationId.trim() ? operation.operationId.trim() : `${method.toUpperCase()} ${path}`;
}

function safeName(value: string): string {
  return value.replace(/[^a-zA-Z0-9_.-]/g, '_').replace(/_+/g, '_') || 'tool';
}

function pathInputs(tool: Tool, path: string): void {
  const properties = (tool.inputs.properties ??= {}) as Record<string, any>;
  const required = new Set(Array.isArray(tool.inputs.required) ? tool.inputs.required : []);
  for (const [, parameter] of path.matchAll(/\{([^{}]+)\}/g)) {
    if (!Object.hasOwn(properties, parameter)) properties[parameter] = { type: 'string', description: 'Parâmetro de rota declarado no path OpenAPI.' };
    required.add(parameter);
  }
  if (required.size) tool.inputs.required = [...required];
}

function removeInjectedHeader(tool: Tool, header: string): void {
  const key = Object.keys(tool.inputs.properties ?? {}).find(value => value.toLowerCase() === header.toLowerCase());
  if (!key) return;
  delete (tool.inputs.properties as Record<string, unknown>)[key];
  tool.inputs.required = (tool.inputs.required ?? []).filter(value => value !== key);
  const template = tool.tool_call_template as Record<string, any>;
  template.header_fields = (template.header_fields ?? []).filter((value: string) => value.toLowerCase() !== header.toLowerCase());
}

function normalizeTool(tool: Tool, source: UtcpSourceConfig, path: string, method: string, requestTimeoutMs: number): void {
  const template = tool.tool_call_template as Record<string, any>;
  const headers: Record<string, string> = { ...(template.headers ?? {}) };
  if (source.kind === 'parecer-pasta') {
    headers.authorization = 'Bearer ${PARECER_GATEWAY_TOKEN}';
    headers['x-company-id'] = '${PARECER_COMPANY_ID}';
    removeInjectedHeader(tool, 'authorization');
    removeInjectedHeader(tool, 'x-company-id');
  } else {
    headers.authorization = 'Bearer ${B2C_TOKEN}';
    removeInjectedHeader(tool, 'authorization');
  }
  if (source.id === 'mobile') {
    const mobileHeader = Object.keys(tool.inputs.properties ?? {}).find(key => key.toLowerCase() === 'x-app-check-token');
    if (mobileHeader) {
      if (source.appCheckToken) headers['x-app-check-token'] = '${MOBILE_APP_CHECK_TOKEN}';
      removeInjectedHeader(tool, 'x-app-check-token');
    }
  }
  for (const header of template.header_fields ?? []) {
    if (/^(host|cookie|set-cookie|content-length|transfer-encoding|connection|upgrade|expect|forwarded|x-forwarded-)/i.test(header)) throw new Error(`Header de controle ${header} não pode ser argumento de tool.`);
    if (/authorization|token|secret|password|api[-_]?key|credential/i.test(header) && !/^(authorization|x-app-check-token)$/i.test(header)) throw new Error(`Header credencial ${header} precisa de injeção configurada.`);
  }
  template.headers = headers;
  template.auth = undefined;
  template.timeout = requestTimeoutMs;
  pathInputs(tool, path);
  tool.inputs.additionalProperties = false;
  tool.tags = [...new Set([...tool.tags, source.id])];
  tool.name = `${MANUAL_NAME}.${safeName(source.id)}.${safeName(tool.name)}`;
  tool.description = tool.description.trim();
  if (!tool.description) tool.description = `${method.toUpperCase()} ${path}`;
}

function convertSource(spec: Record<string, any>, source: UtcpSourceConfig, requestTimeoutMs: number, skipped: SkippedUtcpOperation[]): Tool[] {
  const candidates: Array<{ id: string; path: string; method: string; operation: Record<string, any> }> = [];
  const ids = new Map<string, number>();
  const filteredSpec = structuredClone(spec);
  for (const [path, rawItem] of Object.entries(filteredSpec.paths ?? {})) {
    const item = rawItem as Record<string, any>;
    for (const [method, operation] of Object.entries(item)) {
      if (!OPENAPI_OPERATIONS.has(method.toLowerCase()) || !operation || typeof operation !== 'object') continue;
      const operationData = operation as Record<string, any>;
      if (!exposure(filteredSpec, item, operationData)) {
        skipped.push({ source: source.id, operationId: operationLabel(operationData, method, path), method: method.toUpperCase(), path, reason: 'x-mcp=false' });
        delete item[method];
        continue;
      }
      if (!OPERATIONS.has(method.toLowerCase())) {
        skipped.push({ source: source.id, operationId: operationLabel(operationData, method, path), method: method.toUpperCase(), path, reason: 'método HTTP não suportado pelo protocolo HTTP UTCP' });
        delete item[method];
        continue;
      }
      const id = operationId(operationData, method, path);
      let reason = requestBodyReason(operationData) ?? responseReason(operationData);
      if (!reason && !securityIsSupported(filteredSpec, operationData, source)) reason = 'autenticação API key não suportada nesta operação';
      const parameters = [...(item.parameters ?? []), ...(operationData.parameters ?? [])] as Array<Record<string, any>>;
      const missingAppCheck = parameters.some(parameter => parameter.in === 'header' && parameter.required === true && typeof parameter.name === 'string' && /x-app-check-token/i.test(parameter.name) && !source.appCheckToken);
      const unsupportedHeader = parameters.find(parameter => parameter.in === 'header' && typeof parameter.name === 'string' && /authorization|token|secret|password|api[-_]?key|credential/i.test(parameter.name) && !/^(authorization|x-app-check-token|x-company-id)$/i.test(parameter.name));
      if (!reason && unsupportedHeader) reason = `header credencial ${unsupportedHeader.name} sem injeção suportada`;
      if (!reason && missingAppCheck) reason = 'header obrigatório x-app-check-token sem configuração';
      if (reason) {
        skipped.push({ source: source.id, operationId: id, method: method.toUpperCase(), path, reason });
        delete item[method];
        continue;
      }
      candidates.push({ id, path, method, operation: operationData });
      ids.set(id, (ids.get(id) ?? 0) + 1);
    }
  }
  const converter = new OpenApiConverter(filteredSpec, {
    callTemplateName: `utcp_${safeName(source.id)}`,
    baseUrl: checkedUrl(source.upstreamBaseUrl, `UPSTREAM_BASE_URL da source ${source.id}`).href.replace(/\/$/, ''),
    authTools: null,
  });
  const converted = converter.convert();
  if (converted.tools.length !== candidates.length) throw new Error(`O conversor UTCP omitiu operações habilitadas da source ${source.id}.`);
  converted.tools.forEach((convertedTool, index) => {
    const candidate = candidates[index]!;
    const duplicate = (ids.get(candidate.id) ?? 0) > 1;
    const suffix = duplicate ? `.${candidate.method.toLowerCase()}.${createHash('sha256').update(`${candidate.method.toUpperCase()} ${candidate.path}`).digest('hex').slice(0, 8)}` : '';
    convertedTool.name = `${candidate.id}${suffix}`;
    const combined = [candidate.operation.summary, candidate.operation.description].filter((value): value is string => typeof value === 'string' && value.trim().length > 0).join('\n\n');
    if (combined) convertedTool.description = combined;
    normalizeTool(convertedTool, source, candidate.path, candidate.method, requestTimeoutMs);
  });
  return converted.tools;
}

function normalizeJsonSchema(value: any): any {
  if (Array.isArray(value)) return value.map(normalizeJsonSchema);
  if (!value || typeof value !== 'object') return value;
  const nullable = value.nullable === true;
  const result = Object.fromEntries(Object.entries(value).filter(([key]) => key !== 'nullable').map(([key, child]) => [key, normalizeJsonSchema(child)]));
  if (result.type === 'uuid') {
    result.type = 'string';
    result.format = result.format ?? 'uuid';
  }
  if (!nullable) return result;
  return { anyOf: [result, { type: 'null' }] };
}

function normalizeRefsForTool(tool: Tool, spec: Record<string, any>): void {
  tool.inputs = normalizeJsonSchema(resolveSchemaReferences(tool.inputs as Record<string, any>, spec));
  tool.outputs = normalizeJsonSchema(resolveSchemaReferences(tool.outputs as Record<string, any>, spec));
}

export async function buildUtcpManual(options: UtcpManualBuildOptions = {}): Promise<UtcpManualSnapshot> {
  const env = options.env ?? process.env;
  const sources = options.sources ?? utcpSourcesFromEnv(env);
  if (!sources.length) throw new Error('Configure ao menos uma source OpenAPI para o manual UTCP.');
  const fetchImpl = options.fetch ?? fetch;
  const maxBytes = options.maxBytes ?? Number(env.AGENT_UTCP_OPENAPI_MAX_BYTES ?? 2097152);
  const timeoutMs = options.timeoutMs ?? Number(env.AGENT_UTCP_OPENAPI_TIMEOUT_MS ?? 10000);
  const requestTimeoutMs = options.requestTimeoutMs ?? Number(env.AGENT_UTCP_HTTP_TIMEOUT_MS ?? 15000);
  for (const value of [maxBytes, timeoutMs, requestTimeoutMs]) if (!Number.isSafeInteger(value) || value <= 0) throw new Error('Limites UTCP devem ser inteiros positivos.');
  const seenSources = new Set<string>();
  const seenToolNames = new Set<string>();
  const allTools: Tool[] = [];
  const skipped: SkippedUtcpOperation[] = [];
  const sourceSnapshots: UtcpManualSnapshot['sources'] = [];
  let discovered = 0;
  let excludedByPolicy = 0;
  for (const source of sources) {
    if (!/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(source.id) || seenSources.has(source.id)) throw new Error(`ID de source duplicado ou inválido: ${source.id}`);
    seenSources.add(source.id);
    const baseUrl = checkedUrl(source.upstreamBaseUrl, `UPSTREAM_BASE_URL da source ${source.id}`);
    const spec = await loadDocument(source.openapiPath, fetchImpl, maxBytes, timeoutMs);
    const operations = Object.entries(spec.paths ?? {}).flatMap(([path, rawItem]) => Object.entries(rawItem as Record<string, any>).flatMap(([method, operation]) => {
      if (!OPENAPI_OPERATIONS.has(method.toLowerCase()) || !operation || typeof operation !== 'object') return [];
      const item = rawItem as Record<string, any>;
      return [{ path, method, operation: operation as Record<string, any>, enabled: exposure(spec, item, operation as Record<string, any>) }];
    }));
    discovered += operations.length;
    excludedByPolicy += operations.filter(item => !item.enabled).length;
    const convertedTools = convertSource(spec, source, requestTimeoutMs, skipped);
    for (const tool of convertedTools) {
      normalizeRefsForTool(tool, spec);
      if (seenToolNames.has(tool.name)) throw new Error(`Colisão de nome UTCP: ${tool.name}`);
      seenToolNames.add(tool.name);
      allTools.push(tool);
    }
    sourceSnapshots.push({ id: source.id, baseUrl: baseUrl.href, title: String(spec.info?.title ?? source.id) });
  }
  const manual = new UtcpManualSerializer().validateDict({ tools: allTools });
  return { manual, sources: sourceSnapshots, skipped, excludedByPolicy, discovered };
}

export function utcpManualIndex(snapshot: UtcpManualSnapshot) {
  const unsupported = snapshot.skipped.filter(item => item.reason !== 'x-mcp=false');
  return {
    utcpManualPath: '/utcp',
    sources: snapshot.sources.map(({ id, baseUrl, title }) => ({ id, baseUrl, title })),
    counts: {
      discovered: snapshot.discovered,
      published: snapshot.manual.tools.length,
      excludedByPolicy: snapshot.excludedByPolicy,
      unsupported: unsupported.length,
    },
    unsupported,
  };
}
