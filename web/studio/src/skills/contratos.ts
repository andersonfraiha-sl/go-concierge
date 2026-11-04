import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { z } from 'zod';
import {
  type Fluxo,
  type FluxoGravado,
  FluxoGravadoSchema,
  type FluxoLido,
  FluxoLidoSchema,
} from '@/fluxo/modelo/tipos';
import { ApiError, apiEnviar, apiGet } from '@/shared/api/http';
import { listaDoGo } from '@/shared/api/listaDoGo';

export const SkillResumoSchema = z.object({
  nome: z.string(),
  id: z.string(),
  titulo: z.string(),
  descricao: z.string(),
  escrita: z.boolean(),
  tipo: z.string(),
  recursos: listaDoGo(z.string()),
  caminho: z.string(),
  resumo: z.string().default(''),
  icone: z.string().default(''),
});
export type SkillResumo = z.infer<typeof SkillResumoSchema>;

export const SkillsSchema = z.object({
  assistente: z.string(),
  skills: z.array(SkillResumoSchema),
});

export const TextosSchema = z.object({
  titulo: z.string(),
  quandoUsar: z.string(),
  frasesDeExemplo: listaDoGo(z.string()),
  palavrasChave: listaDoGo(z.string()),
  temFluxo: z.boolean(),
  resumo: z.string().default(''),
  visibilidade: z.string().default(''),
});
export type Textos = z.infer<typeof TextosSchema>;

export const TextosLidosSchema = z.object({
  caminho: z.string(),
  textos: TextosSchema,
  versao: z.string(),
});
export type TextosLidos = z.infer<typeof TextosLidosSchema>;

const VersaoSchema = z.object({ versao: z.string() });
const NomeSchema = z.object({ nome: z.string() });

export const LIMITE_DO_RESUMO = 90;

const caminhoDaSkill = (nome: string): string => `/api/skills/${encodeURIComponent(nome)}`;

export function useSkills() {
  return useQuery({
    queryKey: ['skills'],
    queryFn: () => apiGet('/api/skills', SkillsSchema),
  });
}

export const chaveDosTextos = (nome: string) => ['skill', nome, 'textos'] as const;

export function buscarTextos(nome: string): Promise<TextosLidos> {
  return apiGet(caminhoDaSkill(nome), TextosLidosSchema);
}

export function useTextos(nome: string) {
  return useQuery({
    queryKey: chaveDosTextos(nome),
    queryFn: () => buscarTextos(nome),
  });
}

export const chaveDoFluxo = (nome: string) => ['skill', nome, 'fluxo'] as const;

const TENTATIVAS_PADRAO = 3;

/** 404 é resposta (a skill não tem fluxo), não falha passageira: repetir só atrasa a tela. */
const repetirSalvo404 = (falhas: number, erro: Error): boolean =>
  !(erro instanceof ApiError && erro.status === 404) && falhas < TENTATIVAS_PADRAO;

export function buscarFluxo(nome: string): Promise<FluxoLido> {
  return apiGet(`${caminhoDaSkill(nome)}/fluxo`, FluxoLidoSchema);
}

export function useFluxo(nome: string) {
  return useQuery({
    queryKey: chaveDoFluxo(nome),
    queryFn: () => buscarFluxo(nome),
    retry: repetirSalvo404,
  });
}

export function salvarTextos(
  nome: string,
  corpo: { versao: string; textos: Textos },
): Promise<{ versao: string }> {
  return apiEnviar('PUT', `${caminhoDaSkill(nome)}/textos`, corpo, VersaoSchema);
}

export function salvarFluxo(
  nome: string,
  corpo: { versao: string; fluxo: Fluxo },
): Promise<FluxoGravado> {
  return apiEnviar('PUT', `${caminhoDaSkill(nome)}/fluxo`, corpo, FluxoGravadoSchema);
}

/** Grava resumo e critério de uma skill recém-criada; sem efeito se os dois vierem em branco. */
export async function preencherTextosNovos(
  nome: string,
  novos: { resumo: string; quandoUsar: string },
): Promise<void> {
  if (novos.resumo === '' && novos.quandoUsar === '') return;
  const lido = await buscarTextos(nome);
  await salvarTextos(nome, { versao: lido.versao, textos: { ...lido.textos, ...novos } });
}

/** Mutation que, ao dar certo, refaz a lista de skills. */
function useMutationDeSkills<V, R>(enviar: (variaveis: V) => Promise<R>) {
  const cliente = useQueryClient();
  return useMutation({
    mutationFn: enviar,
    onSuccess: () => cliente.invalidateQueries({ queryKey: ['skills'] }),
  });
}

export const criarSkill = (c: { nome: string; titulo: string }): Promise<{ nome: string }> =>
  apiEnviar('POST', '/api/skills', c, NomeSchema);

export function useCriarSkill() {
  return useMutationDeSkills(criarSkill);
}

export function useDuplicarSkill() {
  return useMutationDeSkills((c: { origem: string; nome: string; titulo: string }) =>
    apiEnviar(
      'POST',
      `${caminhoDaSkill(c.origem)}/duplicar`,
      { nome: c.nome, titulo: c.titulo },
      NomeSchema,
    ),
  );
}

export const excluirSkill = (nome: string): Promise<null> =>
  apiEnviar('DELETE', caminhoDaSkill(nome), undefined, z.null());

export function useExcluirSkill() {
  return useMutationDeSkills(excluirSkill);
}

/** Nome de pasta a partir do título: minúsculas, sem acento, hífen entre palavras. */
export function paraIdentificador(titulo: string): string {
  return titulo
    .normalize('NFD')
    .replace(/\p{M}/gu, '')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
}
