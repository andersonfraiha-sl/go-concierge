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
  agente: 'Agentes',
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
  agent: 'Agente',
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

/**
 * Quem não edita não vê o código técnico (`name`): sem título, a ferramenta
 * aparece como "Ferramenta sem nome".
 */
export const tituloDe = (ferramenta: Ferramenta, editavel = true): string =>
  ferramenta.titulo || (editavel ? ferramenta.name : 'Ferramenta sem nome');

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

/** Valores aceitos pelo servidor em `conexao.metodo`, com o rótulo que o Studio mostra. */
export const METODOS: Record<string, string> = {
  '': '—',
  GET: 'GET',
  POST: 'POST',
  PUT: 'PUT',
  PATCH: 'PATCH',
  DELETE: 'DELETE',
  MCP: 'Servidor de ferramentas',
};

export const TIPOS_DE_AJUSTE: Record<string, string> = {
  string: 'Texto curto',
  text: 'Texto longo',
  number: 'Número',
  boolean: 'Sim/não',
  select: 'Escolha',
};

export type Cabecalho = { chave: string; valor: string };

export function cabecalhosEmLista(headers: Record<string, string> | undefined): Cabecalho[] {
  return Object.entries(headers ?? {}).map(([chave, valor]) => ({ chave, valor }));
}

export function listaEmCabecalhos(lista: readonly Cabecalho[]): Record<string, string> | undefined {
  const comChave = lista
    .map((cabecalho) => ({ chave: cabecalho.chave.trim(), valor: cabecalho.valor }))
    .filter((cabecalho) => cabecalho.chave !== '');
  return comChave.length === 0
    ? undefined
    : Object.fromEntries(comChave.map((cabecalho) => [cabecalho.chave, cabecalho.valor]));
}

const ValorSchema = z.json();
export type Valor = z.infer<typeof ValorSchema>;

/** Converte o texto digitado em "Padrão" para o tipo do ajuste; texto vazio é "sem padrão". */
export function padraoDoAjuste(tipo: string | undefined, texto: string): Valor | undefined {
  if (texto === '') return undefined;
  if (tipo === 'number') {
    const numero = Number(texto);
    return Number.isFinite(numero) ? numero : undefined;
  }
  if (tipo === 'boolean') {
    if (texto === 'sim') return true;
    if (texto === 'não') return false;
    return undefined;
  }
  return texto;
}

const PADRAO_DO_CODIGO = /^[a-z0-9_-]+(\.[a-z0-9_-]+)+$/;
const MENSAGEM_CODIGO_INVALIDO = 'Use grupo.acao, em minúsculas, como reservas.criar.';

export function validarCodigo(codigo: string): string | null {
  return PADRAO_DO_CODIGO.test(codigo) ? null : MENSAGEM_CODIGO_INVALIDO;
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
        options: d.type === 'select' ? d.options.map((o) => o.trim()).filter((o) => o !== '') : [],
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
