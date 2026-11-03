import { Box, Button, Tooltip, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import type { EstadoSalvamento } from './useAutosave';

type Props = { estado: EstadoSalvamento; onTentarDeNovo: () => void };

const formatarHora = (em: Date): string =>
  em.toLocaleTimeString('pt-BR', { hour: '2-digit', minute: '2-digit' });

export function SaveIndicator({ estado, onTentarDeNovo }: Props): ReactNode {
  switch (estado.tipo) {
    case 'em_dia':
      return null;
    case 'salvando':
      return <Typography variant="caption">Salvando…</Typography>;
    case 'salvo':
      return <Typography variant="caption">Salvo às {formatarHora(estado.em)}</Typography>;
    case 'bloqueado':
      return (
        <Typography variant="caption" color="text.secondary">
          Não salvo: {estado.motivo}
        </Typography>
      );
    case 'erro':
      return (
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
          <Tooltip title={estado.mensagem}>
            <Typography variant="caption" color="error">
              Falha ao salvar
            </Typography>
          </Tooltip>
          <Button size="small" color="error" onClick={onTentarDeNovo}>
            Tentar de novo
          </Button>
        </Box>
      );
    default:
      return assertNever(estado);
  }
}

function assertNever(valor: never): never {
  throw new Error(`estado de salvamento não tratado: ${JSON.stringify(valor)}`);
}
