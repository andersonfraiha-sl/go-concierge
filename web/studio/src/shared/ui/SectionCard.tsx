import { Box, Paper, Typography } from '@mui/material';
import type { ReactNode } from 'react';

type Props = { titulo: string; ajuda: string; acao?: ReactNode; children: ReactNode };

// por que: só JSX declarativo, sem lógica; quebrar espalharia o markup sem ganho de leitura.
export function SectionCard({ titulo, ajuda, acao, children }: Props): ReactNode {
  return (
    <Paper variant="outlined" sx={{ p: 2, display: 'flex', flexDirection: 'column', gap: 1.5 }}>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
        <Box sx={{ flex: 1 }}>
          <Typography variant="subtitle2" component="h2">
            {titulo}
          </Typography>
          <Typography variant="caption" color="textSecondary">
            {ajuda}
          </Typography>
        </Box>
        {acao}
      </Box>
      {children}
    </Paper>
  );
}
