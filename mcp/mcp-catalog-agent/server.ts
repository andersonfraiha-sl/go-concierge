import { pathToFileURL } from 'node:url';
import { CapabilityViewsServer, capabilityViewsConfigFromEnv, type CapabilitySourceIdentity, type CapabilityViewsServerConfig } from '../src/mcp-capability-views/server.js';
import { buildMcpToolName } from './tool-names.js';

export function mcpCatalogAgentServerConfigFromEnv(env: NodeJS.ProcessEnv = process.env) {
  const config = capabilityViewsConfigFromEnv({
    ...env,
    MCP_VIEWS_HOST: env.MCP_CATALOG_AGENT_MCP_HOST ?? '127.0.0.1',
    MCP_VIEWS_PORT: env.MCP_CATALOG_AGENT_MCP_PORT ?? '3337',
  });
  const toolNameBuilder: NonNullable<CapabilityViewsServerConfig['toolNameBuilder']> = ({ source, ...tool }: { source: CapabilitySourceIdentity; operationId: string; method: string; path: string; duplicateOperationId: boolean }) => buildMcpToolName({ hierarchy: source.hierarchy, ...tool });
  return { ...config, toolNameBuilder };
}

export async function runMcpCatalogAgentServer() {
  const server = await new CapabilityViewsServer(mcpCatalogAgentServerConfigFromEnv()).start();
  console.info('MCP Catalog Agent iniciado com ' + (server.registry?.capabilities.length ?? 0) + ' tools publicáveis.');
  console.info('Explorer local: ' + server.inspectorUrl);
  console.info('Views MCP: ' + server.viewUrls.join(', '));
  const close = () => { void server.close().finally(() => { process.exitCode = 0; }); };
  process.once('SIGINT', close);
  process.once('SIGTERM', close);
  return server;
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  runMcpCatalogAgentServer().catch((error: unknown) => {
    console.error(error instanceof Error ? error.message : String(error));
    process.exitCode = 1;
  });
}
