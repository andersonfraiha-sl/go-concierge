import { Client, StreamableHTTPClientTransport, type AuthProvider, type JsonSchemaType } from '@modelcontextprotocol/client';
import { AjvJsonSchemaValidator } from '@modelcontextprotocol/client/validators/ajv';

export interface McpCatalogServerConfig {
  name: string;
  url: string;
  tokenEnv?: string;
}

export interface McpCatalogConnectionOptions {
  watchResources?: boolean;
  getBearerToken?: (server: McpCatalogServerConfig) => string | undefined | Promise<string | undefined>;
  onBearerToken?: (token: string) => void;
}

type McpToolDefinition = Awaited<ReturnType<Client['listTools']>>['tools'][number];
type McpResourceDefinition = Awaited<ReturnType<Client['listResources']>>['resources'][number];

export interface McpCatalogEntry {
  serverName: string;
  definition: McpToolDefinition;
  client: Client;
  validate: ReturnType<AjvJsonSchemaValidator['getValidator']>;
}

export class McpCatalog {
  private readonly definitions = new Map<string, McpCatalogEntry>();
  private readonly clients: Array<{ serverName: string; client: Client }> = [];
  private invalidated = false;
  private invalidationReason = 'a lista de tools MCP mudou; recrie o runtime do agent.';
  private closed = false;

  async addClient(serverName: string, client: Client): Promise<void> {
    this.clients.push({ serverName, client });
    const listed = await client.listTools();
    for (const definition of listed.tools) {
      if (this.definitions.has(definition.name)) throw new Error('Nome MCP duplicado entre endpoints: ' + definition.name);
      let validate: McpCatalogEntry['validate'];
      try {
        validate = new AjvJsonSchemaValidator().getValidator(definition.inputSchema as JsonSchemaType);
      } catch (error) {
        throw new Error('inputSchema MCP inválido para ' + definition.name + ': ' + (error instanceof Error ? error.message : 'falha de compilação'));
      }
      this.definitions.set(definition.name, { serverName, definition, client, validate });
    }
    if (!serverName) throw new Error('Nome do servidor MCP inválido');
  }

  list(): McpToolDefinition[] {
    this.assertValid();
    return [...this.definitions.values()].map(entry => entry.definition).sort((left, right) => left.name < right.name ? -1 : left.name > right.name ? 1 : 0);
  }

  get(name: string): McpCatalogEntry | undefined {
    this.assertValid();
    return this.definitions.get(name);
  }

  lightweight(): Array<{ name: string; description?: string }> {
    return this.list().map(tool => ({ name: tool.name, ...(typeof tool.description === 'string' ? { description: tool.description } : {}) }));
  }

  listForServer(serverName: string): McpToolDefinition[] {
    this.assertValid();
    return [...this.definitions.values()].filter(entry => entry.serverName === serverName).map(entry => entry.definition);
  }

  async listResources(): Promise<Array<{ serverName: string; resource: McpResourceDefinition }>> {
    this.assertValid();
    const pages = await Promise.all(this.clients.map(async ({ serverName, client }) => {
      const resources: McpResourceDefinition[] = [];
      let cursor: string | undefined;
      do {
        const page = await client.listResources(cursor ? { cursor } : undefined);
        resources.push(...page.resources);
        cursor = page.nextCursor;
      } while (cursor);
      return resources.map(resource => ({ serverName, resource }));
    }));
    return pages.flat().sort((left, right) => left.serverName.localeCompare(right.serverName) || left.resource.name.localeCompare(right.resource.name));
  }

  async readResource(serverName: string, uri: string, signal?: AbortSignal): Promise<Awaited<ReturnType<Client['readResource']>>> {
    this.assertValid();
    const entry = this.clients.find(client => client.serverName === serverName);
    if (!entry) throw new Error('Servidor MCP de resource desconhecido: ' + serverName);
    return entry.client.readResource({ uri }, { signal, cacheMode: 'bypass' });
  }

  assertValid(): void {
    if (this.closed) throw new Error('O catálogo MCP foi encerrado; crie um novo runtime.');
    if (this.invalidated) throw new Error('CATALOG_INVALIDATED: ' + this.invalidationReason);
  }

  invalidate(reason?: string): void {
    this.invalidated = true;
    if (reason) this.invalidationReason = reason;
  }

