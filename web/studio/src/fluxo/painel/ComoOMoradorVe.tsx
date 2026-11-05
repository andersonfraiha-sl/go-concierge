import { Box, Chip, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import type { Bloco } from '@/fluxo/modelo/tipos';
import { RAIOS } from '@/theme/tokens';

const SEM_FALA = 'Este passo não aparece para o morador.';

type Previa =
  | { tipo: 'silencioso' }
  | { tipo: 'bolha'; texto: string }
  | { tipo: 'chips'; itens: readonly string[] };

const SILENCIOSO: Previa = { tipo: 'silencioso' };
const bolha = (texto: string): Previa => (texto ? { tipo: 'bolha', texto } : SILENCIOSO);
const chips = (itens: readonly string[]): Previa =>
  itens.length > 0 ? { tipo: 'chips', itens } : SILENCIOSO;

/** O que o bloco mostra ao morador: a fala literal (pergunta/texto) ou, para "Perguntar", os campos pedidos. */
function previaDoBloco(bloco: Bloco): Previa {
  switch (bloco.tipo) {
    case 'pedir_escolha':
    case 'reaproveitar':
      return bolha(bloco.pergunta);
    case 'confirmar':
    case 'montar_texto':
    case 'responder':
      return bolha(bloco.texto);
    case 'perguntar':
      return chips(bloco.campos);
    case 'buscar_dados':
    case 'gravar_no_sistema':
    case 'formulario':
      return SILENCIOSO;
  }
}

function BolhaDaFala({ texto }: { texto: string }): ReactNode {
  return (
    <Box
      sx={{
        alignSelf: 'flex-start',
        maxWidth: '85%',
        bgcolor: 'background.paper',
        border: 1,
        borderColor: 'border.default',
        borderRadius: `${RAIOS.painel}px ${RAIOS.painel}px ${RAIOS.painel}px ${RAIOS.canto}px`,
        px: 1.5,
        py: 1,
      }}
    >
      <Typography variant="body2">“{texto}”</Typography>
    </Box>
  );
}

function CamposPedidos({ itens }: { itens: readonly string[] }): ReactNode {
  return (
    <Box sx={{ display: 'flex', gap: 1, flexWrap: 'wrap' }}>
      {itens.map((item) => (
        <Chip key={item} label={item} size="small" variant="outlined" />
      ))}
    </Box>
  );
}

export function ComoOMoradorVe({ bloco }: { bloco: Bloco }): ReactNode {
  const previa = previaDoBloco(bloco);
  if (previa.tipo === 'chips') return <CamposPedidos itens={previa.itens} />;
  if (previa.tipo === 'bolha') return <BolhaDaFala texto={previa.texto} />;
  return (
    <Typography variant="body2" color="textSecondary">
      {SEM_FALA}
    </Typography>
  );
}
