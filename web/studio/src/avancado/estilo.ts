import { FONTE_MONO, TIPOS } from '@/theme/tokens';

export const ESTILO_DE_CODIGO = {
  fontFamily: FONTE_MONO,
  fontSize: TIPOS.body2,
  whiteSpace: 'pre-wrap',
  overflowWrap: 'anywhere',
  m: 0,
} as const;

/**
 * Estilo do cartão do editor de arquivo: `pre` (sem quebra) em vez de
 * `pre-wrap`, para que uma linha lógica seja sempre uma linha visual e a
 * coluna de números não desalinhe; a rolagem horizontal fica para o
 * contêiner (`overflowX: 'auto'`).
 */
export const ESTILO_DO_EDITOR = {
  fontFamily: FONTE_MONO,
  fontSize: TIPOS.body2,
  whiteSpace: 'pre',
  lineHeight: 1.5,
  m: 0,
} as const;
