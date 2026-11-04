import { Box, Typography } from '@mui/material';
import { useState } from 'react';
import { Outlet } from 'react-router';
import { naoReconhecido, useEu } from '@/acesso/eu';
import { UsuarioNaoReconhecido } from '@/acesso/UsuarioNaoReconhecido';
import { CopilotoProvider } from '@/copiloto/CopilotoProvider';
import { PainelDoCopiloto } from '@/copiloto/PainelDoCopiloto';
import { Sidebar } from './Sidebar';
import { Topbar } from './Topbar';

const CHAVE_MENU_RECOLHIDO = 'studio.menuRecolhido';

export function StudioLayout() {
  const [recolhido, setRecolhido] = useState(lerMenuRecolhido);
  const usuario = useEu();
  const recusado = naoReconhecido(usuario.error);

  const alternarMenu = () => {
    const proximo = !recolhido;
    setRecolhido(proximo);
    gravarMenuRecolhido(proximo);
  };

  if (recusado) return <UsuarioNaoReconhecido dev={recusado.dev} />;

  return (
    <CopilotoProvider>
      <Box sx={{ minHeight: '100vh', display: 'flex', flexDirection: 'column' }}>
        <Topbar />
        <Box sx={{ flex: 1, minHeight: 0, display: 'flex' }}>
          <Sidebar recolhido={recolhido} aoAlternar={alternarMenu} />
          <PainelDoCopiloto />
          <AreaPrincipal carregando={usuario.isPending} />
        </Box>
      </Box>
    </CopilotoProvider>
  );
}

/** Sem o Eu, nenhuma tela: na troca de usuário a anterior some no mesmo clique. */
function AreaPrincipal({ carregando }: { carregando: boolean }) {
  return (
    <Box component="main" sx={{ flex: 1, minWidth: 0, p: 3, bgcolor: 'background.default' }}>
      {carregando ? <Typography color="text.secondary">Carregando…</Typography> : <Outlet />}
    </Box>
  );
}

function lerMenuRecolhido(): boolean {
  try {
    return localStorage.getItem(CHAVE_MENU_RECOLHIDO) === 'true';
  } catch (cause) {
    console.warn('ler preferência do menu:', cause);
    return false;
  }
}

function gravarMenuRecolhido(recolhido: boolean): void {
  try {
    localStorage.setItem(CHAVE_MENU_RECOLHIDO, String(recolhido));
  } catch (cause) {
    console.warn('gravar preferência do menu:', cause);
  }
}
