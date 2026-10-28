import { tool, type ToolRuntime } from '@langchain/core/tools';
import { isInputRequiredResult } from '@modelcontextprotocol/client';
import { McpCatalog } from './catalog.js';
import { SessionRecorder } from './session.js';

const inspectInputSchema = {
  type: 'object',
  properties: { name: { type: 'string' } },
  required: ['name'],
  additionalProperties: false,
} as const;

const callInputSchema = {
  type: 'object',
  properties: {
    name: { type: 'string' },
    arguments: { type: 'object', additionalProperties: true },
  },
  required: ['name', 'arguments'],
  additionalProperties: false,
} as const;

function metadataFor(definition: ReturnType<McpCatalog['list']>[number]): Record<string, unknown> {
  const sourceMeta = definition._meta;
  const metadata: Record<string, unknown> = {};
  if (sourceMeta && typeof sourceMeta === 'object') {
    if (typeof sourceMeta.source === 'string') metadata.source = sourceMeta.source;
    if (typeof sourceMeta.operationId === 'string') metadata.operationId = sourceMeta.operationId;
    if (sourceMeta.hierarchy && typeof sourceMeta.hierarchy === 'object') {
      const hierarchy = sourceMeta.hierarchy as Record<string, unknown>;
      metadata.hierarchy = Object.fromEntries(['domain', 'service', 'module'].flatMap(key => typeof hierarchy[key] === 'string' ? [[key, hierarchy[key]]] : []));
    }
  }
  return metadata;
}

function annotationsFor(definition: ReturnType<McpCatalog['list']>[number]): Record<string, unknown> | undefined {
  const source = definition.annotations as Record<string, unknown> | undefined;
  if (!source) return undefined;
  const annotations = Object.fromEntries(['readOnlyHint', 'destructiveHint', 'idempotentHint', 'openWorldHint', 'title'].flatMap(key => source[key] !== undefined ? [[key, source[key]]] : []));
  return Object.keys(annotations).length ? annotations : undefined;
}

function resultText(value: unknown): string {
  return JSON.stringify(value);
}

function getSignal(runtime: ToolRuntime): AbortSignal | undefined {
  const config = runtime.config as (typeof runtime.config & { signal?: AbortSignal }) | undefined;
  return config?.signal;
}

