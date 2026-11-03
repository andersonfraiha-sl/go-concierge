import { Box } from '@mui/material';
import { useState } from 'react';
import { Outlet } from 'react-router';
import { Sidebar } from './Sidebar';
import { Topbar } from './Topbar';

const CHAVE_MENU_RECOLHIDO = 'studio.menuRecolhido';

export function StudioLayout() {
  const [recolhido, setRecolhido] = useState(lerMenuRecolhido);

  const alternarMenu = () => {
    const proximo = !recolhido;
    setRecolhido(proximo);
    gravarMenuRecolhido(proximo);
  };

  return (
    <Box sx={{ minHeight: '100vh', display: 'flex', flexDirection: 'column' }}>
      <Topbar />
      <Box sx={{ flex: 1, minHeight: 0, display: 'flex' }}>
        <Sidebar recolhido={recolhido} aoAlternar={alternarMenu} />
        {/* Lugar do painel do Copiloto, montado pela Task 25. */}
        <Box component="main" sx={{ flex: 1, minWidth: 0, p: 3, bgcolor: 'background.default' }}>
          <Outlet />
        </Box>
      </Box>
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
