import { Box } from '@mui/material';
import type { ReactNode } from 'react';
import { atualizarBloco } from '@/fluxo/modelo/edicao';
import type { Bloco, Fluxo, ProblemaFluxo } from '@/fluxo/modelo/tipos';
import { BlocoPainel } from '@/fluxo/painel/BlocoPainel';
import type { CatalogoDeFerramentas } from '@/fluxo/painel/camposPorTipo';
import { InformacoesPainel } from '@/fluxo/painel/InformacoesPainel';
import { tokens } from '@/theme/tokens';

type Props = {
  fluxo: Fluxo;
  /** Ausente: mostra as informações que o fluxo pede. */
  bloco: Bloco | undefined;
  problemas: readonly ProblemaFluxo[];
  catalogo: CatalogoDeFerramentas;
  editavel: boolean;
  onMudarFluxo: (mudar: (atual: Fluxo) => Fluxo) => void;
  onExcluir: (id: string) => void;
};

/** Painel à direita do canvas: o bloco selecionado ou, sem seleção, as informações do fluxo. */
export function PainelLateralDoFluxo({
  fluxo,
  bloco,
  problemas,
  catalogo,
  editavel,
  onMudarFluxo,
  onExcluir,
}: Props): ReactNode {
  return (
    <Box
      component="aside"
      sx={{
        width: tokens.layout.painelLateral,
        flexShrink: 0,
        borderLeft: 1,
        borderColor: 'divider',
        overflowY: 'auto',
        bgcolor: 'background.paper',
      }}
    >
      {bloco ? (
        <BlocoPainel
          key={bloco.id}
          bloco={bloco}
          problemas={problemas.filter((p) => p.blocoId === bloco.id)}
          catalogo={catalogo}
          informacoes={fluxo.informacoes}
          editavel={editavel}
          onMudar={(mudado) => onMudarFluxo((atual) => atualizarBloco(atual, mudado))}
          onExcluir={onExcluir}
        />
      ) : (
        <InformacoesPainel
          informacoes={fluxo.informacoes}
          editavel={editavel}
          onMudar={(informacoes) => onMudarFluxo((atual) => ({ ...atual, informacoes }))}
        />
      )}
    </Box>
  );
}
