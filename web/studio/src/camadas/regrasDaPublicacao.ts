import type { z } from 'zod';
import type { ProblemaFluxo } from '@/fluxo/modelo/tipos';
import { ApiError } from '@/shared/api/http';
import { mensagemDe } from '@/shared/api/mensagemDe';
import { problemasDaRecusa } from '@/skills/abas/fluxoDaSkill';
import { CamadasQuebramSchema } from './contratos';

export type CamadaQuebrada = z.infer<typeof CamadasQuebramSchema>['camadas'][number];

export type ErroDaPublicacao =
  | { tipo: 'quebram'; camadas: CamadaQuebrada[] }
  | { tipo: 'problemas'; problemas: ProblemaFluxo[] }
  | { tipo: 'outro'; mensagem: string };

export function classificarErroDaPublicacao(erro: unknown): ErroDaPublicacao {
  if (erro instanceof ApiError && erro.status === 409 && erro.codigo === 'CAMADAS_QUEBRAM') {
    const lida = CamadasQuebramSchema.safeParse(erro.corpo);
    if (lida.success) return { tipo: 'quebram', camadas: lida.data.camadas };
  }
  const problemas = problemasDaRecusa(erro);
  // Um 422 sem problemas (MOTIVO_OBRIGATORIO) vale pela mensagem do servidor.
  if (problemas !== null && problemas.length > 0) return { tipo: 'problemas', problemas };
  return { tipo: 'outro', mensagem: mensagemDe(erro) };
}

/** Trecho obrigatório e encerramento mexem em camadas que não pediram: a Superlógica explica o porquê. */
export function exigeMotivo(obrigatorios: readonly string[], encerraEm: string): boolean {
  return obrigatorios.length > 0 || encerraEm !== '';
}
