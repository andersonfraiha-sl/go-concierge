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
export const MENSAGEM_DO_RESUMO = `O resumo tem uma linha só, até ${LIMITE_DO_RESUMO} caracteres.`;

/** Regra do resumo, a mesma no diálogo, na aba e nas propostas do Copiloto. */
export const ResumoSchema = z
  .string()
  .max(LIMITE_DO_RESUMO, MENSAGEM_DO_RESUMO)
  .regex(/^[^\r\n]*$/, MENSAGEM_DO_RESUMO);

/** Título aparado que precisa virar um nome de pasta não vazio. */
export const TituloDaSkillSchema = z
  .string()
  .trim()
  .refine((titulo) => paraIdentificador(titulo) !== '', 'Use letras ou números no nome.');

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

export function useTextos(nome: string, opcoes: { enabled?: boolean } = {}) {
  return useQuery({
    queryKey: chaveDosTextos(nome),
    queryFn: () => buscarTextos(nome),
    enabled: opcoes.enabled ?? true,
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

type CampoDeTexto = 'titulo' | 'resumo' | 'quandoUsar' | 'palavrasChave' | 'frasesDeExemplo';
/** Campo ausente (ou undefined) mantém o que está gravado. */
export type TextosNovos = { [C in CampoDeTexto]?: Textos[C] | undefined };

/** Grava textos sobre a versão atual da skill; sem efeito se todos vierem ausentes ou em branco. */
export async function preencherTextosNovos(nome: string, novos: TextosNovos): Promise<void> {
  const vazio = Object.values(novos).every((valor) => valor === undefined || valor === '');
  if (vazio) return;
  const lido = await buscarTextos(nome);
  await salvarTextos(nome, { versao: lido.versao, textos: mesclarTextos(lido.textos, novos) });
}

const mesclarTextos = (atuais: Textos, novos: TextosNovos): Textos => ({
  ...atuais,
  titulo: novos.titulo ?? atuais.titulo,
  resumo: novos.resumo ?? atuais.resumo,
  quandoUsar: novos.quandoUsar ?? atuais.quandoUsar,
  palavrasChave: novos.palavrasChave ?? atuais.palavrasChave,
  frasesDeExemplo: novos.frasesDeExemplo ?? atuais.frasesDeExemplo,
});

/** Mutation que, ao dar certo, refaz a lista de skills. */
function useMutationDeSkills<V, R>(enviar: (variaveis: V) => Promise<R>) {
  const cliente = useQueryClient();
  return useMutation({
    mutationFn: enviar,
    onSuccess: () => cliente.invalidateQueries({ queryKey: ['skills'] }),
  });
}

export const criarSkill = (nova: { nome: string; titulo: string }): Promise<{ nome: string }> =>
  apiEnviar('POST', '/api/skills', nova, NomeSchema);

export function useCriarSkill() {
  return useMutationDeSkills(criarSkill);
}

type PedidoDeCopia = { origem: string; nome: string; titulo: string };

function duplicarSkill({ origem, nome, titulo }: PedidoDeCopia): Promise<{ nome: string }> {
  return apiEnviar('POST', `${caminhoDaSkill(origem)}/duplicar`, { nome, titulo }, NomeSchema);
}

export function useDuplicarSkill() {
  return useMutationDeSkills(duplicarSkill);
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
