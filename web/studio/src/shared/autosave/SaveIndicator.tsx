import { Box, Button, Tooltip, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import type { EstadoSalvamento } from './useAutosave';

type Props = { estado: EstadoSalvamento; onTentarDeNovo: () => void };

export const formatarHora = (em: Date): string =>
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
      return <FalhaAoSalvar mensagem={estado.mensagem} onTentarDeNovo={onTentarDeNovo} />;
    default:
      return assertNever(estado);
  }
}

type PropsDaFalha = { mensagem: string; onTentarDeNovo: () => void };

function FalhaAoSalvar({ mensagem, onTentarDeNovo }: PropsDaFalha): ReactNode {
  return (
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
      <Tooltip title={mensagem}>
        <Typography variant="caption" color="error">
          Falha ao salvar
        </Typography>
      </Tooltip>
      <Button size="small" color="error" onClick={onTentarDeNovo}>
        Tentar de novo
      </Button>
    </Box>
  );
}

function assertNever(valor: never): never {
  throw new Error(`estado de salvamento não tratado: ${JSON.stringify(valor)}`);
}
