import { type UseQueryResult, useQuery } from '@tanstack/react-query';
import { z } from 'zod';
import { FluxoSchema } from '@/fluxo/modelo/tipos';
import { type ApiError, apiEnviar, apiGet } from '@/shared/api/http';
import { listaDoGo } from '@/shared/api/listaDoGo';

// Espelho de internal/copiloto (Go). Os limites batem com a validação de POST /api/copiloto.
export const LIMITE_DA_MENSAGEM = 4000;
export const TAMANHO_DO_HISTORICO = 20;
// Cinco segundos acima dos 90 s do modelo no servidor: o 504 dele chega antes do nosso prazo.
export const PRAZO_DO_COPILOTO_MS = 95_000;

export const TELAS = [
  'skills',
  'skill',
  'fluxo',
  'condominios',
  'condominio',
  'concierge',
  'ferramentas',
  'avancado',
] as const;
export type Tela = (typeof TELAS)[number];

export type ContextoDoCopiloto = {
  tela: Tela;
  skill?: string;
  condominio?: string;
  agente?: string;
  caminho?: string;
};

export const TipoDeOperacaoSchema = z.enum([
  'preencher_textos',
  'trocar_fluxo',
  'criar_skill',
  'excluir_skill',
  'habilitar_skill',
  'orientar',
  'ajustar',
  'editar_persona',
  'editar_arquivo',
]);
export type TipoDeOperacao = z.infer<typeof TipoDeOperacaoSchema>;

const TextosParciaisSchema = z.object({
  titulo: z.string().optional(),
  resumo: z.string().optional(),
  quandoUsar: z.string().optional(),
  palavrasChave: z.array(z.string()).optional(),
  frasesDeExemplo: z.array(z.string()).optional(),
});

// O Go manda `omitempty`: skill vazia (orientação geral) e agente padrão chegam ausentes.
const opcional = z.string().optional();

export const OperacaoSchema = z.discriminatedUnion('tipo', [
  z.object({
    tipo: z.literal('preencher_textos'),
    skill: z.string(),
    textos: TextosParciaisSchema,
  }),
  z.object({ tipo: z.literal('trocar_fluxo'), skill: z.string(), fluxo: FluxoSchema }),
  z.object({
    tipo: z.literal('criar_skill'),
    titulo: z.string(),
    textos: TextosParciaisSchema.optional(),
  }),
  z.object({ tipo: z.literal('excluir_skill'), skill: z.string() }),
  z.object({
    tipo: z.literal('habilitar_skill'),
    condominio: z.string(),
    skill: z.string(),
    habilitado: z.boolean(),
    papeis: z.array(z.string()).optional(),
  }),
  z.object({
    tipo: z.literal('orientar'),
    condominio: z.string(),
    skill: opcional,
    texto: z.string(),
  }),
  z.object({
    tipo: z.literal('ajustar'),
    condominio: z.string(),
    skill: opcional,
    recurso: z.string(),
    campos: z.record(z.string(), z.json()),
  }),
  z.object({ tipo: z.literal('editar_persona'), agente: opcional, texto: z.string() }),
  z.object({
    tipo: z.literal('editar_arquivo'),
    agente: opcional,
    caminho: z.string(),
    conteudo: z.string(),
  }),
]);
export type Operacao = z.infer<typeof OperacaoSchema>;

export const PropostaSchema = z.object({
  id: z.string(),
  resumo: z.string(),
  operacao: OperacaoSchema,
});
export type Proposta = z.infer<typeof PropostaSchema>;

export const RespostaDoCopilotoSchema = z.object({
  mensagem: z.string(),
  propostas: listaDoGo(PropostaSchema),
  recusas: listaDoGo(z.object({ indice: z.number().int(), motivo: z.string() })),
});
export type RespostaDoCopiloto = z.infer<typeof RespostaDoCopilotoSchema>;

export type MensagemDoHistorico = { papel: 'pessoa' | 'copiloto'; texto: string };

export type PedidoAoCopiloto = {
  mensagem: string;
  historico: MensagemDoHistorico[];
  contexto: ContextoDoCopiloto;
};

export function historicoDe(mensagens: readonly MensagemDoHistorico[]): MensagemDoHistorico[] {
  return mensagens.slice(-TAMANHO_DO_HISTORICO).map(({ papel, texto }) => ({ papel, texto }));
}

export function conversar(
  corpo: PedidoAoCopiloto,
  sinal: AbortSignal,
): Promise<RespostaDoCopiloto> {
  const comPrazo = AbortSignal.any([sinal, AbortSignal.timeout(PRAZO_DO_COPILOTO_MS)]);
  return apiEnviar('POST', '/api/copiloto', corpo, RespostaDoCopilotoSchema, comPrazo);
}

const EstadoDoCopilotoSchema = z.object({ disponivel: z.boolean() });
export const CHAVE_DO_ESTADO = ['copiloto', 'estado'] as const;

export function useEstadoDoCopiloto(
  ativo: boolean,
): UseQueryResult<z.infer<typeof EstadoDoCopilotoSchema>, ApiError> {
  return useQuery({
    queryKey: CHAVE_DO_ESTADO,
    queryFn: () => apiGet('/api/copiloto/estado', EstadoDoCopilotoSchema),
    enabled: ativo,
  });
}
