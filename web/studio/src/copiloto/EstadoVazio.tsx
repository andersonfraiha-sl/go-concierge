import { Box, Chip, Typography } from '@mui/material';
import { useEu } from '@/acesso/eu';
import { PESOS, RAIOS, TIPOS, tokens } from '@/theme/tokens';
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
          fontWeight: PESOS.negrito,
          fontSize: TIPOS.h5,
          color: 'secondary.main',
        }}
      >
        {TITULO}
      </Typography>
      <Typography variant="body2" color="text.secondary">
        {DESCRICAO}
      </Typography>
      {sugestoes.length > 0 && <ListaDeSugestoes sugestoes={sugestoes} onEscolher={enviar} />}
    </Box>
  );
}

type SugestoesProps = { sugestoes: string[]; onEscolher: (texto: string) => Promise<void> };

function ListaDeSugestoes({ sugestoes, onEscolher }: SugestoesProps) {
  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1, mt: 1 }}>
      <Typography
        component="h2"
        variant="overline"
        sx={{ color: 'text.secondary', fontWeight: PESOS.negrito }}
      >
        Sugestões
      </Typography>
      {sugestoes.map((texto) => (
        <Chip
          key={texto}
          label={texto}
          variant="outlined"
          onClick={() => onEscolher(texto)}
          sx={{
            justifyContent: 'flex-start',
            height: 'auto',
            borderRadius: `${RAIOS.menu}px`,
            borderColor: 'border.default',
            color: 'text.strong',
            bgcolor: 'background.paper',
            '& .MuiChip-label': { whiteSpace: 'normal', py: 1, px: 1.5 },
          }}
        />
      ))}
    </Box>
  );
}
