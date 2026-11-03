import { Box, Button } from '@mui/material';
import { useId } from 'react';
import { mensagemDe } from '@/shared/api/mensagemDe';
import { useSnackbar } from '@/shared/snackbar/SnackbarContext';
import { RAIOS, tokens } from '@/theme/tokens';
import { useCopiloto } from './CopilotoProvider';
import type { Proposta } from './contratos';

type Props = { proposta: Proposta };

const linha = `1px solid ${tokens.color.border.subtle}`;

export function CartaoDeProposta({ proposta }: Props) {
  const { aplicar, descartar } = useCopiloto();
  const { mostrar } = useSnackbar();
  const idDoResumo = useId();

  const aoAplicar = () => {
    aplicar(proposta).catch((erro: unknown) => mostrar(mensagemDe(erro), 'error'));
  };

  return (
    <Box
      role="group"
      aria-labelledby={idDoResumo}
      sx={{
        alignSelf: 'stretch',
        bgcolor: 'background.paper',
        border: 1,
        borderColor: tokens.color.border.strong,
        borderRadius: `${RAIOS.painel}px`,
        overflow: 'hidden',
      }}
    >
      <Box
        id={idDoResumo}
        sx={{
          px: 2,
          py: 1.25,
          bgcolor: tokens.color.background.subtle,
          borderBottom: linha,
          fontSize: 13,
          fontWeight: 600,
          color: tokens.color.text.strong,
        }}
      >
        {proposta.resumo}
      </Box>
      <Box sx={{ px: 2, py: 1.25, display: 'flex', gap: 1 }}>
        <Button variant="contained" size="small" fullWidth onClick={aoAplicar}>
          Aplicar
        </Button>
        <Button
          variant="outlined"
          color="inherit"
          size="small"
          fullWidth
          onClick={() => descartar(proposta.id)}
          sx={{ color: 'text.secondary', borderColor: tokens.color.border.default }}
        >
          Descartar
        </Button>
      </Box>
    </Box>
  );
}
