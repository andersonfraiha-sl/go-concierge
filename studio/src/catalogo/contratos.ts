import { useQuery } from '@tanstack/react-query';
import { z } from 'zod';
import { apiEnviar, apiGet } from '@/shared/api/http';

/** O Go serializa slice nil como null: a borda entrega sempre lista. */
export const listaDoGo = <S extends z.ZodType>(item: S) =>
  z
    .array(item)
    .nullish()
    .transform((lista) => lista ?? []);

const mapaDoGo = <S extends z.ZodType>(item: S) =>
  z
    .record(z.string(), listaDoGo(item))
    .nullish()
    .transform((mapa) => mapa ?? {});

export const CAMINHO_PERSONA = 'persona.md';
export const caminhoDeRegra = (arquivo: string): string => `rules/${arquivo}`;
export const caminhoDeDocumento = (arquivo: string): string => `knowledge/${arquivo}`;

const ArquivoSchema = z.object({
  caminho: z.string(),
  bytes: z.number(),
  resumo: z.string().optional(),
  herdado_de: z.string().optional(),
});
export type Arquivo = z.infer<typeof ArquivoSchema>;

const ProblemaSchema = z.object({
  nivel: z.string(),
  onde: z.string(),
  texto: z.string(),
  solucao: z.string().optional(),
});
export type Problema = z.infer<typeof ProblemaSchema>;

const ParametroSchema = z.object({
  name: z.string(),
  type: z.string().optional(),
  required: z.boolean().optional(),
  description: z.string().optional(),
});

const CampoDeSaidaSchema = z.object({
  name: z.string(),
  path: z.string().optional(),
  type: z.string().optional(),
  description: z.string().optional(),
});

const DefinicaoSchema = z.object({
  name: z.string(),
  label: z.string(),
  type: z.string().optional(),
  options: listaDoGo(z.string()),
  default: z.json().optional(),
  description: z.string().optional(),
});

const ConexaoSchema = z.object({
  metodo: z.string().optional(),
  url: z.string().optional(),
  headers: z.record(z.string(), z.string()).optional(),
  corpo: z.string().optional(),
});

export const FerramentaSchema = z.object({
  name: z.string(),
  titulo: z.string().optional(),
  kind: z.string(),
  visibility: z.string(),
  write: z.boolean(),
  description: z.string(),
  params: listaDoGo(ParametroSchema),
  output: z.string().optional(),
  conexao: ConexaoSchema.optional(),
  saida: listaDoGo(CampoDeSaidaSchema),
  amostra: z.json().optional(),
  definicoes: listaDoGo(DefinicaoSchema),
  origem: z.string().optional(),
  schema: z.json().optional(),
});
export type Ferramenta = z.infer<typeof FerramentaSchema>;

export const FerramentasSchema = z.object({
  account: z.string(),
  origem: z.string(),
  conferido: z.boolean(),
  recursos: listaDoGo(FerramentaSchema),
});

const RecursoDeclaradoSchema = z.object({
  nome: z.string(),
  origem: z.string(),
  ok: z.boolean(),
  problema: z.string().optional(),
  recurso: FerramentaSchema.optional(),
  conferido: z.boolean(),
});
export type RecursoDeclarado = z.infer<typeof RecursoDeclaradoSchema>;

export const AgenteSchema = z.object({
  agente: z.object({
    caminho: z.string(),
    nome: z.string(),
    descricao: z.string(),
    persona: z.string(),
    arquivos: mapaDoGo(ArquivoSchema),
    herdados: mapaDoGo(ArquivoSchema),
    problemas: listaDoGo(ProblemaSchema),
  }),
  recursos: listaDoGo(RecursoDeclaradoSchema),
});

const ConteudoSchema = z.object({ caminho: z.string(), conteudo: z.string() });

const consulta = (parametros: Record<string, string>): string => new URLSearchParams(parametros).toString();

export function useAgente(caminho: string) {
  return useQuery({
    queryKey: ['agente', caminho],
    queryFn: () => apiGet(`/api/agente?${consulta({ agente: caminho })}`, AgenteSchema),
  });
}

export function useArquivo(agente: string, caminho: string) {
  return useQuery({
    queryKey: ['arquivo', agente, caminho],
    queryFn: () => apiGet(`/api/arquivo?${consulta({ agente, caminho })}`, ConteudoSchema),
  });
}

export function useFerramentas() {
  return useQuery({
    queryKey: ['ferramentas'],
    queryFn: () => apiGet('/api/nexus/recursos', FerramentasSchema),
  });
}

export async function salvarArquivo(c: { agente: string; caminho: string; conteudo: string }): Promise<void> {
  await apiEnviar('PUT', '/api/arquivo', c, z.object({ salvo: z.string() }));
}

export async function excluirArquivo(agente: string, caminho: string): Promise<void> {
  await apiEnviar('DELETE', `/api/arquivo?${consulta({ agente, caminho })}`, undefined, z.object({ excluido: z.string() }));
}

export async function lerModeloDeArquivo(caminho: string): Promise<string> {
  const modelo = await apiGet(`/api/arquivo/modelo?${consulta({ caminho })}`, ConteudoSchema);
  return modelo.conteudo;
}

export async function vincularFerramenta(c: { agente: string; nome: string; ligar: boolean }): Promise<void> {
  await apiEnviar('POST', '/api/agente/recursos', c, z.object({ ligado: z.boolean() }));
}
