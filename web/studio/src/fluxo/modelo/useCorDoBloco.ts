import { useTheme } from '@mui/material';
import { BLOCOS } from './catalogo';
import type { TipoBloco } from './tipos';

/** Cor da categoria do bloco, resolvida no tema. */
export function useCorDoBloco(tipo: TipoBloco): string {
  const { palette } = useTheme();
  return palette.studio.category[BLOCOS[tipo].categoria];
}
