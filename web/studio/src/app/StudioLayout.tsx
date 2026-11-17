import type { SxProps, Theme } from '@mui/material';
import { type ReactNode, useState } from 'react';
import { MolduraDoStudio } from './MolduraDoStudio';
import { Sidebar } from './Sidebar';
import { Topbar } from './Topbar';

const CHAVE_MENU_RECOLHIDO = 'studio.menuRecolhido';
const CHAVE_SEM_CONDOMINIOS = 'studio.semCondominios';

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

/** Menu à esquerda com as preferências lembradas entre telas e sessões. */
export function MenuLateral(): ReactNode {
  const recolhido = usePreferencia(CHAVE_MENU_RECOLHIDO);
  const semCondominios = usePreferencia(CHAVE_SEM_CONDOMINIOS);
  return (
    <Sidebar
      recolhido={recolhido.ligada}
      aoAlternar={recolhido.alternar}
      semCondominios={semCondominios.ligada}
      aoAlternarCondominios={semCondominios.alternar}
    />
  );
}

function usePreferencia(chave: string): { ligada: boolean; alternar: () => void } {
  const [ligada, setLigada] = useState(() => lerPreferencia(chave));
  const alternar = () => {
    const proxima = !ligada;
    setLigada(proxima);
    gravarPreferencia(chave, proxima);
  };
  return { ligada, alternar };
}

function lerPreferencia(chave: string): boolean {
  try {
    return localStorage.getItem(chave) === 'true';
  } catch (cause) {
    console.warn('ler preferência do menu:', cause);
    return false;
  }
}

function gravarPreferencia(chave: string, ligada: boolean): void {
  try {
    localStorage.setItem(chave, String(ligada));
  } catch (cause) {
    console.warn('gravar preferência do menu:', cause);
  }
}
