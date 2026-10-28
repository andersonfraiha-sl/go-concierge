import { randomUUID } from 'node:crypto';
import { createInterface } from 'node:readline/promises';
import { stdin, stdout } from 'node:process';
import { pathToFileURL } from 'node:url';
import { ChatOpenAI } from '@langchain/openai';
import type { BaseChatModel } from '@langchain/core/language_models/chat_models';
import { HumanMessage } from '@langchain/core/messages';
import { MemorySaver } from '@langchain/langgraph';
import { createAgent } from 'langchain';
import { buildSystemPrompt, type ConciergeContext } from '../src/concierge-agent.js';
import { connectMcpCatalog, type McpCatalogServerConfig } from './catalog.js';
import { SessionRecorder } from './session.js';
import { createMcpCatalogTools } from './tools.js';

export interface McpCatalogAgentOptions extends ConciergeContext {
  model?: BaseChatModel;
  servers?: McpCatalogServerConfig[];
  env?: NodeJS.ProcessEnv;
  sessionPath?: string;
  fetch?: typeof fetch;
}

export function mcpCatalogAgentServersFromEnv(env: NodeJS.ProcessEnv = process.env): McpCatalogServerConfig[] {
  const value = env.MCP_CATALOG_AGENT_SERVERS_JSON;
  if (!value?.trim()) return [{
    name: 'gruviBff',
    url: env.MCP_CATALOG_AGENT_MCP_URL?.trim() || 'http://127.0.0.1:3337/mcp/gruvi/bff',
    tokenEnv: 'B2C_CLIENT_KEYCLOAK_TOKEN',
  }];
  let parsed: unknown;
  try { parsed = JSON.parse(value); } catch { throw new Error('MCP_CATALOG_AGENT_SERVERS_JSON deve ser um JSON válido'); }
  if (!Array.isArray(parsed) || parsed.length === 0) throw new Error('MCP_CATALOG_AGENT_SERVERS_JSON deve conter ao menos um servidor');
  for (const entry of parsed) {
    if (!entry || typeof entry !== 'object' || Array.isArray(entry) || Object.keys(entry).some(key => !['name', 'url', 'tokenEnv'].includes(key))) throw new Error('A configuração MCP aceita somente name, url e tokenEnv');
  }
  return parsed as McpCatalogServerConfig[];
}

function credentialValues(env: NodeJS.ProcessEnv, servers: McpCatalogServerConfig[]): string[] {
  const envSecrets = Object.entries(env).flatMap(([key, value]) => /token|secret|password|api[_-]?key|app[_-]?check/i.test(key) && value?.trim() ? [value.trim()] : []);
  const serverSecrets = servers.flatMap(server => server.tokenEnv && env[server.tokenEnv]?.trim() ? [env[server.tokenEnv]!.trim()] : []);
  return [...new Set([...envSecrets, ...serverSecrets])];
}

export function createCatalogAgentLunaModel(env: NodeJS.ProcessEnv, recorder: SessionRecorder, fetchImpl: typeof fetch = fetch): BaseChatModel {
  const token = env.LLM_LUNA_GATEWAY_TOKEN?.trim() || env.LLM_GATEWAY_TOKEN?.trim();
  if (!token) throw new Error('Configure LLM_LUNA_GATEWAY_TOKEN ou LLM_GATEWAY_TOKEN para iniciar o agent Luna.');
  if (!env.CONCIERGE_COMPANY_ID?.trim()) throw new Error('Configure CONCIERGE_COMPANY_ID para usar o gateway OpenAI.');
  return new ChatOpenAI({
    model: env.LLM_LUNA_MODEL?.trim() || 'gpt-6-luna',
    apiKey: token,
    maxTokens: 32000,
    modelKwargs: { reasoning: { effort: 'medium' }, store: false },
    supportsStrictToolCalling: false,
    configuration: {
      baseURL: env.LLM_LUNA_OPENAI_URL?.trim() || 'https://kong-ai-gtw.idp.nexus.superlogica.net/v1',
      defaultHeaders: { 'x-company-id': 'internal', 'x-origem': 'workspace' },
      fetch: recorder.createFetch(fetchImpl),
    },
    useResponsesApi: true,
    maxRetries: 1,
  });
}

export function buildMcpCatalogSystemPrompt(context: ConciergeContext, catalog: Array<{ name: string; description?: string }>): string {
  const base = buildSystemPrompt({});
  const contextMarker = 'Contexto operacional fornecido pela aplicação:\n';
  const fixedRules = base.slice(0, base.indexOf(contextMarker)).replace(
    'Suas capacidades são exclusivamente as ferramentas descobertas no servidor MCP. Use seus nomes, descrições e schemas para decidir como atender. Não há catálogo de skills local nem roteador intermediário.',
    'Suas capacidades são as tools MCP listadas no catálogo leve abaixo. O modelo recebe apenas as tools fixas inspect_mcp_tool e call_mcp_tool. Escolha pelo nome exato do catálogo, inspecione o schema antes da primeira chamada e só execute com call_mcp_tool depois de validar os dados necessários. Nunca invente nomes de tools nem execute HTTP diretamente.',
  );
  const catalogText = catalog.map(item => `- ${item.name}\n  ${item.description ?? ''}`).join('\n');
  const contextJson = JSON.stringify({ companyId: context.companyId ?? null, phone: context.phone ?? null });
  return `${fixedRules}Catálogo MCP leve, com descrições integrais:\n${catalogText}\n\nA definição completa e o inputSchema aparecem somente depois de inspect_mcp_tool. Envie o objeto arguments sem transformar em texto JSON. Trate respostas ok:false e isError como falhas.\n${contextMarker}${contextJson}`;
}

