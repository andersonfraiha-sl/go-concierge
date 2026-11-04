import { Alert, Box, Button } from '@mui/material';
import type { ReactNode } from 'react';
import type { OrigemDoConflito } from './useEdicaoSobOServidor';

const QUEM_MUDOU: Record<OrigemDoConflito, string> = {
  copiloto: 'O Copiloto mudou este campo enquanto você editava.',
  outra: 'Este campo mudou em outro lugar enquanto você editava.',
};
const USAR_O_NOVO: Record<OrigemDoConflito, string> = {
  copiloto: 'Usar o do Copiloto',
  outra: 'Usar a versão nova',
};
const NADA_FOI_GRAVADO =
  'Nada foi salvo até você escolher. Se sair desta tela sem escolher, a sua edição se perde.';

type Props = {
  conflito: OrigemDoConflito | null;
  onManterOMeu: () => void;
  onUsarONovo: () => void;
};

/** Edição pendente e versão nova no servidor: a pessoa escolhe qual fica, nada some em silêncio. */
export function AvisoDeConflito({ conflito, onManterOMeu, onUsarONovo }: Props): ReactNode {
  if (conflito === null) return null;
  return (
    <Alert
      severity="warning"
      action={
        <EscolhasDoConflito
          conflito={conflito}
          onManterOMeu={onManterOMeu}
          onUsarONovo={onUsarONovo}
        />
      }
    >
      {`${QUEM_MUDOU[conflito]} ${NADA_FOI_GRAVADO}`}
    </Alert>
  );
}

function EscolhasDoConflito({
  conflito,
  onManterOMeu,
  onUsarONovo,
}: Omit<Props, 'conflito'> & { conflito: OrigemDoConflito }): ReactNode {
  return (
    <Box sx={{ display: 'flex', gap: 1, alignItems: 'center' }}>
      <Button color="inherit" size="small" onClick={onManterOMeu}>
        Manter o meu
      </Button>
      <Button color="inherit" size="small" onClick={onUsarONovo}>
        {USAR_O_NOVO[conflito]}
      </Button>
    </Box>
  );
}
