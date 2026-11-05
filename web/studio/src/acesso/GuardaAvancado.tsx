import { Box } from '@mui/material';
import type { ReactNode } from 'react';
import { AvancadoPage, CabecalhoDoAvancado } from '@/avancado/AvancadoPage';
import { ErroAcesso } from './ErroAcesso';
import { pode, useEu } from './eu';
import { SemAcesso } from './SemAcesso';

export function GuardaAvancado(): ReactNode {
  const { data: eu, isPending, isError, refetch } = useEu();
  if (isPending) return null;
  if (isError) return <ComCabecalho>{<ErroAcesso aoTentar={() => void refetch()} />}</ComCabecalho>;
  if (!pode(eu, 'studio.avancado')) return <ComCabecalho>{<SemAcesso />}</ComCabecalho>;
  return <AvancadoPage />;
}

/** Sem a página, o título continua: a pessoa sabe onde está. */
function ComCabecalho({ children }: { children: ReactNode }): ReactNode {
  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 3 }}>
      <CabecalhoDoAvancado />
      {children}
    </Box>
  );
}
