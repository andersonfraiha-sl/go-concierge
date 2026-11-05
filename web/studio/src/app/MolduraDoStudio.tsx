import { Box, type SxProps, type Theme, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import { Outlet } from 'react-router';
import { naoReconhecido, useEu } from '@/acesso/eu';
import { UsuarioNaoReconhecido } from '@/acesso/UsuarioNaoReconhecido';
import { PainelDoCopiloto } from '@/copiloto/PainelDoCopiloto';

type Props = {
  /** Faixa do topo: barra global ou cabeçalho de detalhe. */
  topo: ReactNode;
  /** Menu à esquerda do Copiloto; a tela de detalhe não tem. */
  lateral?: ReactNode;
  estiloDoConteudo?: SxProps<Theme>;
};

// A tela ocupa a janela e só o conteúdo rola: topo e Copiloto ficam sempre à vista.
const ESTILO_DA_TELA: SxProps<Theme> = {
  height: '100vh',
  display: 'flex',
  flexDirection: 'column',
  overflow: 'hidden',
  bgcolor: 'background.default',
};

const ESTILO_DO_MIOLO: SxProps<Theme> = { flex: 1, minHeight: 0, display: 'flex' };

/** Moldura comum: topo, menu opcional, Copiloto e a área da rota; 401 vira a tela própria. */
export function MolduraDoStudio({ topo, lateral, estiloDoConteudo }: Props): ReactNode {
  const usuario = useEu();
  const recusado = naoReconhecido(usuario.error);
  if (recusado) return <UsuarioNaoReconhecido dev={recusado.dev} />;

  return (
    <Box sx={ESTILO_DA_TELA}>
      {topo}
      <Box sx={ESTILO_DO_MIOLO}>
        {lateral}
        <PainelDoCopiloto />
        <Box component="main" sx={{ flex: 1, minWidth: 0, overflowY: 'auto' }}>
          <Box sx={estiloDoConteudo}>
            {/* Sem o Eu, nenhuma tela: na troca de usuário a anterior some no mesmo clique. */}
            {usuario.isPending ? (
              <Typography color="textSecondary">Carregando…</Typography>
            ) : (
              <Outlet />
            )}
          </Box>
        </Box>
      </Box>
    </Box>
  );
}