  async close(): Promise<void> {
    if (this.closed) return;
    this.closed = true;
    const results = await Promise.allSettled(this.clients.map(({ client }) => client.close()));
    const failure = results.find(result => result.status === 'rejected');
    if (failure?.status === 'rejected') throw failure.reason;
  }
}

function validateServerConfig(server: McpCatalogServerConfig, names: Set<string>): URL {
  if (!server || typeof server !== 'object') throw new Error('Configuração de servidor MCP inválida');
  if (typeof server.name !== 'string' || !/^[a-zA-Z0-9_.-]{1,64}$/.test(server.name) || names.has(server.name)) throw new Error('Nome de servidor MCP inválido ou duplicado');
  names.add(server.name);
  if (Object.keys(server as unknown as Record<string, unknown>).some(key => !['name', 'url', 'tokenEnv'].includes(key))) throw new Error('A configuração MCP aceita somente name, url e tokenEnv');
  if (server.tokenEnv !== undefined && (typeof server.tokenEnv !== 'string' || !/^[A-Z_][A-Z0-9_]*$/.test(server.tokenEnv))) throw new Error('tokenEnv deve ser o nome de uma variável de ambiente');
  let url: URL;
  try { url = new URL(server.url); } catch { throw new Error('URL inválida para o servidor MCP ' + server.name); }
  if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password || url.search || url.hash) throw new Error('URL MCP inválida para o servidor ' + server.name);
  return url;
}

function isLoopback(hostname: string): boolean {
  const host = hostname.toLowerCase().replace(/^\[|\]$/g, '');
  return host === 'localhost' || host === '::1' || /^127(?:\.\d{1,3}){3}$/.test(host);
}

async function readBearerToken(server: McpCatalogServerConfig, env: NodeJS.ProcessEnv, options: McpCatalogConnectionOptions): Promise<string> {
  const value = options.getBearerToken ? await options.getBearerToken(server) : server.tokenEnv ? env[server.tokenEnv] : undefined;
  const token = typeof value === 'string' ? value.trim() : '';
  if (!token || !/^[A-Za-z0-9._~+/-]+=*$/.test(token)) throw new Error('Configure um Bearer válido para autenticar no servidor MCP ' + server.name);
  options.onBearerToken?.(token);
  return token;
}

export async function connectMcpCatalog(servers: McpCatalogServerConfig[], env: NodeJS.ProcessEnv = process.env, options: McpCatalogConnectionOptions = {}): Promise<McpCatalog> {
  if (!servers.length) throw new Error('Configure ao menos um endpoint MCP para o catálogo.');
  const names = new Set<string>();
  const entries = servers.map(server => ({ server, url: validateServerConfig(server, names) }));
  const catalog = new McpCatalog();
  try {
    for (const { server, url } of entries) {
      const hasAuthProvider = Boolean(server.tokenEnv || options.getBearerToken);
      if (hasAuthProvider && !isLoopback(url.hostname) && url.protocol !== 'https:') throw new Error('MCP_URL deve usar HTTPS fora de loopback quando envia Bearer');
      const client = new Client({ name: 'mcp-catalog-agent', version: '1' }, {
        inputRequired: { autoFulfill: false },
        listChanged: {
          tools: { autoRefresh: false, onChanged: () => catalog.invalidate('a lista de tools MCP mudou; recrie o runtime do agent.') },
          ...(options.watchResources ? { resources: { autoRefresh: false, onChanged: () => catalog.invalidate('a lista de resources MCP mudou; recrie o runtime do agent.') } } : {}),
        },
      });
      const authProvider: AuthProvider | undefined = hasAuthProvider ? {
        async token() {
          const token = await readBearerToken(server, env, options);
          if (!isLoopback(url.hostname) && url.protocol !== 'https:') throw new Error('MCP_URL deve usar HTTPS fora de loopback quando envia Bearer');
          return token;
        },
      } : undefined;
      const transport = new StreamableHTTPClientTransport(url, authProvider ? { authProvider } : undefined);
      try {
        await client.connect(transport);
        await catalog.addClient(server.name, client);
      } catch (error) {
        await client.close().catch(() => undefined);
        throw error;
      }
    }
    catalog.assertValid();
    if (!catalog.list().length) throw new Error('O servidor MCP configurado não publicou ferramentas.');
    return catalog;
  } catch (error) {
    await catalog.close().catch(() => undefined);
    throw error;
  }
}
