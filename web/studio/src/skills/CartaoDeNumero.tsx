import { Box, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import { RAIOS, TOKENS_DA_SKILL } from '@/theme/tokens';
import type { Estatistica } from './estatisticas';

/** Cartão de número do topo da tela de skills: rótulo + valor, com tom de aviso quando `destaque`. */
// por que: só JSX declarativo, sem lógica; quebrar espalharia o markup sem ganho de leitura.
export function CartaoDeNumero({
  rotulo,
  valor,
  destaque,
}: Omit<Estatistica, 'valor'> & { valor: ReactNode }): ReactNode {
  return (
    <Box
      role="group"
      aria-label={rotulo}
      sx={{
        bgcolor: destaque ? 'warning.soft' : 'background.paper',
        border: 1,
        borderColor: destaque ? TOKENS_DA_SKILL.bordaDeAviso : 'divider',
        borderRadius: `${RAIOS.cartao}px`,
        p: 3,
      }}
    >
      <Typography variant="caption" sx={{ color: destaque ? 'warning.onSoft' : 'text.tertiary' }}>
        {rotulo}
      </Typography>
      <Typography
        variant="h4"
        sx={{ color: destaque ? 'warning.onSoft' : 'secondary.main', mt: 1 }}
      >
        {valor}
      </Typography>
    </Box>
  );
}
