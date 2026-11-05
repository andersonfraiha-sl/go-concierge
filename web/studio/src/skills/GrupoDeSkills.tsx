import { Box, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import { PESOS, TOKENS_DA_SKILL } from '@/theme/tokens';
import type { SkillResumo } from './contratos';

type Props = {
  titulo: string;
  subtitulo: string;
  colunas: number;
  skills: readonly SkillResumo[];
  renderCartao: (skill: SkillResumo) => ReactNode;
};

/** Seção de skills com título, subtítulo e grade; some quando não há skills. */
export function GrupoDeSkills({ skills, colunas, renderCartao, ...cabecalho }: Props): ReactNode {
  if (skills.length === 0) return null;
  return (
    <Box component="section" sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
      <CabecalhoDoGrupo {...cabecalho} />
      <Box
        sx={{
          display: 'grid',
          gridTemplateColumns: `repeat(${colunas}, minmax(0, 1fr))`,
          gap: 1.5,
        }}
      >
        {skills.map(renderCartao)}
      </Box>
    </Box>
  );
}

function CabecalhoDoGrupo({ titulo, subtitulo }: Pick<Props, 'titulo' | 'subtitulo'>): ReactNode {
  return (
    <Box sx={{ display: 'flex', alignItems: 'baseline', gap: 1 }}>
      <Typography
        component="h2"
        variant="overline"
        sx={{ color: 'text.secondary', fontWeight: PESOS.negrito }}
      >
        {titulo}
      </Typography>
      <Typography variant="caption" sx={{ color: TOKENS_DA_SKILL.textoMuitoSuave }}>
        {subtitulo}
      </Typography>
    </Box>
  );
}
