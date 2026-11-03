import AddIcon from '@mui/icons-material/Add';
import CloseIcon from '@mui/icons-material/Close';
import { Box, Button, IconButton, Typography } from '@mui/material';
import type { ReactNode } from 'react';

type Props<T> = {
  titulo: string;
  adicionar: string;
  linhas: readonly T[];
  desabilitado: boolean;
  criarLinha: () => T;
  onMudar: (linhas: T[]) => void;
  renderLinha: (linha: T, mudar: (parcial: Partial<T>) => void) => ReactNode;
};

export function ListaEditavel<T>({
  titulo,
  adicionar,
  linhas,
  desabilitado,
  criarLinha,
  onMudar,
  renderLinha,
}: Props<T>): ReactNode {
  const mudarLinha = (indice: number, parcial: Partial<T>) =>
    onMudar(linhas.map((linha, i) => (i === indice ? { ...linha, ...parcial } : linha)));
  const tirarLinha = (indice: number) => onMudar(linhas.filter((_, i) => i !== indice));

  return (
    <Box component="section" aria-label={titulo} sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
      <Typography variant="subtitle2" component="h3">
        {titulo}
      </Typography>
      {linhas.map((linha, indice) => (
        // Sem id estável nas linhas: a posição é a identidade enquanto a lista só cresce pelo fim.
        <Box key={indice} sx={{ display: 'flex', gap: 1, alignItems: 'center' }}>
          <Box sx={{ flex: 1, display: 'flex', gap: 1, flexWrap: 'wrap' }}>
            {renderLinha(linha, (parcial) => mudarLinha(indice, parcial))}
          </Box>
          <IconButton
            size="small"
            aria-label={`Tirar ${titulo.toLowerCase()} ${indice + 1}`}
            disabled={desabilitado}
            onClick={() => tirarLinha(indice)}
          >
            <CloseIcon fontSize="small" />
          </IconButton>
        </Box>
      ))}
      <Box>
        <Button size="small" startIcon={<AddIcon />} disabled={desabilitado} onClick={() => onMudar([...linhas, criarLinha()])}>
          {adicionar}
        </Button>
      </Box>
    </Box>
  );
}
