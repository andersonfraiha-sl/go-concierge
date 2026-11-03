import { useQuery, useQueryClient } from '@tanstack/react-query';
import { z } from 'zod';
import { apiEnviar, apiEnviarBinario, apiGet } from '@/shared/api/http';
import { listaDoGo } from '@/shared/api/listaDoGo';

const consulta = (parametros: Record<string, string>): string =>
  new URLSearchParams(parametros).toString();

// ---- árvore de itens (assistente e skills) ----

export type NoDaArvore = {
  caminho: string;
  nome: string;
  tipo: string;
  descricao: string;
  caso: boolean;
  erros: number;
  avisos: number;
  subagentes: NoDaArvore[];
};

const NoSchema: z.ZodType<NoDaArvore> = z.lazy(() =>
  z.object({
    caminho: z.string(),
    nome: z.string(),
    tipo: z.string(),
    descricao: z.string(),
    caso: z.boolean(),
    erros: z.number(),
    avisos: z.number(),
    subagentes: listaDoGo(NoSchema),
  }),
);

const ArvoreSchema = z.object({ agentes: listaDoGo(NoSchema) });

export function useArvore() {
  return useQuery({
    queryKey: ['avancado', 'arvore'],
    queryFn: () => apiGet('/api/agentes', ArvoreSchema),
    select: (arvore) => achatar(arvore.agentes),
  });
}

export function achatar(nos: NoDaArvore[]): NoDaArvore[] {
  return nos.flatMap((no) => [no, ...achatar(no.subagentes)]);
}

// ---- arquivos ----

export const PASTAS_DE_ARQUIVOS = [
  'rules',
  'workflows',
  'routers',
  'knowledge',
  'assets',
  'evals',
] as const;
export type PastaDeArquivos = (typeof PASTAS_DE_ARQUIVOS)[number];

export const ARQUIVO_DE_IDENTIDADE = 'agent.yaml';
export const ARQUIVO_DE_PERSONA = 'persona.md';
export const ARQUIVO_DE_CASO = 'caso.yaml';

export const PASTA_INICIAL: PastaDeArquivos = 'rules';

export const SUGESTAO_DE_NOME: Record<PastaDeArquivos, string> = {
  rules: 'politicas.md',
  workflows: 'principal.md',
  routers: 'rotas.md',
  knowledge: 'glossary.yaml',
  assets: 'rodape.md',
  evals: 'casos.yaml',
};

export const caminhoNaPasta = (pasta: PastaDeArquivos, nome: string): string => `${pasta}/${nome}`;

// ---- projeção ----

const ProjecaoSchema = z.object({ projecao: z.string(), roteador: z.string() });

export function useProjecao(caminho: string) {
  return useQuery({
    queryKey: ['avancado', 'projecao', caminho],
    queryFn: () => apiGet(`/api/projecao?${consulta({ agente: caminho })}`, ProjecaoSchema),
  });
}

// ---- orquestração ----

export const MODOS_DE_ORQUESTRACAO = ['router', 'planning', 'react'] as const;
export type ModoDeOrquestracao = (typeof MODOS_DE_ORQUESTRACAO)[number];

const ModoSchema = z.enum(MODOS_DE_ORQUESTRACAO);

const AjusteSchema = z.object({
  modo: ModoSchema,
  bind_confidence: z.number(),
  max_itens: z.number(),
  max_iteracoes: z.number(),
});
export type Orquestracao = z.infer<typeof AjusteSchema>;

const OrquestracaoLidaSchema = z.object({
  agente: z.string(),
  arquivo: AjusteSchema,
  efetivo: AjusteSchema,
  ambiente: z.string(),
});
export type OrquestracaoLida = z.infer<typeof OrquestracaoLidaSchema>;

const ORQUESTRACAO = '/api/admin/orquestracao';

export function useOrquestracao() {
  return useQuery({
    queryKey: ['avancado', 'orquestracao'],
    queryFn: () => apiGet(ORQUESTRACAO, OrquestracaoLidaSchema),
  });
}

export function salvarOrquestracao(ajuste: Orquestracao): Promise<OrquestracaoLida> {
  return apiEnviar('PUT', ORQUESTRACAO, ajuste, OrquestracaoLidaSchema);
}

/** Os mesmos limites de adminSalvarOrquestracao (cmd/okf-admin/admin.go). */
export const LIMITES_DE_ORQUESTRACAO = [
  {
    campo: 'bind_confidence',
    rotulo: 'Confiança mínima',
    min: 0.5,
    max: 1,
    passo: 0.05,
    texto: 'entre 0,5 e 1',
  },
  {
    campo: 'max_itens',
    rotulo: 'Itens por plano',
    min: 1,
    max: 10,
    passo: 1,
    texto: 'entre 1 e 10',
  },
  {
    campo: 'max_iteracoes',
    rotulo: 'Passos por pedido',
    min: 1,
    max: 15,
    passo: 1,
    texto: 'entre 1 e 15',
  },
] as const satisfies ReadonlyArray<{
  campo: keyof Orquestracao;
  rotulo: string;
  min: number;
  max: number;
  passo: number;
  texto: string;
}>;

export type LimiteFurado = {
  campo: (typeof LIMITES_DE_ORQUESTRACAO)[number]['campo'];
  mensagem: string;
};

/** Campos fora dos limites, com a mensagem; lista vazia quando o ajuste vale. */
export function validarOrquestracao(ajuste: Orquestracao): LimiteFurado[] {
  return LIMITES_DE_ORQUESTRACAO.filter(({ campo, min, max }) => {
    const valor = ajuste[campo];
    return Number.isNaN(valor) || valor < min || valor > max;
  }).map(({ campo, rotulo, texto }) => ({ campo, mensagem: `${rotulo} deve ficar ${texto}.` }));
}

// ---- importar e exportar ----

const ImportadoSchema = z.object({ instalado: z.string(), arquivos: listaDoGo(z.string()) });
const TIMEOUT_IMPORTACAO_MS = 120_000;

export const urlExportar = (caminho: string): string =>
  `/api/agentes/exportar?${consulta({ agente: caminho })}`;

export function importarZip(
  caminho: string,
  arquivo: File,
): Promise<z.infer<typeof ImportadoSchema>> {
  return apiEnviarBinario(
    'POST',
    `/api/agentes/importar?${consulta({ caminho })}`,
    arquivo,
    'application/zip',
    ImportadoSchema,
    TIMEOUT_IMPORTACAO_MS,
  );
}

/** Refaz tudo que o Avançado mostra depois de mexer em arquivos ou importar. */
export function useRecarregarAvancado(): () => Promise<void> {
  const cliente = useQueryClient();
  return async () => {
    await Promise.all([
      cliente.invalidateQueries({ queryKey: ['avancado'] }),
      cliente.invalidateQueries({ queryKey: ['agente'] }),
      cliente.invalidateQueries({ queryKey: ['arquivo'] }),
      cliente.invalidateQueries({ queryKey: ['skills'] }),
    ]);
  };
}

export const mensagemDeErro = (erro: unknown): string =>
  erro instanceof Error ? erro.message : String(erro);
