import { type Camada, TIPOS_DE_CAMADA } from './contratos';

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
