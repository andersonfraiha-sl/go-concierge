import { Box, Chip, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import type { Bloco } from '@/fluxo/modelo/tipos';
import { tokens } from '@/theme/tokens';

const SEM_FALA = 'Este passo não aparece para o morador.';

type Previa =
  | { tipo: 'silencioso' }
  | { tipo: 'bolha'; texto: string }
  | { tipo: 'chips'; itens: readonly string[] };

/** O que o bloco mostra ao morador: a fala literal (pergunta/texto) ou, para "Perguntar", os campos pedidos. */
function previaDoBloco(bloco: Bloco): Previa {
  switch (bloco.tipo) {
    case 'pedir_escolha':
    case 'reaproveitar':
      return bloco.pergunta ? { tipo: 'bolha', texto: bloco.pergunta } : { tipo: 'silencioso' };
    case 'confirmar':
    case 'montar_texto':
    case 'responder':
      return bloco.texto ? { tipo: 'bolha', texto: bloco.texto } : { tipo: 'silencioso' };
    case 'perguntar':
      return bloco.campos.length > 0
        ? { tipo: 'chips', itens: bloco.campos }
        : { tipo: 'silencioso' };
    case 'buscar_dados':
    case 'gravar_no_sistema':
    case 'formulario':
      return { tipo: 'silencioso' };
  }
}

export function ComoOMoradorVe({ bloco }: { bloco: Bloco }): ReactNode {
  const previa = previaDoBloco(bloco);

  if (previa.tipo === 'silencioso') {
    return (
      <Typography variant="body2" color="text.secondary">
        {SEM_FALA}
      </Typography>
    );
  }

  if (previa.tipo === 'chips') {
    return (
      <Box sx={{ display: 'flex', gap: 1, flexWrap: 'wrap' }}>
        {previa.itens.map((item) => (
          <Chip key={item} label={item} size="small" variant="outlined" />
        ))}
      </Box>
    );
  }

  return (
    <Box
      sx={{
        alignSelf: 'flex-start',
        maxWidth: '85%',
        bgcolor: tokens.color.background.paper,
        border: 1,
        borderColor: tokens.color.border.default,
        borderRadius: '12px 12px 12px 4px',
        px: 1.5,
        py: 1,
      }}
    >
      <Typography variant="body2">{previa.texto}</Typography>
    </Box>
  );
}