export function createMcpCatalogTools(catalog: McpCatalog, recorder: SessionRecorder) {
  const inspect = tool(async (raw, runtime: ToolRuntime) => {
    const { name } = raw as { name: string };
    let response: Record<string, unknown>;
    try {
      catalog.assertValid();
      const entry = catalog.get(name);
      response = entry ? {
        ok: true,
        tool: {
          name: entry.definition.name,
          ...(typeof entry.definition.description === 'string' ? { description: entry.definition.description } : {}),
          inputSchema: entry.definition.inputSchema,
          ...(entry.definition.outputSchema ? { outputSchema: entry.definition.outputSchema } : {}),
          ...(annotationsFor(entry.definition) ? { annotations: annotationsFor(entry.definition) } : {}),
          metadata: metadataFor(entry.definition),
        },
      } : { ok: false, error: { code: 'TOOL_NOT_FOUND', message: 'O nome não existe no catálogo MCP.' } };
    } catch (error) {
      response = { ok: false, error: { code: error instanceof Error && error.message.startsWith('CATALOG_INVALIDATED') ? 'CATALOG_INVALIDATED' : 'CATALOG_UNAVAILABLE', message: error instanceof Error ? error.message : 'Catálogo indisponível.' } };
    }
    await recorder.record('inspect_mcp_tool', { name, result: response }, { toolCallId: runtime.toolCallId });
    return [resultText(response), response] as const;
  }, {
    name: 'inspect_mcp_tool',
    description: 'Inspeciona uma capability MCP pelo nome exato e retorna sua descrição integral e schemas publicados.',
    schema: inspectInputSchema,
    responseFormat: 'content_and_artifact',
  });

  const call = tool(async (raw, runtime: ToolRuntime) => {
    const { name, arguments: args } = raw as { name: string; arguments: Record<string, unknown> };
    let response: Record<string, unknown>;
    let entry;
    let artifact: unknown;
    try {
      catalog.assertValid();
      entry = catalog.get(name);
    } catch (error) {
      response = { ok: false, error: { code: 'CATALOG_INVALIDATED', message: error instanceof Error ? error.message : 'Recrie o runtime do catálogo MCP.' } };
      await recorder.record('call_mcp_tool', { name, arguments: args, result: response }, { toolCallId: runtime.toolCallId, validation: 'not_attempted' });
      return [resultText(response), response] as const;
    }
    if (!entry) {
      response = { ok: false, error: { code: 'TOOL_NOT_FOUND', message: 'O nome não existe no catálogo MCP.' } };
      await recorder.record('call_mcp_tool', { name, arguments: args, result: response }, { toolCallId: runtime.toolCallId, validation: 'not_attempted' });
      return [resultText(response), response] as const;
    }
    const validation = entry.validate(args);
    if (!validation.valid) {
      response = { ok: false, error: { code: 'ARGUMENT_VALIDATION_FAILED', message: validation.errorMessage } };
      await recorder.record('call_mcp_tool', { name, arguments: args, result: response }, { toolCallId: runtime.toolCallId, validation: 'failed' });
      return [resultText(response), response] as const;
    }
    try {
      const result: unknown = await entry.client.callTool({ name, arguments: args }, { signal: getSignal(runtime), timeout: 30000 });
      if (isInputRequiredResult(result)) {
        response = { ok: false, error: { code: 'INPUT_REQUIRED_NOT_SUPPORTED', message: 'A tool MCP solicitou interação de entrada, fluxo ainda não suportado pelo agent.' } };
        artifact = result;
        await recorder.record('call_mcp_tool', { name, arguments: args, result: response, rawMcpResult: result }, { toolCallId: runtime.toolCallId, validation: 'passed' });
        return [resultText(response), artifact] as const;
      }
      const toolResult = result as Awaited<ReturnType<typeof entry.client.callTool>>;
      response = {
        ok: toolResult.isError !== true,
        result: {
          content: toolResult.content,
          ...(toolResult.structuredContent !== undefined ? { structuredContent: toolResult.structuredContent } : {}),
          ...(toolResult.isError !== undefined ? { isError: toolResult.isError } : {}),
        },
        ...(toolResult.isError === true ? { error: { code: 'MCP_TOOL_ERROR', message: 'A tool MCP retornou isError=true.' } } : {}),
      };
      artifact = toolResult;
      await recorder.record('call_mcp_tool', { name, arguments: args, result: response, rawMcpResult: toolResult }, { toolCallId: runtime.toolCallId, validation: 'passed' });
    } catch (error) {
      if (getSignal(runtime)?.aborted || (error instanceof Error && error.name === 'AbortError')) {
        await recorder.record('call_mcp_tool_error', { name, arguments: args, error: 'MCP_CALL_CANCELLED', originalError: error }, { toolCallId: runtime.toolCallId, validation: 'passed' });
        throw error;
      }
      const inputRequired = isInputRequiredResult(error) || (error instanceof Error && /input[_ -]?required/i.test(error.name + ' ' + error.message));
      response = inputRequired
        ? { ok: false, error: { code: 'INPUT_REQUIRED_NOT_SUPPORTED', message: 'A tool MCP solicitou interação de entrada, fluxo ainda não suportado pelo agent.' } }
        : { ok: false, error: { code: 'MCP_CALL_FAILED', message: 'Falha de transporte ou protocolo ao chamar a tool MCP.' } };
      await recorder.record('call_mcp_tool', { name, arguments: args, result: response, originalError: error }, { toolCallId: runtime.toolCallId, validation: 'passed' });
    }
    return [resultText(response), artifact ?? response] as const;
  }, {
    name: 'call_mcp_tool',
    description: 'Executa uma tool do catálogo MCP pelo nome exato após validar os argumentos contra o inputSchema publicado.',
    schema: callInputSchema,
    responseFormat: 'content_and_artifact',
  });

  return [inspect, call] as const;
}
