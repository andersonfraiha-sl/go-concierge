import { Box } from '@mui/material';
import { createContext, type ReactNode, useContext, useState } from 'react';
import { useLocation } from 'react-router';
import { RegistroDeSalvamento } from '@/shared/autosave/registroDeSalvamento';
import { MolduraDoStudio } from '../MolduraDoStudio';

type Detalhe = {
  /** Onde o cabeçalho da tela se desenha; null até a moldura montar. */
  vaga: HTMLElement | null;
  /** Último autosave bem-sucedido nesta tela (mesmo caminho); null se ainda não houve. */
  salvoEm: Date | null;
};

// undefined = fora do layout de detalhe (ex.: teste de uma tela isolada).
const DetalheContext = createContext<Detalhe | undefined>(undefined);

export function useDetalhe(): Detalhe | undefined {
  return useContext(DetalheContext);
}

type Salvamento = { caminho: string; em: Date };

/**
 * Layout das telas de detalhe (skill, condomínio, Avançado): sem barra global nem menu.
 * A própria tela desenha o cabeçalho (CabecalhoDeDetalhe), que vai para a vaga do topo.
 */
export function LayoutDeDetalhe(): ReactNode {
  const { pathname } = useLocation();
  const [vaga, setVaga] = useState<HTMLElement | null>(null);
  const [ultimo, setUltimo] = useState<Salvamento | null>(null);
  const salvoEm = ultimo?.caminho === pathname ? ultimo.em : null;
  const registrar = (em: Date) => setUltimo({ caminho: pathname, em });

  return (
    <DetalheContext value={{ vaga, salvoEm }}>
      <RegistroDeSalvamento value={registrar}>
        <MolduraDoStudio topo={<Box ref={setVaga} sx={{ flexShrink: 0 }} />} />
      </RegistroDeSalvamento>
    </DetalheContext>
  );
}

/**
 * Moldura padrão do conteúdo de detalhe (34 px em cima e embaixo, 55 nas laterais, 21 entre
 * blocos). O layout não aplica sozinho: o Fluxo e o Avançado usam moldura própria.
 */
export const ESTILO_DO_CONTEUDO_DE_DETALHE = {
  py: 4,
  px: 5,
  display: 'flex',
  flexDirection: 'column',
  gap: 3,
} as const;
