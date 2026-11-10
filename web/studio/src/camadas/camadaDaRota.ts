import { type Camada, TIPOS_DE_CAMADA, type TipoCamada } from './contratos';

type ParametrosDaRota = {
  nome?: string | undefined;
  tipo?: string | undefined;
  id?: string | undefined;
};

/** Lê a camada dos parâmetros da URL; tipo desconhecido ou parte faltando não é camada. */
export function camadaDaRota({ nome, tipo, id }: ParametrosDaRota): Camada | null {
  if (!nome || !id) return null;
  const tipoConhecido = TIPOS_DE_CAMADA.find((conhecido) => conhecido === tipo);
  return tipoConhecido ? { skill: nome, tipo: tipoConhecido, id } : null;
}

const SEGMENTO_DA_ROTA = { administradora: 'administradoras', condominio: 'condominios' } as const;

/** Segmento `:tipo` da URL (plural) para o tipo da camada; segmento desconhecido não é camada. */
export function tipoDoSegmento(segmento: string | undefined): TipoCamada | undefined {
  return TIPOS_DE_CAMADA.find((tipo) => SEGMENTO_DA_ROTA[tipo] === segmento);
}

export function rotaDaCamada({ skill, tipo, id }: Camada): string {
  return `/skills/${encodeURIComponent(skill)}/${SEGMENTO_DA_ROTA[tipo]}/${encodeURIComponent(id)}`;
}
