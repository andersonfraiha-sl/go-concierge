import assert from 'node:assert/strict';
import { once } from 'node:events';
import { createServer } from 'node:http';
import { mkdtemp, readFile, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { test } from 'node:test';
import { ToolMessage } from '@langchain/core/messages';
import { CapabilityViewsServer, capabilityViewsConfigFromEnv, type CapabilitySkillResource, type CapabilitySourceIdentity } from '../src/mcp-capability-views/server.js';
import type { TokenVerifier } from '@prefecthq/fastmcp-ts/server';
import { connectMcpCatalog } from '../mcp-catalog-agent/catalog.js';
import { createMcpCatalogAgent, buildMcpCatalogSystemPrompt, mcpCatalogAgentServersFromEnv } from '../mcp-catalog-agent/agent.js';
import { mcpCatalogAgentServerConfigFromEnv } from '../mcp-catalog-agent/server.js';
import { SessionRecorder } from '../mcp-catalog-agent/session.js';
import { buildMcpToolName } from '../mcp-catalog-agent/tool-names.js';

const bearerA = 'fixture-bearer-alpha-secret';
const bearerB = 'fixture-bearer-beta-secret';
const appCheck = 'fixture-app-check-secret';
const llmKey = 'fixture-llm-api-key-secret';

function toolName(input: { source: CapabilitySourceIdentity; operationId: string; method: string; path: string; duplicateOperationId: boolean }): string {
  return buildMcpToolName({ hierarchy: input.source.hierarchy, operationId: input.operationId, method: input.method, path: input.path, duplicateOperationId: input.duplicateOperationId });
}

function accessTokenVerifier(): TokenVerifier {
  return {
    async verify(token) {
      if (![bearerA, bearerB].includes(token)) throw new Error('Invalid fixture token');
      return { token, scopes: [], claims: {} };
    },
  };
}

async function startFixtureUpstream(marker: string, options: { barrierCount?: number; echoAuthorization?: boolean } = {}) {
  const requests: Array<{ method: string; url: string; authorization?: string; appCheck?: string; body: unknown }> = [];
  let arrivals = 0;
  let releaseBarrier = () => {};
  const barrier = new Promise<void>(resolve => { releaseBarrier = resolve; });
  const server = createServer(async (request, response) => {
    const chunks: Buffer[] = [];
    for await (const chunk of request) chunks.push(Buffer.from(chunk));
    const raw = Buffer.concat(chunks).toString('utf8');
    let body: unknown;
    try { body = raw ? JSON.parse(raw) : null; } catch { body = raw; }
    requests.push({
      method: request.method ?? 'GET',
      url: request.url ?? '/',
      authorization: request.headers.authorization,
      appCheck: request.headers['x-app-check-token'] as string | undefined,
      body,
    });
    if (options.barrierCount) {
      arrivals++;
      if (arrivals === options.barrierCount) releaseBarrier();
      await barrier;
    }
    response.writeHead(200, { 'content-type': 'application/json' }).end(JSON.stringify({ result: options.echoAuthorization ? request.headers.authorization ?? '' : 'fixture-ok', marker }));
  });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const address = server.address();
  assert.ok(address && typeof address !== 'string');
  return {
    requests,
    url: 'http://127.0.0.1:' + address.port,
    async close() {
      server.closeAllConnections();
      await new Promise<void>(resolve => server.close(() => resolve()));
    },
  };
}

async function writeFixtureOpenApi(directory: string): Promise<string> {
  const path = join(directory, 'fixture-openapi.json');
  await writeFile(path, JSON.stringify({
    openapi: '3.0.3',
    info: { title: 'Fixture MCP', version: '1' },
    paths: {
      '/items/{companyId}': {
        post: {
          operationId: 'CreateCatalogFixtureController_handle',
          description: 'Cria um registro no fixture usando o schema íntegro.',
          parameters: [
            { name: 'companyId', in: 'path', required: true, schema: { type: 'string', pattern: '^company-[a-z0-9-]+$' } },
            { name: 'x-app-check-token', in: 'header', required: true, schema: { type: 'string' } },
          ],
          requestBody: { required: true, content: { 'application/json': { schema: { $ref: '#/components/schemas/FixtureRequest' } } } },
          responses: {
            '200': {
              description: 'Criado',
              content: { 'application/json': { schema: { type: 'object', properties: { result: { type: 'string' }, marker: { type: 'string' } } } } },
            },
          },
        },
      },
    },
    components: {
      schemas: {
        FixtureRequest: { type: 'object', required: ['filter'], additionalProperties: false, properties: { filter: { $ref: '#/components/schemas/FixtureFilter' } } },
        FixtureFilter: {
          type: 'object', required: ['status', 'tags', 'settings'], additionalProperties: false,
          properties: {
            status: { type: 'string', enum: ['open', 'closed'] },
            tags: { type: 'array', items: { type: 'string', pattern: '^tag-[a-z]+$' } },
            settings: { $ref: '#/components/schemas/FixtureSettings' },
          },
        },
        FixtureSettings: { type: 'object', required: ['enabled'], additionalProperties: false, properties: { enabled: { type: 'boolean' } } },
      },
    },
  }));
  return path;
}

async function startFixtureViews(openapiPath: string, upstreamUrl: string, options: { requestTokenVerifier?: TokenVerifier; skillResources?: CapabilitySkillResource[] } = {}) {
  const server = new CapabilityViewsServer({
    sources: [{
      id: 'fixture-source',
      hierarchy: { domain: 'gruvi', service: 'bff', module: 'fixture' },
      openapiPath,
      upstreamBaseUrl: upstreamUrl,
      headerEnv: { 'x-app-check-token': 'MCP_APP_CHECK_TOKEN' },
    }],
    requestTokenVerifier: options.requestTokenVerifier ?? accessTokenVerifier(),
    skillResources: options.skillResources,
    resolvedSourceHeaders: { 'fixture-source': { 'x-app-check-token': appCheck } },
    toolNameBuilder: input => toolName(input),
    host: '127.0.0.1',
    port: 0,
  });
  await server.start();
  return server;
}

test('servidor inicia sem token de usuário; o cliente troca Bearer por request em tools e Resources', async () => {
  const directory = await mkdtemp(join(tmpdir(), 'mcp-request-bearer-'));
  let upstream: Awaited<ReturnType<typeof startFixtureUpstream>> | undefined;
  let server: CapabilityViewsServer | undefined;
  let catalog: Awaited<ReturnType<typeof connectMcpCatalog>> | undefined;
  try {
    const activeUpstream = await startFixtureUpstream('request-bearer-marker', { echoAuthorization: true });
    upstream = activeUpstream;
    const openapiPath = await writeFixtureOpenApi(directory);
    const serverEnv = {
      UPSTREAM_BASE_URL: activeUpstream.url,
      UPSTREAM_AUTH_MODE: 'request_bearer',
      MCP_VIEWS_OPENAPI_PATH: openapiPath,
    };
    assert.equal('B2C_CLIENT_KEYCLOAK_TOKEN' in serverEnv, false);
    const serverConfig = capabilityViewsConfigFromEnv(serverEnv);
    const skillResources: CapabilitySkillResource[] = [{
      uri: 'skill://gruvi/bff/fixture/request-bearer',
      name: 'request-bearer-skill',
      description: 'Skill fixture',
      mimeType: 'text/markdown',
      hierarchy: { domain: 'gruvi', service: 'bff', module: 'fixture' },
      metadata: { hierarchy: { domain: 'gruvi', service: 'bff', module: 'fixture' } },
      read: async () => 'conteúdo da Skill',
    }];
    const activeServer = await startFixtureViews(openapiPath, activeUpstream.url, { requestTokenVerifier: serverConfig.requestTokenVerifier, skillResources });
    server = activeServer;
    const endpoint = new URL('/mcp/gruvi/bff/fixture', activeServer.url);
    assert.equal((await fetch(endpoint)).status, 401);
    assert.equal((await fetch(endpoint, { headers: { authorization: 'Basic malformed' } })).status, 401);
    assert.equal((await fetch(endpoint, { headers: { authorization: 'Bearer invalid token' } })).status, 401);
    assert.equal(activeUpstream.requests.length, 0);
    await assert.rejects(connectMcpCatalog([{ name: 'fixture', url: endpoint.href, tokenEnv: 'MISSING_TOKEN' }], {}), /Bearer válido/);

    let activeToken = bearerA;
    const tokensRead: string[] = [];
    const activeCatalog = await connectMcpCatalog([{ name: 'fixture', url: endpoint.href }], {}, {
      getBearerToken: () => activeToken,
      onBearerToken: token => tokensRead.push(token),
    });
    catalog = activeCatalog;
    const definition = activeCatalog.list()[0]!;
    assert.equal((await activeCatalog.listResources()).length, 1);
    activeToken = bearerB;
    assert.equal((await activeCatalog.listResources()).length, 1);
    const skill = await activeCatalog.readResource('fixture', 'skill://gruvi/bff/fixture/request-bearer');
    const skillContent = skill.contents[0];
    assert.ok(skillContent && 'text' in skillContent);
    assert.equal(skillContent.text, 'conteúdo da Skill');
    const entry = activeCatalog.get(definition.name)!;
    const result = await entry.client.callTool({
      name: definition.name,
      arguments: { companyId: 'company-1', filter: { status: 'open', tags: ['tag-one'], settings: { enabled: true } } },
    });
    assert.ok(tokensRead.length >= 6);
    const firstB = tokensRead.indexOf(bearerB);
    assert.ok(firstB >= 0);
    assert.ok(tokensRead.slice(0, firstB).every(token => token === bearerA));
    assert.ok(tokensRead.slice(firstB).every(token => token === bearerB));
    assert.deepEqual(activeUpstream.requests.map(request => request.authorization), ['Bearer ' + bearerB]);
    assert.ok(!JSON.stringify(result).includes(bearerA));
    assert.ok(!JSON.stringify(result).includes(bearerB));

    const executionsUrl = new URL('/api/executions', activeServer.url);
    executionsUrl.searchParams.set('view', '/mcp/gruvi/bff/fixture');
    const executions = await (await fetch(executionsUrl)).json() as { executions: Array<{ id: string }> };
    assert.equal(executions.executions.length, 1);
    const traceUrl = new URL('/api/execution', activeServer.url);
    traceUrl.searchParams.set('view', '/mcp/gruvi/bff/fixture');
    traceUrl.searchParams.set('id', executions.executions[0]!.id);
    const trace = await (await fetch(traceUrl)).text();
    assert.ok(!trace.includes(bearerA));
    assert.ok(!trace.includes(bearerB));
  } finally {
    await catalog?.close();
    await server?.close();
    await upstream?.close();
    await rm(directory, { recursive: true });
  }
});

test('requests MCP concorrentes preservam Bearer A e B até o BFF e redigem as respostas', async () => {
  const directory = await mkdtemp(join(tmpdir(), 'mcp-request-bearer-parallel-'));
  let upstream: Awaited<ReturnType<typeof startFixtureUpstream>> | undefined;
  let server: CapabilityViewsServer | undefined;
  let catalogA: Awaited<ReturnType<typeof connectMcpCatalog>> | undefined;
  let catalogB: Awaited<ReturnType<typeof connectMcpCatalog>> | undefined;
  try {
    const activeUpstream = await startFixtureUpstream('parallel-marker', { barrierCount: 2, echoAuthorization: true });
    upstream = activeUpstream;
    const openapiPath = await writeFixtureOpenApi(directory);
    const activeServer = await startFixtureViews(openapiPath, activeUpstream.url);
    server = activeServer;
    const endpoint = new URL('/mcp/gruvi/bff/fixture', activeServer.url).href;
    const envA = { B2C_CLIENT_KEYCLOAK_TOKEN: bearerA };
    const envB = { B2C_CLIENT_KEYCLOAK_TOKEN: bearerB };
    catalogA = await connectMcpCatalog([{ name: 'fixture', url: endpoint, tokenEnv: 'B2C_CLIENT_KEYCLOAK_TOKEN' }], envA);
    catalogB = await connectMcpCatalog([{ name: 'fixture', url: endpoint, tokenEnv: 'B2C_CLIENT_KEYCLOAK_TOKEN' }], envB);
    const name = catalogA.list()[0]!.name;
    const args = { companyId: 'company-1', filter: { status: 'open', tags: ['tag-one'], settings: { enabled: true } } };
    const [resultA, resultB] = await Promise.all([
      catalogA.get(name)!.client.callTool({ name, arguments: args }),
      catalogB.get(name)!.client.callTool({ name, arguments: args }),
    ]);
    assert.deepEqual(activeUpstream.requests.map(request => request.authorization).sort(), ['Bearer ' + bearerA, 'Bearer ' + bearerB].sort());
    assert.ok(!JSON.stringify(resultA).includes(bearerA));
    assert.ok(!JSON.stringify(resultA).includes(bearerB));
    assert.ok(!JSON.stringify(resultB).includes(bearerA));
    assert.ok(!JSON.stringify(resultB).includes(bearerB));
  } finally {
    await catalogA?.close();
    await catalogB?.close();
    await server?.close();
    await upstream?.close();
    await rm(directory, { recursive: true });
  }
});

function responseFor(id: string, output: unknown[]) {
  return {
    id: 'resp_' + id,
    object: 'response',
    created_at: Math.floor(Date.now() / 1000),
    model: 'gpt-6-luna-fixture',
    status: 'completed',
    output,
    usage: {
      input_tokens: 13,
      output_tokens: 5,
      total_tokens: 18,
      input_tokens_details: { cached_tokens: 3, cache_write_tokens: 2 },
      output_tokens_details: { reasoning_tokens: 1 },
    },
  };
}

function functionCall(id: string, name: string, args: unknown) {
  return { id: 'fc_' + id, call_id: 'call_' + id, type: 'function_call', status: 'completed', name, arguments: JSON.stringify(args) };
}

async function withTimeout<T>(promise: Promise<T>, message: string): Promise<T> {
  let timer: ReturnType<typeof setTimeout> | undefined;
  try {
    return await Promise.race([promise, new Promise<T>((_resolve, reject) => { timer = setTimeout(() => reject(new Error(message)), 3000); })]);
  } finally {
    if (timer) clearTimeout(timer);
  }
}

test('Naming canônico separa acrônimos, PascalCase e duplicatas sem depender da ordem', () => {
  assert.equal(buildMcpToolName({
    hierarchy: { domain: 'Gruvi', service: 'BFF', module: 'Building_operation' },
    operationId: 'CreateVisitController_handle',
  }), 'gruvi.bff.building-operation.create-visit-controller-handle');
  assert.equal(buildMcpToolName({ hierarchy: { domain: 'API Core' }, operationId: 'FetchJSONSchema' }), 'api-core.fetch-json-schema');
  const first = buildMcpToolName({ hierarchy: { domain: 'gruvi', service: 'bff', module: 'mobile' }, operationId: 'GetParametersController_handle', method: 'GET', path: '/mobile/{persona}/companies/{companyId}/communications-parameters', duplicateOperationId: true });
  const second = buildMcpToolName({ hierarchy: { domain: 'gruvi', service: 'bff', module: 'mobile' }, operationId: 'GetParametersController_handle', method: 'GET', path: '/mobile/{persona}/companies/{companyId}/ticket-parameters', duplicateOperationId: true });
  assert.notEqual(first, second);
  assert.equal(first, buildMcpToolName({ hierarchy: { domain: 'gruvi', service: 'bff', module: 'mobile' }, operationId: 'GetParametersController_handle', method: 'GET', path: '/mobile/{persona}/companies/{companyId}/communications-parameters', duplicateOperationId: true }));
  assert.throws(() => buildMcpToolName({ hierarchy: { domain: 'x'.repeat(60) }, operationId: 'y'.repeat(70) }), /128 caracteres/);
  assert.throws(() => buildMcpToolName({ hierarchy: { domain: '' }, operationId: 'x' }), /inválidos/);
});

test('Colisão residual após normalização falha explicitamente no startup', async () => {
  const directory = await mkdtemp(join(tmpdir(), 'mcp-catalog-collision-'));
  const openapiPath = join(directory, 'collision.json');
  await writeFile(openapiPath, JSON.stringify({
    openapi: '3.0.3',
    info: { title: 'Collision', version: '1' },
    paths: {
      '/one': { get: { operationId: 'FetchFixtureItem', responses: { '200': { description: 'ok' } } } },
      '/two': { get: { operationId: 'fetch_fixture_item', responses: { '200': { description: 'ok' } } } },
    },
  }));
  const server = new CapabilityViewsServer({
    sources: [{ id: 'collision-source', hierarchy: { domain: 'gruvi', service: 'bff', module: 'fixture' }, openapiPath, upstreamBaseUrl: 'http://127.0.0.1:1' }],
    requestTokenVerifier: accessTokenVerifier(),
    toolNameBuilder: input => toolName(input),
    host: '127.0.0.1',
    port: 0,
  });
  try {
    await assert.rejects(server.start(), /Colisão de tool MCP/);
  } finally {
    await server.close();
    await rm(directory, { recursive: true });
  }
});

test('Catálogo vem do tools/list real: 138 nomes únicos, descrição integral e prompt inicial sem schemas', async () => {
  const env = {
    UPSTREAM_BASE_URL: 'http://127.0.0.1:1',
    UPSTREAM_AUTH_MODE: 'request_bearer',
    MCP_VIEWS_OPENAPI_PATH: 'building-operation.json',
    MCP_VIEWS_MOBILE_OPENAPI_PATH: 'mobile.json',
    MCP_VIEWS_MOBILE_APP_CHECK_TOKEN: appCheck,
  };
  const agentEnv = { ...env, B2C_CLIENT_KEYCLOAK_TOKEN: bearerA };
  const config = mcpCatalogAgentServerConfigFromEnv(env);
  assert.equal(config.port, 3337);
  const server = new CapabilityViewsServer({ ...config, port: 0 });
  await server.start();
  let catalog: Awaited<ReturnType<typeof connectMcpCatalog>> | undefined;
  try {
    const activeCatalog = await connectMcpCatalog([{ name: 'bff', url: new URL('/mcp/gruvi/bff', server.url).href, tokenEnv: 'B2C_CLIENT_KEYCLOAK_TOKEN' }], agentEnv);
    catalog = activeCatalog;
    const definitions = activeCatalog.list();
    assert.equal(definitions.length, 138);
    assert.equal(new Set(definitions.map(tool => tool.name)).size, 138);
    assert.ok(definitions.every(tool => /^[a-z0-9]+(?:-[a-z0-9]+)*(?:\.[a-z0-9]+(?:-[a-z0-9]+)*)*$/.test(tool.name)));
    const duplicates = definitions.filter(tool => tool.name.startsWith('gruvi.bff.mobile.get-parameters-controller-handle-'));
    assert.equal(duplicates.length, 2);
    assert.ok(duplicates.every(tool => tool.name.length <= 128));
    for (const definition of definitions) {
      const original = server.registry?.capabilities.find(capability => capability.toolName === definition.name)?.definition;
      assert.equal(definition.description, original?.description);
    }
    const light = activeCatalog.lightweight();
    assert.deepEqual(Object.keys(light[0]!).sort(), ['description', 'name']);
    const prompt = buildMcpCatalogSystemPrompt({ companyId: '5629', phone: '5511999999999' }, light);
    assert.ok(prompt.includes('gruvi'));
    assert.ok(!/"inputSchema"\s*:/.test(prompt));
    assert.ok(!prompt.includes(JSON.stringify(definitions[0]!.inputSchema)));
    const known = activeCatalog.get(definitions[0]!.name);
    assert.equal(known?.definition.name, definitions[0]!.name);
    assert.equal(activeCatalog.get('gruvi.fake.unknown.operation'), undefined);
    activeCatalog.invalidate();
    assert.throws(() => activeCatalog.list(), /CATALOG_INVALIDATED/);
  } finally {
    await catalog?.close();
    await server.close();
  }
});

test('Responses envia apenas as duas tools fixas; inspect/call valida schemas e mantém tokens isolados', async () => {
  const directory = await mkdtemp(join(tmpdir(), 'mcp-catalog-agent-flow-'));
  const marker = 'fixture-payload-' + 'M'.repeat(32000);
  let upstream: Awaited<ReturnType<typeof startFixtureUpstream>> | undefined;
  let server: CapabilityViewsServer | undefined;
  let agentA: Awaited<ReturnType<typeof createMcpCatalogAgent>> | undefined;
  let agentB: Awaited<ReturnType<typeof createMcpCatalogAgent>> | undefined;
  try {
  const activeUpstream = await startFixtureUpstream(marker);
  upstream = activeUpstream;
  const openapiPath = await writeFixtureOpenApi(directory);
  const activeServer = await startFixtureViews(openapiPath, activeUpstream.url);
  server = activeServer;
  const endpoint = new URL('/mcp/gruvi/bff/fixture', activeServer.url).href;
  const requests: Array<Record<string, any>> = [];
  const outputs = [
    [functionCall('inspect-1', 'inspect_mcp_tool', { name: 'gruvi.bff.fixture.create-catalog-fixture-controller-handle' })],
    [functionCall('call-1', 'call_mcp_tool', {
      name: 'gruvi.bff.fixture.create-catalog-fixture-controller-handle',
      arguments: { companyId: 'company-1', filter: { status: 'open', tags: ['tag-one'], settings: { enabled: true } } },
    })],
    [{ id: 'msg-final', type: 'message', role: 'assistant', status: 'completed', content: [{ type: 'output_text', text: 'O registro foi criado no fixture.', annotations: [] }] }],
  ];
  const fetchImpl: typeof fetch = async (_input, init) => {
    const body = JSON.parse(String(init?.body)) as Record<string, any>;
    requests.push(body);
    const output = outputs[requests.length - 1];
    if (!output) throw new Error('O mock recebeu mais inferências que as três esperadas.');
    return new Response(JSON.stringify(responseFor('flow-' + requests.length, output)), { status: 200, headers: { 'content-type': 'application/json' } });
  };
  const envA = { B2C_CLIENT_KEYCLOAK_TOKEN: bearerA, MCP_APP_CHECK_TOKEN: appCheck, LLM_LUNA_GATEWAY_TOKEN: llmKey, CONCIERGE_COMPANY_ID: 'company-1' };
  const envB = { ...envA, B2C_CLIENT_KEYCLOAK_TOKEN: bearerB };
  let currentTokenA = bearerA;
  const activeAgentA = await createMcpCatalogAgent({
    servers: [{ name: 'fixture', url: endpoint, tokenEnv: 'B2C_CLIENT_KEYCLOAK_TOKEN' }],
    getBearerToken: () => currentTokenA,
    env: envA,
    sessionPath: join(directory, 'session-a.json'),
    fetch: fetchImpl,
  });
  agentA = activeAgentA;
  const activeAgentB = await createMcpCatalogAgent({
    servers: [{ name: 'fixture', url: endpoint, tokenEnv: 'B2C_CLIENT_KEYCLOAK_TOKEN' }],
    env: envB,
    sessionPath: join(directory, 'session-b.json'),
    fetch: fetchImpl,
  });
  agentB = activeAgentB;
    assert.equal(activeAgentA.catalog.list().length, 1);
    assert.deepEqual(activeAgentA.tools.map(tool => tool.name), ['inspect_mcp_tool', 'call_mcp_tool']);
    const definition = activeAgentA.catalog.list()[0]!;
    assert.equal(definition.description, 'Cria um registro no fixture usando o schema íntegro.');
    const schema = definition.inputSchema as { properties: Record<string, unknown>; required: string[] };
    assert.ok(schema.required.includes('companyId'));
    assert.ok(Object.hasOwn(schema.properties, 'filter'));
    const callTool = activeAgentA.tools.find(tool => tool.name === 'call_mcp_tool')!;
    const invalid = await callTool.invoke({ name: definition.name, arguments: { companyId: 'company-1', filter: { status: 'pending', tags: ['NOT_TAG'], settings: { enabled: true }, unexpected: 1 } } });
    assert.match(typeof invalid === 'string' ? invalid : JSON.stringify(invalid), /ARGUMENT_VALIDATION_FAILED/);
    const invented = await callTool.invoke({ name: 'fake.operation', arguments: { companyId: 'company-1', filter: { status: 'open', tags: [], settings: { enabled: true } } } });
    assert.match(typeof invented === 'string' ? invented : JSON.stringify(invented), /TOOL_NOT_FOUND/);
    assert.equal(activeUpstream.requests.length, 0);

    const result = await activeAgentA.invoke('Crie o registro aberto com a tag indicada.');
    assert.equal(result.messages.at(-1)?.text, 'O registro foi criado no fixture.');
    assert.equal(requests.length, 3);
    for (const request of requests) {
      assert.equal(request.tools.filter((definition: { type: string }) => definition.type === 'function').length, 2);
      assert.deepEqual(request.tools.map((definition: { name: string }) => definition.name).sort(), ['call_mcp_tool', 'inspect_mcp_tool']);
      assert.ok(request.tools.every((definition: { strict?: boolean }) => definition.strict === false));
      const callDefinition = request.tools.find((definition: { name: string }) => definition.name === 'call_mcp_tool');
      assert.equal(callDefinition.parameters.properties.arguments.additionalProperties, true);
    }
    assert.ok(!JSON.stringify(requests[0]!.input).includes(JSON.stringify(definition.inputSchema)));
    assert.ok(JSON.stringify(requests[1]!.input).includes('inputSchema'));
    const inspectMessage = result.messages.find(message => ToolMessage.isInstance(message) && message.name === 'inspect_mcp_tool');
    const callMessage = result.messages.find(message => ToolMessage.isInstance(message) && message.name === 'call_mcp_tool');
    assert.ok(inspectMessage && ToolMessage.isInstance(inspectMessage));
    assert.ok(callMessage && ToolMessage.isInstance(callMessage));
    assert.equal((inspectMessage.artifact as any).tool.description, definition.description);
    assert.deepEqual(Object.keys((inspectMessage.artifact as any).tool.metadata).sort(), ['hierarchy', 'operationId', 'source']);
    assert.ok(!('path' in (inspectMessage.artifact as any).tool.metadata));
    assert.ok(callMessage.artifact && typeof callMessage.artifact === 'object' && 'content' in callMessage.artifact);
    assert.equal(activeUpstream.requests.length, 1);
    assert.deepEqual(activeUpstream.requests.map(request => request.authorization).sort(), ['Bearer ' + bearerA]);
    assert.ok(activeUpstream.requests.every(request => request.appCheck === appCheck));
    assert.deepEqual(activeUpstream.requests[0]?.body, { filter: { status: 'open', tags: ['tag-one'], settings: { enabled: true } } });
    assert.equal(activeUpstream.requests[0]?.url, '/items/company-1');

    const validArgs = { companyId: 'company-2', filter: { status: 'closed', tags: ['tag-two'], settings: { enabled: false } } };
    currentTokenA = bearerB;
    const sameConnectionBCall = await activeAgentA.tools.find(tool => tool.name === 'call_mcp_tool')!.invoke({ name: definition.name, arguments: validArgs });
    assert.ok(sameConnectionBCall);
    assert.deepEqual(activeUpstream.requests.map(request => request.authorization).sort(), ['Bearer ' + bearerA, 'Bearer ' + bearerB].sort());
    const agentBCall = await activeAgentB.tools.find(tool => tool.name === 'call_mcp_tool')!.invoke({ name: definition.name, arguments: validArgs });
    assert.ok(agentBCall);
    assert.deepEqual(activeUpstream.requests.map(request => request.authorization).sort(), ['Bearer ' + bearerA, 'Bearer ' + bearerB, 'Bearer ' + bearerB].sort());
    assert.ok(activeUpstream.requests.every(request => request.appCheck === appCheck));

    const session = JSON.parse(await readFile(activeAgentA.recorder.path, 'utf8')) as any;
    assert.equal(session.snapshot.tools.length, 1);
    assert.equal(session.snapshot.lightweightCatalog.length, 1);
    assert.equal(session.totals.inferences, 3);
    assert.equal(session.totals.responsesWithUsage, 3);
    assert.equal(session.totals.cachedInputTokens, 9);
    assert.equal(session.totals.cacheWriteTokens, 6);
    assert.equal(session.totals.reasoningTokens, 3);
    assert.equal(session.costEstimate.status, 'complete');
    assert.ok(Math.abs(session.costEstimate.estimatedAmountUsd - 0.00001074) < 1e-12);
    assert.ok(session.events.some((event: any) => event.type === 'call_mcp_tool' && JSON.stringify(event.data).includes(marker)));
    assert.ok(!JSON.stringify(session).includes(bearerA));
    assert.ok(!JSON.stringify(session).includes(bearerB));
    assert.ok(!JSON.stringify(session).includes(appCheck));
    assert.ok(!JSON.stringify(session).includes(llmKey));
    assert.ok(JSON.stringify(session).includes('cache_write_tokens'));
  } finally {
    await agentA?.close('failed').catch(() => undefined);
    await agentB?.close('failed').catch(() => undefined);
    await server?.close();
    await upstream?.close();
    await rm(directory, { recursive: true });
  }
});

test('AbortSignal de encerramento cancela uma requisição HTTP do modelo em andamento', async () => {
  const directory = await mkdtemp(join(tmpdir(), 'mcp-catalog-abort-'));
  let upstream: Awaited<ReturnType<typeof startFixtureUpstream>> | undefined;
  let server: CapabilityViewsServer | undefined;
  let agent: Awaited<ReturnType<typeof createMcpCatalogAgent>> | undefined;
  const externalAbort = new AbortController();
  try {
    upstream = await startFixtureUpstream('unused');
    const openapiPath = await writeFixtureOpenApi(directory);
    server = await startFixtureViews(openapiPath, upstream.url);
    const endpoint = new URL('/mcp/gruvi/bff/fixture', server.url).href;
    let signalSeen: AbortSignal | undefined;
    let markStarted: () => void = () => undefined;
    const started = new Promise<void>(resolve => { markStarted = resolve; });
    const slowFetch: typeof fetch = async (input, init) => {
      signalSeen = init?.signal ?? (input instanceof Request ? input.signal : undefined);
      markStarted();
      return new Promise<Response>((_resolve, reject) => {
        const abort = () => {
          const error = new Error('mock request aborted');
          error.name = 'AbortError';
          reject(error);
        };
        if (signalSeen?.aborted) abort();
        else signalSeen?.addEventListener('abort', abort, { once: true });
      });
    };
    const runtime = await createMcpCatalogAgent({
      servers: [{ name: 'fixture', url: endpoint, tokenEnv: 'B2C_CLIENT_KEYCLOAK_TOKEN' }],
      env: { B2C_CLIENT_KEYCLOAK_TOKEN: bearerA, MCP_APP_CHECK_TOKEN: appCheck, LLM_LUNA_GATEWAY_TOKEN: llmKey, CONCIERGE_COMPANY_ID: 'company-1' },
      sessionPath: join(directory, 'session.json'),
      fetch: slowFetch,
    });
    agent = runtime;
    const invocation = runtime.invoke('Aguarde a resposta do serviço.', externalAbort.signal);
    const settledInvocation = invocation.then(
      () => ({ status: 'resolved' as const }),
      error => ({ status: 'rejected' as const, error }),
    );
    await withTimeout(started, 'A chamada HTTP não iniciou.');
    await withTimeout(runtime.close('failed'), 'O encerramento não cancelou a chamada HTTP.');
    const outcome = await settledInvocation;
    assert.equal(outcome.status, 'rejected');
    assert.ok(signalSeen);
    assert.equal(signalSeen.aborted, true);
    const session = JSON.parse(await readFile(runtime.recorder.path, 'utf8')) as any;
    assert.ok(session.events.some((event: any) => event.type === 'llm_http_error'));
  } finally {
    externalAbort.abort();
    await agent?.close('failed').catch(() => undefined);
    await server?.close();
    await upstream?.close();
    await rm(directory, { recursive: true });
  }
});

test('Session preserva schemas e campos de negócio semelhantes a credenciais e redige chaves de credencial exatas', async () => {
  const directory = await mkdtemp(join(tmpdir(), 'mcp-catalog-session-redaction-'));
  const path = join(directory, 'session.json');
  try {
    const recorder = await SessionRecorder.start({ path, model: 'fixture-model' });
    const schema = {
      name: 'gruvi.bff.visits.get-authorizations',
      inputSchema: {
        type: 'object',
        properties: {
          authorizations: { type: 'array', items: { $ref: '#/$defs/AuthorizationDTO' } },
          AuthorizationDTO: { $ref: '#/$defs/AuthorizationDTO' },
          authorizationCode: { type: 'string' },
          GetAuthorizationDTO: { $ref: '#/$defs/GetAuthorizationDTO' },
          FetchAuthorizationBlockDTO: { $ref: '#/$defs/FetchAuthorizationBlockDTO' },
          FetchAuthorizationUnityDTO: { $ref: '#/$defs/FetchAuthorizationUnityDTO' },
          FetchAuthorizationCondominiumDTO: { $ref: '#/$defs/FetchAuthorizationCondominiumDTO' },
          FetchAuthorizationVehicleDTO: { $ref: '#/$defs/FetchAuthorizationVehicleDTO' },
        },
        $defs: {
          AuthorizationDTO: { type: 'object', properties: { authorizationCode: { type: 'string' } } },
          GetAuthorizationDTO: { type: 'object', properties: { authorizationCode: { type: 'string' } } },
          FetchAuthorizationBlockDTO: { type: 'object', properties: { authorizationCode: { type: 'string' } } },
          FetchAuthorizationUnityDTO: { type: 'object', properties: { authorizationCode: { type: 'string' } } },
          FetchAuthorizationCondominiumDTO: { type: 'object', properties: { authorizationCode: { type: 'string' } } },
          FetchAuthorizationVehicleDTO: { type: 'object', properties: { authorizationCode: { type: 'string' } } },
        },
      },
    };
    await recorder.setSnapshot([schema], [{ name: schema.name, description: 'Busca autorizações' }]);
    await recorder.record('credential_fields', {
      authorization: 'credential-authorization',
      proxyAuthorization: 'credential-proxy-authorization',
      cookie: 'credential-cookie',
      setCookie: 'credential-set-cookie',
      apiKey: 'credential-api-key',
      xApiKey: 'credential-x-api-key',
      accessToken: 'credential-access-token',
      refreshToken: 'credential-refresh-token',
      idToken: 'credential-id-token',
      token: 'credential-token',
      clientSecret: 'credential-client-secret',
      password: 'credential-password',
      xAppCheckToken: 'credential-x-app-check-token',
      appCheckToken: 'credential-app-check-token',
      b2cClientKeycloakToken: 'credential-b2c-token',
      authorizations: ['view', 'approve'],
      AuthorizationDTO: { authorizationCode: 'business-authorization-code' },
      authorizationCode: 'business-authorization-code-2',
    });
    await recorder.finish('completed');

    const saved = JSON.parse(await readFile(path, 'utf8')) as any;
    assert.deepEqual(saved.snapshot.tools, [schema]);
    assert.deepEqual(saved.events[1].data, {
      authorization: '[REDACTED]',
      proxyAuthorization: '[REDACTED]',
      cookie: '[REDACTED]',
      setCookie: '[REDACTED]',
      apiKey: '[REDACTED]',
      xApiKey: '[REDACTED]',
      accessToken: '[REDACTED]',
      refreshToken: '[REDACTED]',
      idToken: '[REDACTED]',
      token: '[REDACTED]',
      clientSecret: '[REDACTED]',
      password: '[REDACTED]',
      xAppCheckToken: '[REDACTED]',
      appCheckToken: '[REDACTED]',
      b2cClientKeycloakToken: '[REDACTED]',
      authorizations: ['view', 'approve'],
      AuthorizationDTO: { authorizationCode: 'business-authorization-code' },
      authorizationCode: 'business-authorization-code-2',
    });
  } finally {
    await rm(directory, { recursive: true });
  }
});

test('Session JSON reseta no novo runtime e serializa eventos concorrentes sem truncar payloads', async () => {
  const directory = await mkdtemp(join(tmpdir(), 'mcp-catalog-session-'));
  const path = join(directory, 'session.json');
  try {
    await writeFile(path, JSON.stringify({ previous: true }));
    const first = await SessionRecorder.start({ path, model: 'fixture-model', secrets: ['known-secret'] });
    const shared = { value: 'preservado' };
    const marker = 'large-' + 'L'.repeat(40000);
    await Promise.all(Array.from({ length: 12 }, (_, index) => first.record('parallel_event', { index, marker, left: shared, right: shared })));
    await first.record('redaction_event', { embedded: JSON.stringify({ access_token: 'unknown-secret', xAppCheckToken: 'app-secret' }), usage: { input_tokens: 11, cached_tokens: 4 } });
    const fetched = await first.createFetch(async () => new Response(JSON.stringify({ usage: { input_tokens: 100, output_tokens: 20, input_tokens_details: { cached_tokens: 30, cache_write_tokens: 40 }, output_tokens_details: { reasoning_tokens: 5 } }, content: 'response intact' }), { status: 200 }))(new Request('http://mock.invalid', { method: 'POST', body: JSON.stringify({ marker, apiKey: 'known-secret' }) }));
    assert.equal((await fetched.json() as any).content, 'response intact');
    await first.finish('completed');
    const saved = JSON.parse(await readFile(path, 'utf8')) as any;
    assert.equal(saved.events.length, 15);
    assert.deepEqual(saved.events.map((event: any) => event.sequence), Array.from({ length: 15 }, (_, index) => index + 1));
    assert.ok(saved.events.filter((event: any) => event.type === 'parallel_event').every((event: any) => JSON.stringify(event).includes(marker)));
    assert.deepEqual(saved.events[0].data.left, saved.events[0].data.right);
    assert.equal(saved.totals.inputTokens, 100);
    assert.equal(saved.totals.cachedInputTokens, 30);
    assert.equal(saved.totals.cacheWriteTokens, 40);
    assert.equal(saved.totals.reasoningTokens, 5);
    assert.equal(saved.totals.responsesWithUsage, 1);
    assert.equal(saved.costEstimate.status, 'complete');
    assert.ok(Math.abs(saved.costEstimate.estimatedAmountUsd - 0.0000183) < 1e-12);
    assert.ok(!JSON.stringify(saved).includes('known-secret'));
    assert.ok(!JSON.stringify(saved).includes('unknown-secret'));
    assert.ok(!JSON.stringify(saved).includes('app-secret'));
    const second = await SessionRecorder.start({ path, model: 'fixture-model' });
    assert.notEqual(second.sessionId, first.sessionId);
    assert.equal(JSON.parse(await readFile(path, 'utf8')).events.length, 0);
    await second.finish('completed');
  } finally {
    await rm(directory, { recursive: true });
  }
});
