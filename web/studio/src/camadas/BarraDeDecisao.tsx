import { Box, Button, Typography } from '@mui/material';
import type { ReactNode } from 'react';

type Props = {
  /** Conflitos que ainda esperam escolha. */
  faltam: number;
  enviando: boolean;
  onAceitarTudo: () => void;
  onRecusarTudo: () => void;
  onSalvar: () => void;
};

const textoDoQueFalta = (faltam: number): string =>
  faltam === 1
    ? 'Falta decidir 1 trecho em conflito.'
    : `Faltam decidir ${faltam} trechos em conflito.`;

export function BarraDeDecisao({
  faltam,
  enviando,
  onAceitarTudo,
  onRecusarTudo,
  onSalvar,
}: Props): ReactNode {
  return (
    <Box
      role="toolbar"
      aria-label="Decidir a atualização"
      sx={{ display: 'flex', alignItems: 'center', gap: 1.5, flexWrap: 'wrap' }}
    >
      <Button variant="outlined" onClick={onAceitarTudo} disabled={enviando}>
        Aceitar tudo
      </Button>
      <Button variant="outlined" onClick={onRecusarTudo} disabled={enviando}>
        Recusar tudo
      </Button>
      <Box sx={{ flex: 1 }} />
      {faltam > 0 && (
        <Typography variant="body2" color="textSecondary" role="status">
          {textoDoQueFalta(faltam)}
        </Typography>
      )}
      <Button variant="contained" onClick={onSalvar} disabled={faltam > 0 || enviando}>
        Salvar decisão
      </Button>
    </Box>
  );
}
