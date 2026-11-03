import { Box, Button, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import { SectionCard } from '@/shared/ui/SectionCard';
import { useProjecao } from './contratos';
import { ESTILO_DE_CODIGO } from './estilo';

export function ProjecaoAba({ caminho }: { caminho: string }): ReactNode {
  const projecao = useProjecao(caminho);
  if (projecao.isPending) return <Typography color="text.secondary">Carregando…</Typography>;
  if (projecao.isError) {
    return (
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
        <Typography role="alert">Não foi possível montar o prompt.</Typography>
        <Button size="small" onClick={() => void projecao.refetch()}>
          Tentar de novo
        </Button>
      </Box>
    );
  }
  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
      <SectionCard
        titulo="Prompt montado"
        ajuda="O texto que o modelo recebe para este item. Só leitura."
      >
        <Typography component="pre" sx={ESTILO_DE_CODIGO}>
          {projecao.data.projecao}
        </Typography>
      </SectionCard>
      <SectionCard
        titulo="Roteador"
        ajuda="O texto que escolhe a rota entre as skills. Só leitura."
      >
        <Typography component="pre" sx={ESTILO_DE_CODIGO}>
          {projecao.data.roteador}
        </Typography>
      </SectionCard>
    </Box>
  );
}
