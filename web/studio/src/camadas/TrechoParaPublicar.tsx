import { Box, Checkbox, FormControlLabel, Paper, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import { PESOS, RAIO } from '@/theme/tokens';
import { ConteudoDoTrecho } from './ConteudoDoTrecho';
import type { Diferenca } from './contratos';
import { lerConteudo, type RotulosDasEtapas } from './leituraDoTrecho';

type Props = {
  diferenca: Diferenca;
  rotulosDasEtapas: RotulosDasEtapas;
  obrigatorio: boolean;
  onMudarObrigatorio: (obrigatorio: boolean) => void;
};

/** Um trecho que mudou desde a última versão: como era, como fica e a opção de torná-lo obrigatório. */
export function TrechoParaPublicar({
  diferenca,
  rotulosDasEtapas,
  obrigatorio,
  onMudarObrigatorio,
}: Props): ReactNode {
  const coluna = (valor: string | null) =>
    valor === null ? null : lerConteudo(diferenca.tipo, valor, rotulosDasEtapas);
  return (
    <Paper
      component="section"
      variant="outlined"
      aria-label={diferenca.rotulo}
      sx={{ p: 1.5, display: 'flex', flexDirection: 'column', gap: 1 }}
    >
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
        <Typography variant="subtitle2" component="h3" sx={{ flex: 1 }}>
          {diferenca.rotulo}
        </Typography>
        <FormControlLabel
          label="Obrigatória"
          control={
            <Checkbox
              size="small"
              checked={obrigatorio}
              onChange={(evento) => onMudarObrigatorio(evento.target.checked)}
              slotProps={{ input: { 'aria-label': `Obrigatória: ${diferenca.rotulo}` } }}
            />
          }
        />
      </Box>
      <Box sx={{ display: 'grid', gap: 1.5, gridTemplateColumns: { xs: '1fr', md: '1fr 1fr' } }}>
        <Coluna rotulo="Como era" conteudo={coluna(diferenca.antes)} vazio="Não existia" />
        <Coluna
          rotulo="Como fica"
          conteudo={coluna(diferenca.depois)}
          vazio="Será removido"
          destaque
        />
      </Box>
    </Paper>
  );
}

type PropsDaColuna = {
  rotulo: string;
  conteudo: ReturnType<typeof lerConteudo> | null;
  vazio: string;
  destaque?: boolean;
};

function Coluna({ rotulo, conteudo, vazio, destaque = false }: PropsDaColuna): ReactNode {
  return (
    <Box
      role="group"
      aria-label={rotulo}
      sx={{
        p: 1.5,
        minWidth: 0,
        borderRadius: RAIO.campo,
        bgcolor: destaque ? 'background.tint' : 'background.subtle',
      }}
    >
      <Typography
        variant="caption"
        color="textSecondary"
        sx={{ display: 'block', fontWeight: PESOS.semi }}
      >
        {rotulo}
      </Typography>
      {conteudo === null ? (
        <Typography color="textSecondary" sx={{ fontStyle: 'italic' }}>
          {vazio}
        </Typography>
      ) : (
        <ConteudoDoTrecho conteudo={conteudo} />
      )}
    </Box>
  );
}
