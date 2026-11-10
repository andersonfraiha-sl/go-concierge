import type { ReactNode } from 'react';
import { CampoTexto } from '@/shared/ui/CampoTexto';
import { SectionCard } from '@/shared/ui/SectionCard';
import type { RascunhoDaCamada } from './useRascunhoDaCamada';

export function OrientacaoDoCondominio({
  rascunho,
  editavel,
}: {
  rascunho: RascunhoDaCamada;
  editavel: boolean;
}): ReactNode {
  return (
    <SectionCard
      titulo="Orientação para este condomínio"
      ajuda="Um recado do condomínio para o concierge seguir nesta versão."
    >
      <CampoTexto
        control={rascunho.controlDaOrientacao}
        name="orientacao"
        multiline
        minRows={3}
        fullWidth
        disabled={!editavel}
        slotProps={{ htmlInput: { 'aria-label': 'Orientação para este condomínio' } }}
      />
    </SectionCard>
  );
}
