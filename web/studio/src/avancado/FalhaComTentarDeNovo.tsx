import { Box, Button, Typography } from '@mui/material';
import type { ReactNode } from 'react';

type Props = { mensagem: string; onTentarDeNovo: () => void };

export function FalhaComTentarDeNovo({ mensagem, onTentarDeNovo }: Props): ReactNode {
  return (
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
      <Typography role="alert">{mensagem}</Typography>
      <Button size="small" onClick={onTentarDeNovo}>
        Tentar de novo
      </Button>
    </Box>
  );
}
