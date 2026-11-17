import { z } from 'zod';
import { type Ferramenta, FerramentasSchema } from '@/catalogo/contratos';
import { apiEnviar } from '@/shared/api/http';
import type { SkillResumo } from '@/skills/contratos';

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

/** Skills (já carregadas) que declaram usar a ferramenta pelo código dela. */
export function skillsQueUsam(skills: readonly SkillResumo[], ferramenta: string): SkillResumo[] {
  return skills.filter((skill) => skill.recursos.includes(ferramenta));
}

export const contagemDeSkills = (skills: readonly SkillResumo[], ferramenta: string): number =>
  skillsQueUsam(skills, ferramenta).length;

export const rotuloDaContagem = (quantidade: number): string =>
  `${quantidade} skill${quantidade === 1 ? '' : 's'}`;

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

function aparado(texto: string | undefined): string | undefined {
  return texto?.trim() || undefined;
}

type Conexao = NonNullable<Ferramenta['conexao']>;

function conexaoPreenchida(conexao: Conexao | undefined): boolean {
  if (conexao === undefined) return false;
  const temCabecalho = Object.keys(conexao.headers ?? {}).length > 0;
  return Boolean(
    aparado(conexao.metodo) || aparado(conexao.url) || aparado(conexao.corpo) || temCabecalho,
  );
}

const comNomeAparado = <T extends { name: string }>(itens: readonly T[]): T[] =>
  itens
    .filter((item) => item.name.trim() !== '')
    .map((item) => ({ ...item, name: item.name.trim() }));

type Definicao = Ferramenta['definicoes'][number];

function definicaoParaGravar(definicao: Definicao): Definicao {
  const opcoes =
    definicao.type === 'select'
      ? definicao.options.map((opcao) => opcao.trim()).filter((opcao) => opcao !== '')
      : [];
  return { ...definicao, options: opcoes };
}

/** Tira linhas sem nome e espaços das pontas; o que o servidor recebe é só o que tem conteúdo. */
export function paraGravar(rascunho: Ferramenta, amostra: Amostra & { ok: true }): Ferramenta {
  return {
    ...rascunho,
    name: rascunho.name.trim(),
    titulo: aparado(rascunho.titulo),
    description: rascunho.description.trim(),
    output: aparado(rascunho.output),
    params: comNomeAparado(rascunho.params),
    saida: comNomeAparado(rascunho.saida),
    definicoes: comNomeAparado(rascunho.definicoes).map(definicaoParaGravar),
    conexao: conexaoPreenchida(rascunho.conexao) ? rascunho.conexao : undefined,
    amostra: amostra.valor,
  };
}

export function salvarFerramenta(pedido: {
  original: string;
  recurso: Ferramenta;
}): Promise<Ferramentas> {
  return apiEnviar('PUT', '/api/nexus/recurso', pedido, FerramentasSchema);
}

export function removerFerramenta(nome: string): Promise<Ferramentas> {
  return apiEnviar(
    'DELETE',
    `/api/nexus/recurso?${new URLSearchParams({ nome })}`,
    undefined,
    FerramentasSchema,
  );
}
