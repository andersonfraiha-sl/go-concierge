import { z } from 'zod';

export const TIPOS_BLOCO = [
  'buscar_dados',
  'gravar_no_sistema',
  'pedir_escolha',
  'perguntar',
  'formulario',
  'confirmar',
  'montar_texto',
  'reaproveitar',
  'responder',
] as const;
export type TipoBloco = (typeof TIPOS_BLOCO)[number];

// O Go manda `omitempty`: texto e lista ausentes viram '' e [] aqui, e voltam completos no PUT.
const texto = z.string().default('');
const lista = z.array(z.string()).default([]);

const base = {
  id: z.string(),
  extra: z.record(z.string(), z.json()).optional(),
};

const CampoValorSchema = z.object({ campo: z.string(), valor: z.json() });

const recurso = {
  recurso: texto,
  com: z.array(CampoValorSchema).default([]),
  guardarEm: texto,
};

export const BlocoSchema = z.discriminatedUnion('tipo', [
  z.object({ ...base, tipo: z.literal('buscar_dados'), ...recurso }),
  z.object({ ...base, tipo: z.literal('gravar_no_sistema'), ...recurso }),
  z.object({
    ...base,
    tipo: z.literal('pedir_escolha'),
    param: texto,
    titulo: texto,
    de: texto,
    rotulo: texto,
    valor: texto,
    pergunta: texto,
    seVazio: texto,
    reaproveitar: texto,
  }),
  z.object({ ...base, tipo: z.literal('perguntar'), campos: lista }),
  z.object({ ...base, tipo: z.literal('formulario'), de: texto, guardarEm: texto }),
  z.object({ ...base, tipo: z.literal('confirmar'), texto }),
  z.object({ ...base, tipo: z.literal('montar_texto'), texto }),
  z.object({
    ...base,
    tipo: z.literal('reaproveitar'),
    campos: lista,
    confirmacao: texto,
    de: texto,
    rotulo: texto,
    valor: texto,
    pergunta: texto,
  }),
  z.object({ ...base, tipo: z.literal('responder'), texto }),
]);
export type Bloco = z.infer<typeof BlocoSchema>;

export const InformacaoSchema = z.object({
  nome: z.string(),
  titulo: texto,
  pergunta: texto,
  tipo: texto,
  opcoes: z.array(z.object({ valor: z.string(), rotulo: z.string() })).default([]),
  opcoesSimples: z.boolean().default(false),
  extra: z.record(z.string(), z.json()).optional(),
});
export type Informacao = z.infer<typeof InformacaoSchema>;

export const FluxoSchema = z.object({
  blocos: z.array(BlocoSchema),
  informacoes: z.array(InformacaoSchema),
});
export type Fluxo = z.infer<typeof FluxoSchema>;

export const ProblemaFluxoSchema = z.object({
  blocoId: z.string().optional(),
  codigo: z.string(),
  nivel: z.enum(['erro', 'aviso']),
  mensagem: z.string(),
});
export type ProblemaFluxo = z.infer<typeof ProblemaFluxoSchema>;

export const FluxoLidoSchema = z.object({
  fluxo: FluxoSchema,
  versao: z.string(),
  problemas: z.array(ProblemaFluxoSchema),
});
export type FluxoLido = z.infer<typeof FluxoLidoSchema>;

export const FluxoGravadoSchema = z.object({
  versao: z.string(),
  problemas: z.array(ProblemaFluxoSchema),
});
export type FluxoGravado = z.infer<typeof FluxoGravadoSchema>;
