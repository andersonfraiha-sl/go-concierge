import { Box, Chip, Typography } from '@mui/material';
import { useEu } from '@/acesso/eu';
import { RAIOS, tokens } from '@/theme/tokens';
import { useCopiloto } from './CopilotoProvider';
import { sugestoesPara } from './sugestoes';

const TITULO = 'Como posso ajudar?';
const DESCRICAO = 'Peça uma mudança, uma explicação ou uma skill nova. Nada muda sem você aplicar.';

export function EstadoVazio() {
  const { contexto, enviar } = useCopiloto();
  const { data: eu } = useEu();
  const sugestoes = eu ? sugestoesPara(contexto, eu) : [];

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
      <Typography
        sx={{
          fontFamily: tokens.typography.h5.fontFamily,
          fontWeight: 700,
          fontSize: 21,
          color: tokens.color.navy,
        }}
      >
        {TITULO}
      </Typography>
      <Typography variant="body2" color="text.secondary">
        {DESCRICAO}
      </Typography>
      {sugestoes.length > 0 && (
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1, mt: 1 }}>
          {sugestoes.map((texto) => (
            <Chip
              key={texto}
              label={texto}
              variant="outlined"
              onClick={() => enviar(texto)}
              sx={{
                justifyContent: 'flex-start',
                height: 'auto',
                borderRadius: `${RAIOS.menu}px`,
                borderColor: tokens.color.border.default,
                color: tokens.color.text.strong,
                bgcolor: tokens.color.background.paper,
                '& .MuiChip-label': { whiteSpace: 'normal', py: 1, px: 1.5 },
              }}
            />
          ))}
        </Box>
      )}
    </Box>
  );
}
