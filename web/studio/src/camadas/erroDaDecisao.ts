import type { ProblemaFluxo } from '@/fluxo/modelo/tipos';
import { ApiError } from '@/shared/api/http';
import { mensagemDe } from '@/shared/api/mensagemDe';
import { problemasDaRecusa } from '@/skills/abas/fluxoDaSkill';
import type { Diferenca } from './contratos';

export type ErroDaDecisao =
  | { tipo: 'desatualizada' }
  | { tipo: 'substituida' }
  | { tipo: 'problemas'; problemas: ProblemaFluxo[] }
  | { tipo: 'outro'; mensagem: string };

export function classificarErroDaDecisao(erro: unknown): ErroDaDecisao {
  if (erro instanceof ApiError && erro.status === 409) {
    if (erro.codigo === 'VERSAO_DESATUALIZADA') return { tipo: 'desatualizada' };
    if (erro.codigo === 'ATUALIZACAO_SUBSTITUIDA') return { tipo: 'substituida' };
  }
  const problemas = problemasDaRecusa(erro);
  if (problemas !== null) return { tipo: 'problemas', problemas };
  return { tipo: 'outro', mensagem: mensagemDe(erro) };
}

export type ProblemasDistribuidos = {
  porTrecho: ReadonlyMap<string, ProblemaFluxo[]>;
  /** Os que não apontam para um trecho desta atualização (ex.: o fluxo todo). */
  gerais: ProblemaFluxo[];
};

/** Um problema de etapa aparece no trecho `etapa:<id>` dela; o resto fica na tela. */
export function distribuirProblemas(
  problemas: readonly ProblemaFluxo[],
  diferencas: readonly Diferenca[],
): ProblemasDistribuidos {
  const trechos = new Set(diferencas.map((d) => d.trecho));
  const porTrecho = new Map<string, ProblemaFluxo[]>();
  const gerais: ProblemaFluxo[] = [];
  for (const problema of problemas) {
    const trecho = problema.blocoId === undefined ? undefined : `etapa:${problema.blocoId}`;
    if (trecho === undefined || !trechos.has(trecho)) {
      gerais.push(problema);
      continue;
    }
    porTrecho.set(trecho, [...(porTrecho.get(trecho) ?? []), problema]);
  }
  return { porTrecho, gerais };
}
