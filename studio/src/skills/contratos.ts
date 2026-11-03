import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { z } from 'zod';
import { apiEnviar, apiGet } from '@/shared/api/http';
import { listaDoGo } from '@/shared/api/listaDoGo';
import {
  FluxoGravadoSchema,
  FluxoLidoSchema,
  type Fluxo,
  type FluxoGravado,
} from '@/fluxo/modelo/tipos';

export const SkillResumoSchema = z.object({
  nome: z.string(),
  id: z.string(),
  titulo: z.string(),
  descricao: z.string(),
  escrita: z.boolean(),
  tipo: z.string(),
  recursos: listaDoGo(z.string()),
  caminho: z.string(),
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
});
export type Textos = z.infer<typeof TextosSchema>;

export const TextosLidosSchema = z.object({
  caminho: z.string(),
  textos: TextosSchema,
  versao: z.string(),
});

const VersaoSchema = z.object({ versao: z.string() });
const NomeSchema = z.object({ nome: z.string() });

const caminhoDaSkill = (nome: string): string => `/api/skills/${encodeURIComponent(nome)}`;

export function useSkills() {
  return useQuery({
    queryKey: ['skills'],
    queryFn: () => apiGet('/api/skills', SkillsSchema),
  });
}

export function useTextos(nome: string) {
  return useQuery({
    queryKey: ['skill', nome, 'textos'],
    queryFn: () => apiGet(caminhoDaSkill(nome), TextosLidosSchema),
  });
}

export function useFluxo(nome: string) {
  return useQuery({
    queryKey: ['skill', nome, 'fluxo'],
    queryFn: () => apiGet(`${caminhoDaSkill(nome)}/fluxo`, FluxoLidoSchema),
  });
}

export function salvarTextos(nome: string, corpo: { versao: string; textos: Textos }): Promise<{ versao: string }> {
  return apiEnviar('PUT', `${caminhoDaSkill(nome)}/textos`, corpo, VersaoSchema);
}

export function salvarFluxo(nome: string, corpo: { versao: string; fluxo: Fluxo }): Promise<FluxoGravado> {
  return apiEnviar('PUT', `${caminhoDaSkill(nome)}/fluxo`, corpo, FluxoGravadoSchema);
}

/** Mutation que, ao dar certo, refaz a lista de skills. */
function useMutationDeSkills<V, R>(enviar: (variaveis: V) => Promise<R>) {
  const cliente = useQueryClient();
  return useMutation({
    mutationFn: enviar,
    onSuccess: () => cliente.invalidateQueries({ queryKey: ['skills'] }),
  });
}

export function useCriarSkill() {
  return useMutationDeSkills((c: { nome: string; titulo: string }) =>
    apiEnviar('POST', '/api/skills', c, NomeSchema),
  );
}

export function useDuplicarSkill() {
  return useMutationDeSkills((c: { origem: string; nome: string; titulo: string }) =>
    apiEnviar('POST', `${caminhoDaSkill(c.origem)}/duplicar`, { nome: c.nome, titulo: c.titulo }, NomeSchema),
  );
}

export function useExcluirSkill() {
  return useMutationDeSkills((nome: string) =>
    apiEnviar('DELETE', caminhoDaSkill(nome), undefined, z.null()),
  );
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
