import { Box, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import { useAgente } from '@/catalogo/contratos';
import { CAMPO, PESOS } from '@/theme/tokens';
import { ConciergeCard } from './ConciergeCard';
import { Falha } from './PersonaAba';

/**
 * "Quem pode conversar" e o canal (WhatsApp) do board não têm dado real hoje: a API não expõe
 * visibilidade nem canal no nível do assistente raiz (só por skill, no arquivo do fluxo). Por
 * isso o cartão mostra só o nome, de leitura (sem endpoint de gravação para um "nome de exibição").
 */
export function IdentidadeCard({ assistente }: { assistente: string }): ReactNode {
  const agente = useAgente(assistente);
  if (agente.isPending) return <Typography color="text.secondary">Carregando…</Typography>;
  if (agente.isError) return <Falha aoTentar={() => void agente.refetch()} />;
  return (
    <ConciergeCard
      titulo="Identidade"
      ajuda="Como o concierge se apresenta e com quem ele conversa."
    >
      <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.5, maxWidth: 320 }}>
        <Typography variant="body2" sx={{ fontWeight: PESOS.semi }}>
          Nome
        </Typography>
        <Box
          sx={{
            borderRadius: `${CAMPO.raio}px`,
            border: 1,
            borderColor: CAMPO.bordaEmRepouso,
            px: `${CAMPO.paddingX}px`,
            py: `${CAMPO.paddingY}px`,
            color: 'text.primary',
          }}
        >
          {agente.data.agente.nome}
        </Box>
      </Box>
    </ConciergeCard>
  );
}
