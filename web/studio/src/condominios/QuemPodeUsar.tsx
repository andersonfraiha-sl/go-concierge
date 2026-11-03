import { Box, ToggleButton, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import { rotuloDoPapel } from './listaDeSkills';

type Props = {
  papeis: readonly string[];
  escolhidos: readonly string[];
  travado: boolean;
  onMudar: (papeis: string[]) => void;
};

export function QuemPodeUsar({ papeis, escolhidos, travado, onMudar }: Props): ReactNode {
  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
      <Typography variant="subtitle2" component="h3">
        Quem pode usar
      </Typography>
      <Box sx={{ display: 'flex', gap: 1, flexWrap: 'wrap' }}>
        <ToggleButton
          value="todos"
          selected={escolhidos.length === 0}
          disabled={travado}
          sx={{ borderRadius: 9 }}
          onClick={() => onMudar([])}
        >
          Todos
        </ToggleButton>
        {papeis.map((papel) => {
          const marcado = escolhidos.includes(papel);
          const proximos = marcado ? escolhidos.filter((p) => p !== papel) : [...escolhidos, papel];
          return (
            <ToggleButton
              key={papel}
              value={papel}
              selected={marcado}
              disabled={travado}
              sx={{ borderRadius: 9 }}
              onClick={() => onMudar(proximos)}
            >
              {rotuloDoPapel(papel)}
            </ToggleButton>
          );
        })}
      </Box>
    </Box>
  );
}
