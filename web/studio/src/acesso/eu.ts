import { useMediaQuery, useTheme } from '@mui/material';
import { type UseQueryResult, useQuery } from '@tanstack/react-query';
import { z } from 'zod';
import { type ApiError, apiGet } from '@/shared/api/http';
import { useUsuarioDev } from './usuarioDev';

export const PERMISSOES = [
  'skills.ler',
  'skills.editar_textos',
  'skills.editar_fluxo',
  'skills.criar',
  'skills.excluir',
  'ferramentas.editar',
  'condominios.habilitar',
  'condominios.editar',
  'administradoras.editar',
  'studio.avancado',
] as const;

export type Permissao = (typeof PERMISSOES)[number];

export const EuSchema = z.object({
  id: z.string(),
  nome: z.string(),
  papel: z.string(),
  permissoes: z.array(z.string()),
  administradoraId: z.string().nullable(),
  dev: z.boolean(),
});

export type Eu = z.infer<typeof EuSchema>;

const STATUS_NAO_AUTENTICADO = 401;
const CorpoNaoAutenticado = z.object({ dev: z.boolean().catch(false) });

/** O 401 de /api/eu não traz o Eu; o modo dev vem no próprio corpo do erro. */
export function naoReconhecido(erro: ApiError | null): { dev: boolean } | null {
  if (erro?.status !== STATUS_NAO_AUTENTICADO) return null;
  const corpo = CorpoNaoAutenticado.safeParse(erro.corpo);
  return { dev: corpo.success && corpo.data.dev };
}

export function useEu(): UseQueryResult<Eu, ApiError> {
  const usuarioDev = useUsuarioDev();
  return useQuery({ queryKey: ['eu', usuarioDev], queryFn: () => apiGet('/api/eu', EuSchema) });
}

export function pode(eu: Eu, permissao: Permissao): boolean {
  return eu.permissoes.includes(permissao);
}

export function podeVerCondominios(eu: Eu): boolean {
  return pode(eu, 'condominios.habilitar') || pode(eu, 'condominios.editar');
}

/** O catálogo é global: quem pertence a uma administradora não o edita. */
export function podeNoCatalogo(eu: Eu, permissao: Permissao): boolean {
  return pode(eu, permissao) && eu.administradoraId === null;
}

// Somente leitura abaixo do breakpoint `md`, para qualquer papel.
function useTelaEstreita(): boolean {
  const { breakpoints } = useTheme();
  return useMediaQuery(breakpoints.down('md'), { noSsr: true });
}

export function usePodeEditar(permissao: Permissao): boolean {
  const { data: eu } = useEu();
  const telaEstreita = useTelaEstreita();
  return eu !== undefined && !telaEstreita && podeNoCatalogo(eu, permissao);
}

export function usePodeEditarCondominio(permissao: Permissao): boolean {
  const { data: eu } = useEu();
  const telaEstreita = useTelaEstreita();
  return eu !== undefined && !telaEstreita && pode(eu, permissao);
}
