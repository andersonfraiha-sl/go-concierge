import { AsyncLocalStorage } from 'node:async_hooks';
import { randomUUID } from 'node:crypto';
import { mkdir, rename, writeFile } from 'node:fs/promises';
import { dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

export const MCP_CATALOG_SESSION_PATH = fileURLToPath(new URL('./session-mcp-catalog-agent.json', import.meta.url));

type SessionEvent = {
  sequence: number;
  timestamp: string;
  type: string;
  correlation?: Record<string, unknown>;
  data: unknown;
};

type SessionDocument = {
  schemaVersion: 1;
  sessionId: string;
  threadId: string;
  startedAt: string;
  endedAt?: string;
  model: string;
  status: 'running' | 'completed' | 'failed';
  snapshot?: { tools: unknown[]; lightweightCatalog: unknown[] };
  totals: Record<string, number>;
  costEstimate: {
    currency: 'USD';
    referenceModel: 'gpt-6-luna';
    pricingScope: 'Standard; prompt cache write 30m';
    gatewayPricingVerified: false;
    source: string;
    checkedAt: '2026-10-27';
    ratesPerMillionTokens: { input: 0.1; cachedInputRead: 0.01; cachedInputWrite30m: 0.125; output: 0.5 };
    status: 'complete' | 'incomplete';
    responses: number;
    completeResponses: number;
    missingFields: string[];
    estimatedAmountUsd?: number;
  };
  events: SessionEvent[];
};

const SENSITIVE_KEYS = new Set([
  'authorization', 'proxyauthorization', 'cookie', 'setcookie', 'apikey', 'xapikey',
  'token', 'accesstoken', 'refreshtoken', 'idtoken', 'bearertoken', 'authtoken', 'oauthtoken',
  'oauthaccesstoken', 'sessiontoken', 'csrftoken', 'xsrftoken', 'clienttoken',
  'clientsecret', 'secret', 'secretkey', 'password', 'clientpassword', 'passwd',
  'appchecktoken', 'xappchecktoken', 'b2cclientkeycloaktoken',
]);

function sensitiveKey(key: string): boolean {
  const normalized = key.replace(/[^A-Za-z0-9]/g, '').toLowerCase();
  return SENSITIVE_KEYS.has(normalized);
}

function redactString(value: string, secrets: string[]): string {
  const known = [...secrets].filter(Boolean).sort((left, right) => right.length - left.length).reduce((text, secret) => text.replaceAll(secret, '[REDACTED]'), value);
  return known
    .replace(/\bBearer\s+[A-Za-z0-9._~+/-]+=*/gi, 'Bearer [REDACTED]')
    .replace(/\beyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\b/g, '[REDACTED]')
    .replace(/\b(?:sk|pk|rk)-[A-Za-z0-9_-]{16,}\b/g, '[REDACTED]');
}

function sanitize(value: unknown, secrets: string[], key = '', seen = new WeakSet<object>()): unknown {
  if (sensitiveKey(key)) return '[REDACTED]';
  if (typeof value === 'string') {
    const cleaned = redactString(value, secrets);
    if (cleaned.trimStart().startsWith('{') || cleaned.trimStart().startsWith('[')) {
      try { return JSON.stringify(sanitize(JSON.parse(cleaned), secrets, '', seen)); } catch {}
    }
    return cleaned;
  }
  if (typeof value === 'bigint') return value.toString();
  if (typeof value === 'undefined') return null;
  if (value === null || typeof value === 'number' || typeof value === 'boolean') return value;
  if (value instanceof Error) {
    const details: Record<string, unknown> = { name: value.name, message: redactString(value.message, secrets) };
    const source = value as Error & Record<string, unknown>;
    for (const field of ['code', 'data', 'status', 'statusCode', 'details']) if (source[field] !== undefined) details[field] = sanitize(source[field], secrets, field, seen);
    return details;
  }
  if (value instanceof Date) return value.toISOString();
  if (value instanceof Uint8Array) return Buffer.from(value).toString('base64');
  if (typeof value !== 'object') return redactString(String(value), secrets);
  if (seen.has(value)) return '[Circular]';
  seen.add(value);
  if (Array.isArray(value)) {
    const result = value.map(item => sanitize(item, secrets, '', seen));
    seen.delete(value);
    return result;
  }
  const result = Object.fromEntries(Object.entries(value).map(([childKey, child]) => [childKey, sanitize(child, secrets, childKey, seen)]));
  seen.delete(value);
  return result;
}

function jsonBody(body: unknown): unknown {
  if (typeof body !== 'string') return body ?? null;
  try { return JSON.parse(body); } catch { return body; }
}

export class SessionRecorder {
  readonly sessionId = randomUUID();
  readonly threadId = randomUUID();
  readonly startedAt = new Date().toISOString();
  readonly path: string;
  private readonly secrets: string[];
  private readonly turn = new AsyncLocalStorage<string>();
  private readonly document: SessionDocument;
  private queue: Promise<void> = Promise.resolve();
  private writeFailure?: Error;

  private constructor(path: string, model: string, secrets: string[]) {
    this.path = path;
    this.secrets = [...new Set(secrets.filter(Boolean))];
    this.document = {
      schemaVersion: 1,
      sessionId: this.sessionId,
      threadId: this.threadId,
      startedAt: this.startedAt,
      model,
      status: 'running',
      totals: { inferenceRequests: 0, inferences: 0, responsesWithUsage: 0, responsesWithoutUsage: 0, inspectCalls: 0, toolCalls: 0, failures: 0, inputTokens: 0, outputTokens: 0, cachedInputTokens: 0, cacheWriteTokens: 0, reasoningTokens: 0, llmDurationMs: 0, durationMs: 0 },
      costEstimate: {
        currency: 'USD',
        referenceModel: 'gpt-6-luna',
        pricingScope: 'Standard; prompt cache write 30m',
        gatewayPricingVerified: false,
        source: 'OpenAI Standard pricing: https://developers.openai.com/api/docs/models/gpt-6-luna and https://developers.openai.com/api/docs/guides/prompt-caching',
        checkedAt: '2026-10-27',
        ratesPerMillionTokens: { input: 0.1, cachedInputRead: 0.01, cachedInputWrite30m: 0.125, output: 0.5 },
        status: 'incomplete',
        responses: 0,
        completeResponses: 0,
        missingFields: [],
      },
      events: [],
    };
  }

  static async start(options: { path?: string; model: string; secrets?: string[] }): Promise<SessionRecorder> {
    const recorder = new SessionRecorder(options.path ?? MCP_CATALOG_SESSION_PATH, options.model, options.secrets ?? []);
    await mkdir(dirname(recorder.path), { recursive: true });
    await recorder.persist();
    return recorder;
  }

  get currentTurnId(): string | undefined {
    return this.turn.getStore();
  }

  async runTurn<T>(turnId: string, fn: () => Promise<T>): Promise<T> {
    return this.turn.run(turnId, fn);
  }

  async setSnapshot(tools: unknown[], lightweightCatalog: unknown[]): Promise<void> {
    this.document.snapshot = sanitize({ tools, lightweightCatalog }, this.secrets) as SessionDocument['snapshot'];
    await this.record('catalog_snapshot', { tools: tools.length, lightweightCatalog: lightweightCatalog.length });
  }

  async record(type: string, data: unknown, correlation: Record<string, unknown> = {}): Promise<void> {
    const event: SessionEvent = {
      sequence: this.document.events.length + 1,
      timestamp: new Date().toISOString(),
      type,
      ...(Object.keys(correlation).length ? { correlation: sanitize({ turnId: this.currentTurnId, ...correlation }, this.secrets) as Record<string, unknown> } : this.currentTurnId ? { correlation: { turnId: this.currentTurnId } } : {}),
      data: sanitize(data, this.secrets),
    };
    this.document.events.push(event);
    if (type === 'llm_http_request') this.document.totals.inferenceRequests++;
    if (type === 'llm_http_response') {
      this.document.totals.inferences++;
      this.addUsage((data as Record<string, unknown> | undefined)?.response);
    }
    if (type === 'llm_http_response' || type === 'llm_http_error') this.document.totals.llmDurationMs += Number((data as Record<string, unknown> | undefined)?.durationMs ?? 0);
    if (type === 'llm_http_error') this.addCostMissing('response_after_http_error');
    if (type === 'inspect_mcp_tool') this.document.totals.inspectCalls++;
    if (type === 'call_mcp_tool') this.document.totals.toolCalls++;
    if (type.endsWith('_error') || (typeof data === 'object' && data !== null && 'ok' in data && (data as { ok?: unknown }).ok === false)) this.document.totals.failures++;
    const write = () => this.persist();
    const pending = this.queue.then(write, write);
    this.queue = pending.catch(error => { this.writeFailure = error instanceof Error ? error : new Error(String(error)); });
    await pending;
  }

  async finish(status: 'completed' | 'failed'): Promise<void> {
    this.document.status = status;
    this.document.endedAt = new Date().toISOString();
    this.document.totals.durationMs = Date.parse(this.document.endedAt) - Date.parse(this.document.startedAt);
    await this.queue;
    await this.persist();
    if (this.writeFailure) throw new Error('Falha ao gravar o arquivo de sessão: ' + this.writeFailure.message);
  }

  createFetch(delegate: typeof fetch): typeof fetch {
    return async (input: Parameters<typeof fetch>[0], init?: Parameters<typeof fetch>[1]) => {
      const requestId = randomUUID();
      const started = performance.now();
      const inputBody = init?.body ?? (input instanceof Request ? await input.clone().text() : null);
      const request = { method: init?.method ?? (input instanceof Request ? input.method : 'GET'), body: jsonBody(inputBody) };
      await this.record('llm_http_request', { request }, { requestId });
      try {
        const response = await delegate(input, init);
        const text = await response.clone().text();
        let body: unknown;
        try { body = JSON.parse(text); } catch { body = text; }
        await this.record('llm_http_response', {
          request,
          response: body,
          status: response.status,
          durationMs: Math.round(performance.now() - started),
        }, { requestId });
        return response;
      } catch (error) {
        await this.record('llm_http_error', { request, error, durationMs: Math.round(performance.now() - started) }, { requestId });
        throw error;
      }
    };
  }

  private addUsage(response: unknown): void {
    this.document.costEstimate.responses++;
    if (!response || typeof response !== 'object') {
      this.document.totals.responsesWithoutUsage++;
      this.addCostMissing('response_usage');
      return;
    }
    const usage = 'usage' in response ? (response as { usage?: unknown }).usage : undefined;
    if (!usage || typeof usage !== 'object') {
      this.document.totals.responsesWithoutUsage++;
      this.addCostMissing('usage');
      return;
    }
    const values = usage as Record<string, unknown>;
    this.document.totals.responsesWithUsage++;
    const input = values.input_tokens ?? values.prompt_tokens;
    const output = values.output_tokens ?? values.completion_tokens;
    const cached = (values.input_tokens_details as Record<string, unknown> | undefined)?.cached_tokens ?? (values.prompt_tokens_details as Record<string, unknown> | undefined)?.cached_tokens;
    const cacheWrite = (values.input_tokens_details as Record<string, unknown> | undefined)?.cache_write_tokens ?? (values.input_tokens_details as Record<string, unknown> | undefined)?.cache_creation_tokens;
    const reasoning = (values.output_tokens_details as Record<string, unknown> | undefined)?.reasoning_tokens ?? (values.completion_tokens_details as Record<string, unknown> | undefined)?.reasoning_tokens;
    if (typeof input === 'number') this.document.totals.inputTokens += input;
    if (typeof output === 'number') this.document.totals.outputTokens += output;
    if (typeof cached === 'number') this.document.totals.cachedInputTokens += cached;
    if (typeof cacheWrite === 'number') this.document.totals.cacheWriteTokens += cacheWrite;
    if (typeof reasoning === 'number') this.document.totals.reasoningTokens += reasoning;
    const missing = [
      ...(typeof input === 'number' ? [] : ['input_tokens']),
      ...(typeof output === 'number' ? [] : ['output_tokens']),
      ...(typeof cached === 'number' ? [] : ['input_tokens_details.cached_tokens']),
      ...(typeof cacheWrite === 'number' ? [] : ['input_tokens_details.cache_write_tokens']),
    ];
    if (missing.length || (cached as number) + (cacheWrite as number) > (input as number)) {
      this.addCostMissing(...missing, ...((cached as number) + (cacheWrite as number) > (input as number) ? ['input_token_details_inconsistent'] : []));
      return;
    }
    const uncached = (input as number) - (cached as number) - (cacheWrite as number);
    const rates = this.document.costEstimate.ratesPerMillionTokens;
    const amount = (uncached * rates.input + (cached as number) * rates.cachedInputRead + (cacheWrite as number) * rates.cachedInputWrite30m + (output as number) * rates.output) / 1_000_000;
    this.document.costEstimate.completeResponses++;
    this.document.costEstimate.estimatedAmountUsd = (this.document.costEstimate.estimatedAmountUsd ?? 0) + amount;
    if (this.document.costEstimate.completeResponses === this.document.costEstimate.responses && !this.document.costEstimate.missingFields.length) this.document.costEstimate.status = 'complete';
    else {
      this.document.costEstimate.status = 'incomplete';
      delete this.document.costEstimate.estimatedAmountUsd;
    }
  }

  private addCostMissing(...fields: string[]): void {
    this.document.costEstimate.status = 'incomplete';
    delete this.document.costEstimate.estimatedAmountUsd;
    this.document.costEstimate.missingFields = [...new Set([...this.document.costEstimate.missingFields, ...fields])];
  }

  private async persist(): Promise<void> {
    const temporaryPath = this.path + '.' + process.pid + '.tmp';
    await writeFile(temporaryPath, JSON.stringify(this.document, null, 2), 'utf8');
    await rename(temporaryPath, this.path);
  }
}
