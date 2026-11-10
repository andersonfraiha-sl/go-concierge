import type { Decisao, Diferenca } from './contratos';

export type Escolha = { escolha: 'aceitar' | 'manter' } | { escolha: 'misturar'; mistura: string };

/** Chave: o id do trecho. Trecho sem chave ainda não foi decidido. */
export type Escolhas = Readonly<Record<string, Escolha>>;

const ACEITAR: Escolha = { escolha: 'aceitar' };
const MANTER: Escolha = { escolha: 'manter' };

const ehConflito = (diferenca: Diferenca): boolean => diferenca.situacao === 'conflito';

function escolhasPara(
  diferencas: readonly Diferenca[],
  escolher: (diferenca: Diferenca) => Escolha,
): Escolhas {
  return Object.fromEntries(diferencas.map((d) => [d.trecho, escolher(d)]));
}

/** Padrão da tela: o que só a base mudou entra; conflito espera a pessoa. Obrigatório sempre entra. */
export function escolhasIniciais(diferencas: readonly Diferenca[]): Escolhas {
  return escolhasPara(
    diferencas.filter((d) => d.obrigatorio || !ehConflito(d)),
    () => ACEITAR,
  );
}

export function aceitarTudo(diferencas: readonly Diferenca[]): Escolhas {
  return escolhasPara(diferencas, () => ACEITAR);
}

/** O obrigatório não se recusa: a Superlógica já decidiu por todos. */
export function recusarTudo(diferencas: readonly Diferenca[]): Escolhas {
  return escolhasPara(diferencas, (d) => (d.obrigatorio ? ACEITAR : MANTER));
}

/** Conflitos que ainda esperam uma escolha. */
export function pendentes(diferencas: readonly Diferenca[], escolhas: Escolhas): string[] {
  return diferencas
    .filter((d) => ehConflito(d) && !d.obrigatorio && escolhas[d.trecho] === undefined)
    .map((d) => d.trecho);
}

export function paraEnvio(escolhas: Escolhas): Decisao[] {
  return Object.entries(escolhas).map(([trecho, escolha]) =>
    escolha.escolha === 'misturar'
      ? { trecho, escolha: 'misturar', mistura: escolha.mistura }
      : { trecho, escolha: escolha.escolha },
  );
}
