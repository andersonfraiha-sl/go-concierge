import type { ReactNode } from 'react';
import { useLocation } from 'react-router';

/** Mostra a rota atual para o teste conferir para onde a tela navegou. */
export function LocalizacaoAtual(): ReactNode {
  return <output data-testid="localizacao">{useLocation().pathname}</output>;
}
