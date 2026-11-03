import { FONTE_MONO } from '@/theme/tokens';

export const ESTILO_DE_CODIGO = {
  fontFamily: FONTE_MONO,
  fontSize: 13,
  whiteSpace: 'pre-wrap',
  overflowWrap: 'anywhere',
  m: 0,
} as const;

/** Alinhamento vertical da primeira linha, compartilhado pelo textarea e pela coluna de números. */
export const PADDING_TOPO_DO_EDITOR = '16.5px';

/**
 * Estilo do cartão do editor de arquivo: `pre` (sem quebra) em vez de
 * `pre-wrap`, para que uma linha lógica seja sempre uma linha visual e a
 * coluna de números não desalinhe; a rolagem horizontal fica para o
 * contêiner (`overflowX: 'auto'`).
 */
export const ESTILO_DO_EDITOR = {
  fontFamily: FONTE_MONO,
  fontSize: 13,
  whiteSpace: 'pre',
  m: 0,
} as const;
