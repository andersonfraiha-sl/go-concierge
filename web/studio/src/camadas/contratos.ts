import { useQuery } from '@tanstack/react-query';
import { z } from 'zod';
import { FluxoSchema, ProblemaFluxoSchema } from '@/fluxo/modelo/tipos';
import { apiEnviar, apiGet } from '@/shared/api/http';
import { listaDoGo } from '@/shared/api/listaDoGo';
import { TextosSchema } from '@/skills/contratos';

export const TIPOS_DE_CAMADA = ['administradora', 'condominio'] as const;
export type TipoCamada = (typeof TIPOS_DE_CAMADA)[number];

const problemas = listaDoGo(ProblemaFluxoSchema);
const dataHora = z.string();

export const DocumentoSchema = z.object({
  textos: TextosSchema,
  fluxo: FluxoSchema,
  ferramentas: listaDoGo(z.string()),
  orientacao: z.string().default(''),
});
export type Documento = z.infer<typeof DocumentoSchema>;

export const CamadaSchema = z.object({
  skill: z.string(),
  tipo: z.enum(TIPOS_DE_CAMADA),
  id: z.string(),
});
export type Camada = z.infer<typeof CamadaSchema>;

export const CamadaLidaSchema = z.object({
  camada: CamadaSchema,
  documento: DocumentoSchema,
  versao: z.string(),
  temRascunho: z.boolean(),
  publicada: z.object({ numero: z.number(), autor: z.string(), criadoEm: dataHora }).nullable(),
  ferramentasDaBase: listaDoGo(z.string()),
  problemas,
});
export type CamadaLida = z.infer<typeof CamadaLidaSchema>;

export const RascunhoSalvoSchema = z.object({ versao: z.string(), problemas });
export type RascunhoSalvo = z.infer<typeof RascunhoSalvoSchema>;

/** Resposta de publicar a camada e de voltar a uma versão: o número da versão gerada. */
export const PublicadaSchema = z.object({ numero: z.number() });

export const ORIGENS_DE_VERSAO = [
  'importacao',
  'inicial',
  'publicacao',
  'decisao',
  'obrigatorio',
  'prazo',
  'volta',
] as const;
export type OrigemDeVersao = (typeof ORIGENS_DE_VERSAO)[number];

// Contagens de trechos: o contrato do plano só dá os nomes, os números são decisão deste lado.
const DecisaoRegistradaSchema = z.object({
  para: z.number(),
  aceitos: z.number(),
  mantidos: z.number(),
  misturados: z.number(),
});

const VersaoDoHistoricoSchema = z.object({
  numero: z.number(),
  origem: z.enum(ORIGENS_DE_VERSAO),
  autor: z.string(),
  resumo: z.string().default(''),
  motivo: z.string().default(''),
  criadoEm: dataHora,
  decisao: DecisaoRegistradaSchema.nullable(),
});
export type VersaoDoHistorico = z.infer<typeof VersaoDoHistoricoSchema>;

export const HistoricoSchema = z.object({ versoes: listaDoGo(VersaoDoHistoricoSchema) });

export const TIPOS_DE_TRECHO = ['texto', 'etapa', 'informacao', 'ordem', 'orientacao'] as const;
export const SITUACOES_DE_TRECHO = ['so_base', 'conflito'] as const;

export const DiferencaSchema = z.object({
  trecho: z.string(),
  tipo: z.enum(TIPOS_DE_TRECHO),
  rotulo: z.string(),
  situacao: z.enum(SITUACOES_DE_TRECHO),
  obrigatorio: z.boolean(),
  antes: z.string().nullable(),
  depois: z.string().nullable(),
  minha: z.string().nullable(),
});
export type Diferenca = z.infer<typeof DiferencaSchema>;

export const PreviaDaBaseSchema = z.object({
  numero: z.number().nullable(),
  diferencas: listaDoGo(DiferencaSchema),
});

/** `camadas` é quantas camadas receberam a atualização. */
export const PublicacaoBaseSchema = z.object({ numero: z.number(), camadas: z.number() });

export const CamadasQuebramSchema = z.object({
  camadas: listaDoGo(z.object({ tipo: z.enum(TIPOS_DE_CAMADA), id: z.string(), problemas })),
});

export const ESTADOS_DE_ATUALIZACAO = [
  'pendente',
  'aceita',
  'recusada',
  'parcial',
  'substituida',
] as const;
export type EstadoDeAtualizacao = (typeof ESTADOS_DE_ATUALIZACAO)[number];

const ParaSchema = z.object({ numero: z.number(), resumo: z.string().default('') });

const ResumoDeAtualizacaoSchema = z.object({
  id: z.string(),
  camada: CamadaSchema,
  estado: z.enum(ESTADOS_DE_ATUALIZACAO),
  para: ParaSchema,
  prazoEm: dataHora.nullable(),
  trechos: z.number(),
  conflitos: z.number(),
});

