import { Alert, Button } from '@mui/material';
import type { ReactNode } from 'react';

type Props = { mensagem: string; onTentarDeNovo: () => void };

export function ErroComRetry({ mensagem, onTentarDeNovo }: Props): ReactNode {
  return (
    <Alert
      severity="error"
      action={
        <Button color="inherit" onClick={onTentarDeNovo}>
          Tentar de novo
        </Button>
      }
    >
      {mensagem}
    </Alert>
  );
}
