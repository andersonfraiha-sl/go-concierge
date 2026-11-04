import { Box, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import { SectionCard } from '@/shared/ui/SectionCard';
import { useProjecao } from './contratos';
import { ESTILO_DE_CODIGO } from './estilo';
import { FalhaComTentarDeNovo } from './FalhaComTentarDeNovo';

export function ProjecaoAba({ caminho }: { caminho: string }): ReactNode {
  const projecao = useProjecao(caminho);
  if (projecao.isPending) return <Typography color="text.secondary">Carregando…</Typography>;
  if (projecao.isError) {
    return (
      <FalhaComTentarDeNovo
        mensagem="Não foi possível montar o prompt."
        onTentarDeNovo={() => void projecao.refetch()}
      />
    );
  }
  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
      <TextoSomenteLeitura
        titulo="Prompt montado"
        ajuda="O texto que o modelo recebe para este item. Só leitura."
        texto={projecao.data.projecao}
      />
      <TextoSomenteLeitura
        titulo="Roteador"
        ajuda="O texto que escolhe a rota entre as skills. Só leitura."
        texto={projecao.data.roteador}
      />
    </Box>
  );
}

type TextoProps = { titulo: string; ajuda: string; texto: string };

function TextoSomenteLeitura({ titulo, ajuda, texto }: TextoProps): ReactNode {
  return (
    <SectionCard titulo={titulo} ajuda={ajuda}>
      <Typography component="pre" sx={ESTILO_DE_CODIGO}>
        {texto}
      </Typography>
    </SectionCard>
  );
}