const AvisoSchema = z.object({
  id: z.string(),
  camada: CamadaSchema,
  motivo: z.string(),
  trechos: listaDoGo(z.string()),
  encerraEm: z.string().nullable(),
  criadoEm: dataHora,
});

export const AtualizacoesSchema = z.object({
  atualizacoes: listaDoGo(ResumoDeAtualizacaoSchema),
  avisos: listaDoGo(AvisoSchema),
});
export type Atualizacoes = z.infer<typeof AtualizacoesSchema>;

export const AtualizacaoSchema = z.object({
  id: z.string(),
  camada: CamadaSchema,
  estado: z.enum(ESTADOS_DE_ATUALIZACAO),
  cabeca: z.number(),
  para: ParaSchema,
  prazoEm: dataHora.nullable(),
  diferencas: listaDoGo(DiferencaSchema),
});
export type Atualizacao = z.infer<typeof AtualizacaoSchema>;

export const DecididaSchema = z.object({
  numero: z.number(),
  estado: z.enum(ESTADOS_DE_ATUALIZACAO),
});

export type Decisao =
  | { trecho: string; escolha: 'aceitar' | 'manter' }
  | { trecho: string; escolha: 'misturar'; mistura: string };

export type PublicacaoDaBase = {
  resumo: string;
  obrigatorios: string[];
  motivo: string;
  encerraEm: string | null;
};

export function caminhoDaCamada(skill: string, tipo: TipoCamada, id: string): string {
  const colecao = tipo === 'administradora' ? 'administradoras' : 'condominios';
  return `/api/skills/${encodeURIComponent(skill)}/${colecao}/${encodeURIComponent(id)}`;
}

const caminhoDe = (c: Camada) => caminhoDaCamada(c.skill, c.tipo, c.id);
const caminhoDaBase = (skill: string) => `/api/skills/${encodeURIComponent(skill)}/publicar`;
const caminhoDaAtualizacao = (id: string) => `/api/atualizacoes/${encodeURIComponent(id)}`;

export const chaveDaCamada = (c: Camada) => ['camada', c.skill, c.tipo, c.id] as const;
export const chaveDoHistorico = (c: Camada) => ['camada-historico', c.skill, c.tipo, c.id] as const;
export const CHAVE_DAS_ATUALIZACOES = ['atualizacoes'] as const;
export const chaveDaAtualizacao = (id: string) => ['atualizacao', id] as const;
export const chaveDaPreviaDaBase = (skill: string) => ['previa-base', skill] as const;

export function useCamada(skill: string, tipo: TipoCamada, id: string) {
  const camada: Camada = { skill, tipo, id };
  return useQuery({
    queryKey: chaveDaCamada(camada),
    queryFn: () => apiGet(caminhoDe(camada), CamadaLidaSchema),
  });
}

export function salvarRascunho(
  c: Camada,
  corpo: { versao: string; documento: Documento },
): Promise<RascunhoSalvo> {
  return apiEnviar('PUT', `${caminhoDe(c)}/rascunho`, corpo, RascunhoSalvoSchema);
}

export function publicarCamada(c: Camada, corpo: { resumo: string }) {
  return apiEnviar('POST', `${caminhoDe(c)}/publicar`, corpo, PublicadaSchema);
}

export function useHistorico(c: Camada) {
  return useQuery({
    queryKey: chaveDoHistorico(c),
    queryFn: () => apiGet(`${caminhoDe(c)}/historico`, HistoricoSchema),
  });
}

export function voltarVersao(c: Camada, numero: number) {
  return apiEnviar('POST', `${caminhoDe(c)}/voltar`, { numero }, PublicadaSchema);
}

export function usePreviaDaBase(skill: string) {
  return useQuery({
    queryKey: chaveDaPreviaDaBase(skill),
    queryFn: () => apiGet(caminhoDaBase(skill), PreviaDaBaseSchema),
  });
}

export function publicarBase(skill: string, corpo: PublicacaoDaBase) {
  return apiEnviar('POST', caminhoDaBase(skill), corpo, PublicacaoBaseSchema);
}

export function useAtualizacoes() {
  return useQuery({
    queryKey: CHAVE_DAS_ATUALIZACOES,
    queryFn: () => apiGet('/api/atualizacoes', AtualizacoesSchema),
  });
}

export function useAtualizacao(id: string) {
  return useQuery({
    queryKey: chaveDaAtualizacao(id),
    queryFn: () => apiGet(caminhoDaAtualizacao(id), AtualizacaoSchema),
  });
}

export function decidir(id: string, corpo: { cabeca: number; decisoes: Decisao[] }) {
  return apiEnviar('POST', `${caminhoDaAtualizacao(id)}/decisao`, corpo, DecididaSchema);
}

export function marcarCiente(id: string) {
  return apiEnviar('POST', `/api/avisos/${encodeURIComponent(id)}/ciente`, undefined, z.null());
}
