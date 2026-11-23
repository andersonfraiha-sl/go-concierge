import { Box, Typography } from '@mui/material';
import type { ReactNode } from 'react';

type Props = { frase: string; acao?: ReactNode };

// por que: só JSX declarativo, sem lógica; quebrar espalharia o markup sem ganho de leitura.
export function EmptyState({ frase, acao }: Props): ReactNode {
  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 2, py: 6 }}>
      <Typography color="textSecondary">{frase}</Typography>
      {acao}
    </Box>
  );
}
