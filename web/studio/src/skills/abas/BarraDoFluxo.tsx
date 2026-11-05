import LockOutlinedIcon from '@mui/icons-material/LockOutlined';
import { Box, Chip, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import type { ProblemaFluxo } from '@/fluxo/modelo/tipos';
import { SaveIndicator } from '@/shared/autosave/SaveIndicator';
import type { EstadoSalvamento } from '@/shared/autosave/useAutosave';
import { PESOS, RAIO, TIPOS, TOKENS_DA_SKILL } from '@/theme/tokens';
import { chaveDoProblema, corDoNivel } from './fluxoDaSkill';

type Props = {
  editavel: boolean;
  totalDePassos: number;
  /** Todos os problemas do fluxo (do fluxo todo e de cada bloco): decide o selo "Sem problemas". */
  problemas: readonly ProblemaFluxo[];
  /** Problemas sem bloco: valem para o fluxo todo. */
  doFluxoTodo: readonly ProblemaFluxo[];
  salvamento: EstadoSalvamento;
  onTentarDeNovo: () => void;
};

const rotuloDePassos = (total: number): string =>
  `${total} ${total === 1 ? 'passo' : 'passos'} · o concierge segue de cima para baixo`;

/** Linha acima do canvas: contagem de passos, selo de problemas e o estado do salvamento. */
export function BarraDoFluxo({
  editavel,
  totalDePassos,
  problemas,
  doFluxoTodo,
  salvamento,
  onTentarDeNovo,
}: Props) {
  return (
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, minHeight: 32 }}>
      {!editavel && <ChipSoLeitura />}
      <Typography variant="body2" sx={{ color: 'text.secondary' }}>
        {rotuloDePassos(totalDePassos)}
      </Typography>
      <SeloDeProblemas problemas={problemas} />
      <ProblemasDoFluxoTodo problemas={doFluxoTodo} />
      <Box sx={{ ml: 'auto' }}>
        <SaveIndicator estado={salvamento} onTentarDeNovo={onTentarDeNovo} />
      </Box>
    </Box>
  );
}

function SeloDeProblemas({ problemas }: { problemas: readonly ProblemaFluxo[] }): ReactNode {
  const semProblemas = problemas.length === 0;
  const temErro = problemas.some((p) => p.nivel === 'erro');
  return (
    <Box
      component="span"
      sx={{
        fontSize: TIPOS.caption,
        fontWeight: PESOS.medio,
        py: 0.5,
        px: 1.25,
        borderRadius: RAIO.pilula,
        bgcolor: semProblemas
          ? 'success.soft'
          : temErro
            ? TOKENS_DA_SKILL.erroFundoSuave
            : 'warning.soft',
        color: semProblemas ? 'success.onSoft' : temErro ? 'error.main' : 'warning.onSoft',
      }}
    >
      {semProblemas ? 'Sem problemas' : `${problemas.length} para corrigir`}
    </Box>
  );
}

function ChipSoLeitura(): ReactNode {
  return (
    <Chip
      icon={<LockOutlinedIcon fontSize="small" />}
      label="Só leitura"
      size="small"
      sx={{ bgcolor: 'background.subtle', color: 'text.secondary' }}
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
