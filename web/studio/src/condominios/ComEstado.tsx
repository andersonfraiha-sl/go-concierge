import { Button, Typography } from '@mui/material';
import type { UseQueryResult } from '@tanstack/react-query';
import type { ReactNode } from 'react';
import { EmptyState } from '@/shared/ui/EmptyState';
import type { Estado } from './contratos';

type Props = { consulta: UseQueryResult<Estado>; children: (estado: Estado) => ReactNode };

export function ComEstado({ consulta, children }: Props): ReactNode {
  if (consulta.isError) {
    return (
      <EmptyState
        frase="Não foi possível carregar os condomínios."
        acao={<Button onClick={() => void consulta.refetch()}>Tentar de novo</Button>}
      />
    );
  }
  if (consulta.data === undefined)
    return <Typography color="text.secondary">Carregando…</Typography>;
  return children(consulta.data);
}
