import type { SxProps, Theme } from '@mui/material';
import { type ReactNode, useState } from 'react';
import { MolduraDoStudio } from './MolduraDoStudio';
import { Sidebar } from './Sidebar';
import { Topbar } from './Topbar';

const CHAVE_MENU_RECOLHIDO = 'studio.menuRecolhido';

// Telas de lista: 34 px em cima, 55 nas laterais e embaixo, na largura toda da janela.
const ESTILO_DO_CONTEUDO: SxProps<Theme> = { pt: 4, px: 5, pb: 5 };

/** Layout das telas de lista: barra global no topo e menu à esquerda. */
export function StudioLayout(): ReactNode {
  return (
    <MolduraDoStudio
      topo={<Topbar />}
      lateral={<MenuLateral />}
      estiloDoConteudo={ESTILO_DO_CONTEUDO}
    />
  );
}

/** Menu à esquerda com a preferência de recolhido lembrada entre telas e sessões. */
export function MenuLateral(): ReactNode {
  const [recolhido, setRecolhido] = useState(lerMenuRecolhido);

  const alternarMenu = () => {
    const proximo = !recolhido;
    setRecolhido(proximo);
    gravarMenuRecolhido(proximo);
  };

  return <Sidebar recolhido={recolhido} aoAlternar={alternarMenu} />;
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
