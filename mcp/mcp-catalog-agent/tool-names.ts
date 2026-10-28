import { createHash } from 'node:crypto';
import type { CapabilityHierarchy } from '../src/mcp-capability-views/server.js';

export interface McpToolNameInput {
  hierarchy: CapabilityHierarchy;
  operationId: string;
  method?: string;
  path?: string;
  duplicateOperationId?: boolean;
}

export function normalizeToolNameLevel(value: string): string {
  return value
    .trim()
    .replace(/([A-Z]+)([A-Z][a-z])/g, '$1-$2')
    .replace(/([a-z0-9])([A-Z])/g, '$1-$2')
    .replace(/[^A-Za-z0-9]+/g, '-')
    .toLowerCase()
    .replace(/-+/g, '-')
    .replace(/^-|-$/g, '');
}

function duplicateSuffix(method: string, path: string): string {
  const segments = path.split('/').filter(Boolean);
  const literal = segments.reverse().find(segment => !/^\{[^{}]+\}$/.test(segment)) ?? 'route';
  const slug = normalizeToolNameLevel(literal.replace(/[{}]/g, '')) || 'route';
  const normalizedMethod = normalizeToolNameLevel(method) || 'operation';
  const hash = createHash('sha256').update(method.toUpperCase() + ' ' + path).digest('hex').slice(0, 10);
  return `${normalizedMethod}-${slug}-${hash}`;
}

export function buildMcpToolName(input: McpToolNameInput): string {
  const levels = [input.hierarchy.domain, input.hierarchy.service, input.hierarchy.module, input.operationId]
    .filter((value): value is string => value !== undefined)
    .map(normalizeToolNameLevel);
  if (levels.some(level => !level)) throw new Error('Hierarquia ou operationId inválidos para nome MCP');
  if (input.duplicateOperationId) {
    if (!input.method || !input.path) throw new Error('Método e path são obrigatórios para desambiguar operationId duplicado');
    levels[levels.length - 1] += '-' + duplicateSuffix(input.method, input.path);
  }
  const name = levels.join('.');
  if (name.length > 128) throw new Error('Nome de tool MCP excede 128 caracteres: ' + name);
  if (!/^[a-z0-9]+(?:-[a-z0-9]+)*(?:\.[a-z0-9]+(?:-[a-z0-9]+)*)*$/.test(name)) throw new Error('Nome de tool MCP inválido: ' + name);
  return name;
}
