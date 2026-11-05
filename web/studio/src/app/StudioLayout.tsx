import type { SxProps, Theme } from '@mui/material';
import { type ReactNode, useState } from 'react';
import { SHELL } from '@/theme/tokens';
import { MolduraDoStudio } from './MolduraDoStudio';
import { Sidebar } from './Sidebar';
import { Topbar } from './Topbar';

const CHAVE_MENU_RECOLHIDO = 'studio.menuRecolhido';

// Telas de lista: 34 px em cima, 55 nas laterais e embaixo, até 1204 px de largura.
const ESTILO_DO_CONTEUDO: SxProps<Theme> = {
  pt: 4,
  px: 5,
  pb: 5,
  maxWidth: SHELL.larguraMaximaDaLista,
};

/** Layout das telas de lista: barra global no topo e menu à esquerda. */
export function StudioLayout(): ReactNode {
  const [recolhido, setRecolhido] = useState(lerMenuRecolhido);

  const alternarMenu = () => {
    const proximo = !recolhido;
    setRecolhido(proximo);
    gravarMenuRecolhido(proximo);
  };

  return (
    <MolduraDoStudio
      topo={<Topbar />}
      lateral={<Sidebar recolhido={recolhido} aoAlternar={alternarMenu} />}
      estiloDoConteudo={ESTILO_DO_CONTEUDO}
    />
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
