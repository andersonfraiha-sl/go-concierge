import AutoAwesomeOutlined from '@mui/icons-material/AutoAwesomeOutlined';
import { Box, Chip, Typography } from '@mui/material';
import { type ReactNode, useId } from 'react';
import { useEu } from '@/acesso/eu';
import { ICONES, PESOS, RAIOS, TIPOS } from '@/theme/tokens';
import { useCopiloto } from './CopilotoProvider';
import { sugestoesDoHubVazio } from './sugestoes';

const ROTULO = 'Ou peça ao Copiloto';

const ESTILO_DO_ROTULO = {
  fontSize: TIPOS.overline,
  fontWeight: PESOS.semi,
  color: 'text.muted',
  textTransform: 'uppercase',
  letterSpacing: '.06em',
} as const;

/** Atalhos do hub vazio: abrem o painel e já enviam o pedido, sem a pessoa digitar de novo. */
export function PecaAoCopiloto(): ReactNode {
  const { aberto, alternar, enviar } = useCopiloto();
  const { data: eu } = useEu();
  const idDoRotulo = useId();
  const sugestoes = eu ? sugestoesDoHubVazio(eu) : [];
  if (sugestoes.length === 0) return null;

  const pedir = (texto: string): void => {
    if (!aberto) alternar();
    void enviar(texto);
  };

  return (
    <Box
      role="group"
      aria-labelledby={idDoRotulo}
      sx={{ mt: 2.5, display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 1.25 }}
    >
      <Typography id={idDoRotulo} sx={ESTILO_DO_ROTULO}>
        {ROTULO}
      </Typography>
      <ChipsDePedido sugestoes={sugestoes} onPedir={pedir} />
    </Box>
  );
}

type ChipsProps = { sugestoes: readonly string[]; onPedir: (texto: string) => void };

function ChipsDePedido({ sugestoes, onPedir }: ChipsProps): ReactNode {
  return (
    <Box sx={{ display: 'flex', gap: 1, flexWrap: 'wrap', justifyContent: 'center' }}>
      {sugestoes.map((texto) => (
        <Chip
          key={texto}
          label={texto}
          variant="outlined"
          icon={<AutoAwesomeOutlined />}
          onClick={() => onPedir(texto)}
          sx={{
            height: 'auto',
            borderRadius: `${RAIOS.menu}px`,
            borderColor: 'border.default',
            bgcolor: 'background.paper',
            color: 'text.strong',
            fontSize: TIPOS.body2,
            '& .MuiChip-icon': { color: 'secondary.main', fontSize: ICONES.pequeno, ml: 1.5 },
            '& .MuiChip-label': { py: 1, pr: 1.5 },
          }}
        />
      ))}
    </Box>
  );
}
