import { Box } from '@mui/material';
import type { ReactNode } from 'react';
import { ESTILO_DO_CONTEUDO_DE_DETALHE } from '@/app/detalhe/LayoutDeDetalhe';
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

/** Sem a página, o cabeçalho continua: é ele que leva de volta. */
function ComCabecalho({ children }: { children: ReactNode }): ReactNode {
  return (
    <>
      <CabecalhoDoAvancado />
      <Box sx={ESTILO_DO_CONTEUDO_DE_DETALHE}>{children}</Box>
    </>
  );
}
