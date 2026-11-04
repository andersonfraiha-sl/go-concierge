import { Box, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import type { Problema } from '@/catalogo/contratos';
import { SectionCard } from '@/shared/ui/SectionCard';

export function Problemas({ problemas }: { problemas: Problema[] }): ReactNode {
  return (
    <SectionCard titulo="Problemas" ajuda="O que o servidor achou de errado neste item.">
      {problemas.length === 0 && (
        <Typography variant="body2">Nenhum problema encontrado.</Typography>
      )}
      {problemas.map((problema) => (
        <ItemDeProblema key={`${problema.onde}-${problema.texto}`} problema={problema} />
      ))}
    </SectionCard>
  );
}

function ItemDeProblema({ problema }: { problema: Problema }): ReactNode {
  const ehErro = problema.nivel === 'erro';
  return (
    <Box>
      <Typography variant="body2" color={ehErro ? 'error' : 'text.primary'}>
        {ehErro ? 'Erro' : 'Aviso'} em {problema.onde}
      </Typography>
      <Typography variant="body2">{problema.texto}</Typography>
      {problema.solucao && (
        <Typography variant="caption" color="text.secondary">
          {problema.solucao}
        </Typography>
      )}
    </Box>
  );
}
