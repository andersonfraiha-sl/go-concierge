import { z } from 'zod';
import { type Ferramenta, FerramentasSchema } from '@/catalogo/contratos';
import { apiEnviar } from '@/shared/api/http';

export type Ferramentas = z.infer<typeof FerramentasSchema>;

export const CLASSES = [
  'leitura',
  'escrita',
  'conhecimento',
  'memoria',
  'acao',
  'agente',
  'form',
] as const;
export type Classe = (typeof CLASSES)[number];

export const GRUPOS: Record<Classe, string> = {
  leitura: 'Consultas',
  escrita: 'Gravações',
  conhecimento: 'Conteúdos',
  memoria: 'Memória',
  acao: 'Ações',
  agente: 'Assistentes',
  form: 'Formulários',
};

/** Valores aceitos pelo servidor em `kind`, com o rótulo que o Studio mostra. */
export const TIPOS: Record<string, string> = {
  api: 'Sistema externo',
  database: 'Banco de dados',
  knowledge: 'Conteúdo',
  memory: 'Memória',
  action: 'Ação',
  activity: 'Atividade',
  agent: 'Concierge',
  form: 'Formulário',
};

const CLASSE_DO_TIPO: Record<string, Classe> = {
  knowledge: 'conhecimento',
  memory: 'memoria',
  action: 'acao',
  activity: 'acao',
  agent: 'agente',
  form: 'form',
};

export function classeDe(ferramenta: Ferramenta): Classe {
  return CLASSE_DO_TIPO[ferramenta.kind] ?? (ferramenta.write ? 'escrita' : 'leitura');
}

export const tituloDe = (ferramenta: Ferramenta): string => ferramenta.titulo || ferramenta.name;

export const mantidaFora = (ferramenta: Ferramenta): boolean => ferramenta.origem === 'mcp';

export function ferramentaNova(): Ferramenta {
  return {
    name: '',
    kind: 'api',
    visibility: 'private',
    write: false,
    description: '',
    params: [],
    saida: [],
    definicoes: [],
  };
}

export type Amostra = { ok: true; valor: Ferramenta['amostra'] } | { ok: false };

export function lerAmostra(texto: string): Amostra {
  if (texto.trim() === '') return { ok: true, valor: undefined };
  try {
    const lida = z.json().safeParse(JSON.parse(texto));
    return lida.success ? { ok: true, valor: lida.data } : { ok: false };
  } catch {
    // JSON.parse só lança SyntaxError: o texto digitado não é JSON.
    return { ok: false };
  }
}

export function textoDaAmostra(ferramenta: Ferramenta): string {
  return ferramenta.amostra === undefined || ferramenta.amostra === null
    ? ''
    : JSON.stringify(ferramenta.amostra, null, 2);
}

const aparado = (texto: string | undefined): string | undefined => texto?.trim() || undefined;

/** Tira linhas sem nome e espaços das pontas; o que o servidor recebe é só o que tem conteúdo. */
export function paraGravar(rascunho: Ferramenta, amostra: Amostra & { ok: true }): Ferramenta {
  const conexao = rascunho.conexao;
  const temConexao =
    conexao !== undefined &&
    Boolean(
      aparado(conexao.metodo) ||
        aparado(conexao.url) ||
        aparado(conexao.corpo) ||
        Object.keys(conexao.headers ?? {}).length,
    );
  return {
    ...rascunho,
    name: rascunho.name.trim(),
    titulo: aparado(rascunho.titulo),
    description: rascunho.description.trim(),
    output: aparado(rascunho.output),
    params: rascunho.params
      .filter((p) => p.name.trim() !== '')
      .map((p) => ({ ...p, name: p.name.trim() })),
    saida: rascunho.saida
      .filter((s) => s.name.trim() !== '')
      .map((s) => ({ ...s, name: s.name.trim() })),
    definicoes: rascunho.definicoes
      .filter((d) => d.name.trim() !== '')
      .map((d) => ({
        ...d,
        name: d.name.trim(),
        options: d.options.map((o) => o.trim()).filter((o) => o !== ''),
      })),
    conexao: temConexao ? conexao : undefined,
    amostra: amostra.valor,
  };
}

export function salvarFerramenta(c: {
  original: string;
  recurso: Ferramenta;
}): Promise<Ferramentas> {
  return apiEnviar('PUT', '/api/nexus/recurso', c, FerramentasSchema);
}

export function removerFerramenta(nome: string): Promise<Ferramentas> {
  return apiEnviar(
    'DELETE',
    `/api/nexus/recurso?${new URLSearchParams({ nome })}`,
    undefined,
    FerramentasSchema,
  );
}