function messageText(content: unknown): string {
  if (typeof content === 'string') return content;
  if (!Array.isArray(content)) return '';
  return content.flatMap((block: unknown) => {
    if (typeof block !== 'object' || block === null || !('text' in block)) return [];
    return typeof block.text === 'string' ? [block.text] : [];
  }).join('\n');
}

export async function createMcpCatalogAgent(options: McpCatalogAgentOptions = {}) {
  const env = options.env ?? process.env;
  const servers = options.servers ?? mcpCatalogAgentServersFromEnv(env);
  const modelName = env.LLM_LUNA_MODEL?.trim() || 'gpt-6-luna';
  const recorder = await SessionRecorder.start({ path: options.sessionPath, model: modelName, secrets: credentialValues(env, servers) });
  let catalog: Awaited<ReturnType<typeof connectMcpCatalog>> | undefined;
  try {
    const model = options.model ?? createCatalogAgentLunaModel(env, recorder, options.fetch);
    catalog = await connectMcpCatalog(servers, env);
    const definitions = catalog.list();
    const lightweightCatalog = catalog.lightweight();
    await recorder.setSnapshot(definitions, lightweightCatalog);
    const tools = createMcpCatalogTools(catalog, recorder);
    const agent = createAgent({
      model,
      tools: [...tools],
      systemPrompt: buildMcpCatalogSystemPrompt({
        companyId: options.companyId ?? env.CONCIERGE_COMPANY_ID,
        phone: options.phone ?? env.CONCIERGE_PHONE,
      }, lightweightCatalog),
      checkpointer: new MemorySaver(),
    });
    let closed = false;
    let closePromise: Promise<void> | undefined;
    const shutdown = new AbortController();
    let invocationQueue = Promise.resolve();
    return {
      agent,
      tools,
      catalog,
      recorder,
      threadId: recorder.threadId,
      async invoke(message: string, signal?: AbortSignal) {
        if (closed) throw new Error('O runtime MCP já foi encerrado.');
        if (!message.trim()) throw new Error('Informe uma mensagem para o concierge.');
        const turnId = randomUUID();
        const runSignal = signal ? AbortSignal.any([shutdown.signal, signal]) : shutdown.signal;
        const run = invocationQueue.then(async () => recorder.runTurn(turnId, async () => {
          await recorder.record('user_message', { message }, { threadId: recorder.threadId, turnId });
          try {
            const result = await agent.invoke({ messages: [new HumanMessage(message)] }, { signal: runSignal, configurable: { thread_id: recorder.threadId }, recursionLimit: 30 });
            await recorder.record('assistant_messages', { messages: result.messages }, { threadId: recorder.threadId, turnId });
            return result;
          } catch (error) {
            await recorder.record('agent_turn_error', { error }, { threadId: recorder.threadId, turnId });
            throw error;
          }
        }));
        invocationQueue = run.then(() => undefined, () => undefined);
        return run;
      },
      async close(status: 'completed' | 'failed' = 'completed') {
        if (!closePromise) {
          closed = true;
          shutdown.abort(new Error('O runtime do agent foi encerrado.'));
          closePromise = (async () => {
            await invocationQueue;
            let closeError: unknown;
            try { await catalog!.close(); } catch (error) { closeError = error; }
            await recorder.finish(status === 'failed' || closeError ? 'failed' : 'completed');
            if (closeError) throw closeError;
          })();
        }
        await closePromise;
      },
    };
  } catch (error) {
    await catalog?.close().catch(() => undefined);
    await recorder.record('agent_startup_error', { error }).catch(() => undefined);
    await recorder.finish('failed').catch(() => undefined);
    throw error;
  }
}

export async function runMcpCatalogAgentCli() {
  const concierge = await createMcpCatalogAgent();
  const terminal = createInterface({ input: stdin, output: stdout });
  let terminalClosed = false;
  terminal.once('close', () => { terminalClosed = true; });
  const handleSignal = () => {
    terminal.close();
    void concierge.close('failed').catch(() => undefined);
  };
  process.once('SIGINT', handleSignal);
  process.once('SIGTERM', handleSignal);
  stdout.write('MCP Catalog Agent conectado com ' + concierge.catalog.list().length + ' capabilities e ' + concierge.tools.length + ' tools fixas. Use /tools para listar e /sair para encerrar.\n');
  let status: 'completed' | 'failed' = 'completed';
  try {
    while (!terminalClosed) {
      let message: string;
      try { message = (await terminal.question('Você: ')).trim(); } catch (error) { if (terminalClosed) break; throw error; }
      if (!message) continue;
      if (message === '/sair') break;
      if (message === '/tools') {
        stdout.write(concierge.catalog.lightweight().map(tool => `${tool.name}: ${tool.description ?? ''}`).join('\n') + '\n');
        continue;
      }
      try {
        const result = await concierge.invoke(message);
        stdout.write('Concierge: ' + messageText(result.messages.at(-1)?.content) + '\n');
      } catch (error) {
        status = 'failed';
        stdout.write('Falha no atendimento: ' + (error instanceof Error ? error.message : String(error)) + '\n');
      }
    }
  } finally {
    terminal.close();
    process.removeListener('SIGINT', handleSignal);
    process.removeListener('SIGTERM', handleSignal);
    await concierge.close(status);
  }
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  runMcpCatalogAgentCli().catch((error: unknown) => {
    console.error(error instanceof Error ? error.message : String(error));
    process.exitCode = 1;
  });
}
