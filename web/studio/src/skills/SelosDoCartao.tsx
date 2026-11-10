import { Box } from '@mui/material';
import type { ReactNode } from 'react';
import { PESOS, RAIO, TIPOS, TOKENS_DA_SKILL } from '@/theme/tokens';
import type { Problemas } from './estatisticas';

const rotuloDeErros = (quantidade: number): string =>
  quantidade === 1 ? '1 erro' : `${quantidade} erros`;

const rotuloDeAvisos = (quantidade: number): string =>
  quantidade === 1 ? '1 aviso' : `${quantidade} avisos`;

function BadgeDoCartao({
  label,
  bgcolor,
  color,
}: {
  label: string;
  bgcolor: string;
  color: string;
}) {
  return (
    <Box
      component="span"
      sx={{
        fontSize: TIPOS.overline,
        fontWeight: PESOS.medio,
        lineHeight: 1,
        py: 0.5,
        px: 1.25,
        borderRadius: RAIO.pilula,
        bgcolor,
        color,
      }}
    >
      {label}
    </Box>
  );
}

export function SeloDeAtualizacao(): ReactNode {
  return (
    <Box sx={{ display: 'flex', mt: 1.5 }}>
      <BadgeDoCartao label="Atualização" bgcolor="background.tint" color="primary.main" />
    </Box>
  );
}

export function ChipsDeProblemas({ problemas }: { problemas: Problemas | undefined }): ReactNode {
  const erros = problemas?.erros ?? 0;
  const avisos = problemas?.avisos ?? 0;
  if (erros === 0 && avisos === 0) return null;
  return (
    <Box sx={{ display: 'flex', gap: 1, mt: 1.5, flexWrap: 'wrap' }}>
      {erros > 0 && (
        <BadgeDoCartao
          label={rotuloDeErros(erros)}
          bgcolor={TOKENS_DA_SKILL.erroFundoSuave}
          color="error.main"
        />
      )}
      {avisos > 0 && (
        <BadgeDoCartao
          label={rotuloDeAvisos(avisos)}
          bgcolor="studio.registra.soft"
          color="studio.registra.onSoft"
        />
      )}
    </Box>
  );
}
