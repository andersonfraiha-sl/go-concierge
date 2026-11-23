import { Box, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import { SaveIndicator } from '@/shared/autosave/SaveIndicator';
import type { EstadoSalvamento } from '@/shared/autosave/useAutosave';

type Props = {
  titulo: string;
  salvamento: EstadoSalvamento;
  tentarDeNovo: () => Promise<void>;
  children: ReactNode;
};

/** Cabeçalho de seção do drawer com o indicador de autosave ao lado do título. */
export function SecaoComSalvamento({
  titulo,
  salvamento,
  tentarDeNovo,
  children,
}: Props): ReactNode {
  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
        <Typography variant="subtitle2" component="h3" sx={{ flex: 1 }}>
          {titulo}
        </Typography>
        <SaveIndicator estado={salvamento} onTentarDeNovo={() => void tentarDeNovo()} />
      </Box>
      {children}
    </Box>
  );
}
