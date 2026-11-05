import { Box, Paper, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import { CONCIERGE, RAIO } from '@/theme/tokens';

type Props = { titulo: string; ajuda: string; acao?: ReactNode; children: ReactNode };

/**
 * Cartão do desenho aprovado da página Concierge: borda e raio mais largos que o `SectionCard`
 * padrão do Studio, então vive aqui em vez de alterar o componente compartilhado.
 */
export function ConciergeCard({ titulo, ajuda, acao, children }: Props): ReactNode {
  return (
    <Paper
      variant="outlined"
      sx={{
        p: 3,
        display: 'flex',
        flexDirection: 'column',
        gap: 2,
        borderColor: 'border.default',
        borderRadius: RAIO.cartao,
        fontSize: CONCIERGE.fonteDoCartao,
      }}
    >
      <Box sx={{ display: 'flex', alignItems: 'flex-start', gap: 2 }}>
        <Box sx={{ flex: 1, minWidth: 0 }}>
          <Typography variant="subtitle1" component="h2">
            {titulo}
          </Typography>
          <Typography variant="body2" color="text.secondary">
            {ajuda}
          </Typography>
        </Box>
        {acao && <Box sx={{ flexShrink: 0, whiteSpace: 'nowrap' }}>{acao}</Box>}
      </Box>
      {children}
    </Paper>
  );
}
