import { Alert } from '@mui/material';
import type { ReactNode } from 'react';

const TEXTO =
  'O Copiloto mudou este campo enquanto você editava. Mantivemos o seu texto; ao salvar, ele substitui a mudança do Copiloto.';

type Props = { visivel: boolean; onFechar: () => void };

/** Edição pendente e proposta aplicada no mesmo campo: a pessoa precisa saber que escolheu o dela. */
export function AvisoDoCopiloto({ visivel, onFechar }: Props): ReactNode {
  if (!visivel) return null;
  return (
    <Alert severity="warning" onClose={onFechar}>
      {TEXTO}
    </Alert>
  );
}
