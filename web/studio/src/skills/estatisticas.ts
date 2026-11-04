import type { NoDaArvore } from '@/avancado/contratos';
import { contemBusca } from '@/shared/busca';
import type { SkillResumo } from './contratos';

export type Estatistica = { rotulo: string; valor: number; destaque?: boolean };

export function problemasPorCaminho(
  nos: readonly NoDaArvore[],
): Map<string, { erros: number; avisos: number }> {
  return new Map(nos.map((no) => [no.caminho, { erros: no.erros, avisos: no.avisos }]));
}

/** 4º cartão: "Com problema" para quem edita fluxo; para o resto, "Ferramentas" (recursos distintos). */
export function quartaEstatistica(
  skills: readonly SkillResumo[],
  problemas: ReadonlyMap<string, { erros: number; avisos: number }>,
  podeVerProblemas: boolean,
): Estatistica {
  if (!podeVerProblemas) {
    const totalDeRecursos = new Set(skills.flatMap((skill) => skill.recursos)).size;
    return { rotulo: 'Ferramentas', valor: totalDeRecursos };
  }
  const comProblema = skills.filter((skill) => {
    const problema = problemas.get(skill.caminho);
    return problema !== undefined && (problema.erros > 0 || problema.avisos > 0);
  });
  return { rotulo: 'Com problema', valor: comProblema.length, destaque: true };
}

/** Busca por título ou resumo; nunca pela descrição técnica. */
export function filtrarPorBusca(skills: readonly SkillResumo[], busca: string): SkillResumo[] {
  return skills.filter((skill) => contemBusca([skill.titulo, skill.resumo], busca));
}
