import { createServer, type IncomingMessage, type ServerResponse } from 'node:http';
import { once } from 'node:events';
import { buildUtcpManual, utcpManualIndex, type UtcpManualBuildOptions, type UtcpManualSnapshot } from './manual.js';

export interface UtcpManualServerOptions extends UtcpManualBuildOptions {
  host?: string;
  port?: number;
  snapshot?: UtcpManualSnapshot;
}

function json(response: ServerResponse, status: number, value: unknown): void {
  response.writeHead(status, { 'content-type': 'application/json; charset=utf-8', 'cache-control': 'no-store', 'x-content-type-options': 'nosniff' });
  response.end(JSON.stringify(value));
}

function loopbackHost(host: string): boolean {
  return host === '127.0.0.1' || host === 'localhost' || host === '::1';
}

function requestOrigin(request: IncomingMessage): string | undefined {
  const host = request.headers.host;
  if (!host) return undefined;
  try { return new URL(`http://${host}`).origin; } catch { return undefined; }
}

export async function startUtcpManualServer(options: UtcpManualServerOptions = {}) {
  const env = options.env ?? process.env;
  const host = options.host ?? env.AGENT_UTCP_MANUAL_HOST?.trim() ?? '127.0.0.1';
  const rawPort = options.port ?? Number(env.AGENT_UTCP_MANUAL_PORT ?? 3345);
  if (!loopbackHost(host)) throw new Error('AGENT_UTCP_MANUAL_HOST deve apontar para loopback.');
  if (!Number.isInteger(rawPort) || rawPort < 0 || rawPort > 65535) throw new Error('AGENT_UTCP_MANUAL_PORT inválida.');
  const snapshot = options.snapshot ?? await buildUtcpManual(options);
  const server = createServer((request, response) => {
    const address = server.address();
    const port = address && typeof address !== 'string' ? address.port : rawPort;
    const configuredHost = host.includes(':') ? `[${host}]` : host;
    const expectedHosts = new Set([`${configuredHost}:${port}`, `127.0.0.1:${port}`, `localhost:${port}`, `[::1]:${port}`]);
    if (!request.headers.host || !expectedHosts.has(request.headers.host.toLowerCase())) return json(response, 403, { error: 'Host local inválido.' });
    const expectedOrigin = requestOrigin(request);
    if (request.headers.origin && request.headers.origin !== expectedOrigin) return json(response, 403, { error: 'Origin local inválido.' });
    if (request.method !== 'GET') return json(response, 405, { error: 'Método não permitido.' });
    const path = request.url?.split('?', 1)[0];
    if (request.url === '/utcp') return json(response, 200, snapshot.manual);
    if (path === '/') return json(response, 200, utcpManualIndex(snapshot));
    return json(response, 404, { error: 'Não encontrado.' });
  });
  server.on('clientError', (_error, socket) => { socket.end('HTTP/1.1 400 Bad Request\r\n\r\n'); });
  try {
    server.listen(rawPort, host);
    await once(server, 'listening');
  } catch (error) {
    server.close();
    throw error;
  }
  const address = server.address();
  if (!address || typeof address === 'string') throw new Error('Não foi possível iniciar o publicador UTCP.');
  let closePromise: Promise<void> | undefined;
  return {
    url: `http://${host.includes(':') ? `[${host}]` : host}:${address.port}/utcp`,
    indexUrl: `http://${host.includes(':') ? `[${host}]` : host}:${address.port}/`,
    snapshot,
    async close() {
      if (!closePromise) closePromise = new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
      await closePromise;
    },
  };
}
