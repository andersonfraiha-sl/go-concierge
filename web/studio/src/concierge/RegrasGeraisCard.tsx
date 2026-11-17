import { Box, Paper, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import { regrasDe, tituloDaRegra } from '@/skills/abas/arquivosDaSkill';
import { PESOS } from '@/theme/tokens';
import { BotaoExcluir, type ItemDaLista, ListaDeArquivosCard } from './ListaDeArquivosCard';

const TEXTOS = {
  erro: 'Não foi possível carregar as regras.',
  vazio: 'Esta skill ainda não tem regras.',
  excluir: (caminho: string) =>
    `Excluir a regra "${tituloDaRegra(caminho)}"? A skill deixa de segui-la.`,
};

/** Regras gerais como cartões fechados: título, resumo e excluir. */
export function RegrasGeraisCard({ assistente }: { assistente: string }): ReactNode {
  return (
    <ListaDeArquivosCard
      assistente={assistente}
      tipo="regra"
      titulo="Regras gerais"
      ajuda="Valem para todas as skills."
      rotuloDoNovo="Nova regra"
      textos={TEXTOS}
      arquivosDe={regrasDe}
      item={(props) => <CartaoDeRegra {...props} />}
    />
  );
}

function CartaoDeRegra({ arquivo, emExclusao, onExcluir }: ItemDaLista): ReactNode {
  const titulo = tituloDaRegra(arquivo.caminho);
  return (
    <Paper
      variant="outlined"
      sx={{
        p: 2,
        display: 'flex',
        alignItems: 'flex-start',
        gap: 1,
        borderColor: 'border.default',
        opacity: emExclusao ? 0.5 : 1,
      }}
    >
      <Box sx={{ flex: 1, minWidth: 0 }}>
        <Typography variant="body1" sx={{ fontWeight: PESOS.semi }}>
          {titulo}
        </Typography>
        {arquivo.resumo && (
          <Typography variant="body2" color="textSecondary">
            {arquivo.resumo}
          </Typography>
        )}
      </Box>
      {onExcluir && <BotaoExcluir nome={titulo} emExclusao={emExclusao} onExcluir={onExcluir} />}
    </Paper>
  );
}
