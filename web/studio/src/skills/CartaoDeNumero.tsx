import { Box, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import { RAIOS, tokens } from '@/theme/tokens';
import type { Estatistica } from './estatisticas';

/** Cartão de número do topo da tela de skills: rótulo + valor, com tom de aviso quando `destaque`. */
export function CartaoDeNumero({ rotulo, valor, destaque }: Estatistica): ReactNode {
  return (
    <Box
      role="group"
      aria-label={rotulo}
      sx={{
        bgcolor: destaque ? tokens.color.warning.background : 'background.paper',
        border: '1px solid',
        borderColor: 'divider',
        borderRadius: `${RAIOS.cartao}px`,
        p: 3,
      }}
    >
      <Typography
        variant="caption"
        sx={{ color: destaque ? tokens.color.warning.text : tokens.color.text.tertiary }}
      >
        {rotulo}
      </Typography>
      <Typography
        variant="h4"
        sx={{ color: destaque ? tokens.color.warning.text : tokens.color.navy, mt: 1 }}
      >
        {valor}
      </Typography>
    </Box>
  );
}
