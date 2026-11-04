import { Button, Typography } from '@mui/material';
import type { UseQueryResult } from '@tanstack/react-query';
import type { ReactNode } from 'react';
import { EmptyState } from '@/shared/ui/EmptyState';
import type { Estado } from './contratos';

type Props = { consulta: UseQueryResult<Estado>; children: (estado: Estado) => ReactNode };

export function ComEstado({ consulta, children }: Props): ReactNode {
  if (consulta.isError) return <ErroAoCarregar aoTentar={() => void consulta.refetch()} />;
  if (consulta.data === undefined)
    return <Typography color="text.secondary">Carregando…</Typography>;
  return children(consulta.data);
}

function ErroAoCarregar({ aoTentar }: { aoTentar: () => void }): ReactNode {
  return (
    <EmptyState
      frase="Não foi possível carregar os condomínios."
      acao={<Button onClick={aoTentar}>Tentar de novo</Button>}
    />
  );
}
