import assert from 'node:assert/strict';
import { request } from 'node:http';
import { mkdtemp, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { test } from 'node:test';
import { buildUtcpManual, type UtcpSourceConfig } from '../utcp/manual.js';
import { startUtcpManualServer } from '../utcp/manual-server.js';

test('manual publica as OpenAPIs locais com contratos de unidades e encomendas', async () => {
  const snapshot = await buildUtcpManual({ env: { UPSTREAM_BASE_URL: 'https://bff.example.test/api' } });
  assert.equal(snapshot.sources.length, 2);
  for (const operation of ['GetCompanyByDocumentController_handle', 'FetchUnitPackagesController_handle']) {
    const selected = snapshot.manual.tools.find(tool => tool.name.endsWith('.' + operation));
    assert.ok(selected, operation);
    const template = selected.tool_call_template as unknown as { url: string; headers: Record<string, string> };
    assert.ok(template.url.startsWith('https://bff.example.test/api/'));
    assert.equal(template.headers.authorization, 'Bearer ${B2C_TOKEN}');
    assert.ok(selected.description.length > 0);
    assert.equal(selected.inputs.additionalProperties, false);
  }
});

test('manual preserva descrições e refs, separa fontes e informa exclusões', async t => {
  const directory = await mkdtemp(join(tmpdir(), 'concierge-utcp-'));
  t.after(() => rm(directory, { recursive: true, force: true }));
  const openapiPath = join(directory, 'fixture.json');
  await writeFile(openapiPath, JSON.stringify({
    openapi: '3.0.3', info: { title: 'Fixture', version: '1' },
    paths: {
      '/items/{id}': { get: {
        operationId: 'list', summary: 'Lista itens', description: 'Contrato completo',
        parameters: [{ name: 'id', in: 'path', required: true, schema: { type: 'string', format: 'uuid' } }],
        responses: { '200': { description: 'OK', content: { 'application/json': { schema: { $ref: '#/components/schemas/Item' } } } } },
      } },
      '/private': { get: { operationId: 'private', 'x-mcp': false, responses: {} } },
      '/upload': { post: { operationId: 'upload', requestBody: { content: { 'multipart/form-data': { schema: { type: 'object' } } } }, responses: {} } },
    },
    components: { schemas: { Item: { type: 'object', properties: { name: { type: 'string', nullable: true } } } } },
  }));
  const sources: UtcpSourceConfig[] = [
    { id: 'building-operation', kind: 'bff', openapiPath, upstreamBaseUrl: 'https://bff.example.test' },
    { id: 'parecer-pasta', kind: 'parecer-pasta', openapiPath, upstreamBaseUrl: 'https://parecer.example.test' },
  ];
  const snapshot = await buildUtcpManual({ sources });
  assert.equal(snapshot.manual.tools.length, 2);
  assert.equal(snapshot.excludedByPolicy, 2);
  assert.equal(snapshot.skipped.filter(item => item.reason.includes('multipart')).length, 2);
  assert.equal(new Set(snapshot.manual.tools.map(tool => tool.name)).size, 2);
  for (const tool of snapshot.manual.tools) {
    assert.equal(tool.description, 'Lista itens\n\nContrato completo');
    assert.ok(tool.inputs.required?.includes('id'));
    assert.ok(JSON.stringify(tool.outputs).includes('null'));
  }
  const parecer = snapshot.manual.tools.find(tool => tool.tags.includes('parecer-pasta'))!;
  const template = parecer.tool_call_template as unknown as { headers: Record<string, string> };
  assert.equal(template.headers.authorization, 'Bearer ${PARECER_GATEWAY_TOKEN}');
  assert.equal(template.headers['x-company-id'], '${PARECER_COMPANY_ID}');
});

test('publicador serve manual e índice apenas por GET e não executa operações', async t => {
  const snapshot = await buildUtcpManual({ env: { UPSTREAM_BASE_URL: 'https://bff.example.test' } });
  const server = await startUtcpManualServer({ port: 0, snapshot });
  t.after(() => server.close());
  const response = await fetch(server.url);
  assert.equal(response.status, 200);
  const manual = await response.json() as { tools: unknown[] };
  assert.equal(manual.tools.length, snapshot.manual.tools.length);
  const index = await fetch(server.indexUrl).then(result => result.json()) as { counts: { published: number } };
  assert.equal(index.counts.published, manual.tools.length);
  assert.equal((await fetch(server.url, { method: 'POST', body: '{}' })).status, 405);
  assert.equal((await fetch(server.indexUrl + 'invoke')).status, 404);
  assert.equal((await fetch(server.url, { headers: { origin: 'https://external.example.test' } })).status, 403);
  const invalidHostStatus = await new Promise<number | undefined>((resolve, reject) => {
    const req = request(server.url, { headers: { host: 'external.example.test' } }, response => {
      response.resume();
      resolve(response.statusCode);
    });
    req.on('error', reject);
    req.end();
  });
  assert.equal(invalidHostStatus, 403);
});
