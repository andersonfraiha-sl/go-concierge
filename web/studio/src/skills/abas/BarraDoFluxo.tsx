import LockOutlinedIcon from '@mui/icons-material/LockOutlined';
import { Box, Chip, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import type { ProblemaFluxo } from '@/fluxo/modelo/tipos';
import { SaveIndicator } from '@/shared/autosave/SaveIndicator';
import type { EstadoSalvamento } from '@/shared/autosave/useAutosave';
import { tokens } from '@/theme/tokens';
import { chaveDoProblema, corDoNivel } from './fluxoDaSkill';

type Props = {
  editavel: boolean;
  /** Problemas sem bloco: valem para o fluxo todo. */
  doFluxoTodo: readonly ProblemaFluxo[];
  salvamento: EstadoSalvamento;
  onTentarDeNovo: () => void;
};

/** Linha acima do canvas: só leitura, problemas do fluxo todo e o estado do salvamento. */
export function BarraDoFluxo({ editavel, doFluxoTodo, salvamento, onTentarDeNovo }: Props) {
  return (
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, minHeight: 32 }}>
      {!editavel && <ChipSoLeitura />}
      <ProblemasDoFluxoTodo problemas={doFluxoTodo} />
      <Box sx={{ ml: 'auto' }}>
        <SaveIndicator estado={salvamento} onTentarDeNovo={onTentarDeNovo} />
      </Box>
    </Box>
  );
}

function ChipSoLeitura(): ReactNode {
  return (
    <Chip
      icon={<LockOutlinedIcon fontSize="small" />}
      label="Só leitura"
      size="small"
      sx={{ bgcolor: tokens.color.background.subtle, color: tokens.color.text.secondary }}
    />
  );
}

function ProblemasDoFluxoTodo({ problemas }: { problemas: readonly ProblemaFluxo[] }): ReactNode {
  return problemas.map((p) => (
    <Typography key={chaveDoProblema(p)} variant="caption" sx={{ color: corDoNivel(p.nivel) }}>
      {p.mensagem}
    </Typography>
  ));
}
