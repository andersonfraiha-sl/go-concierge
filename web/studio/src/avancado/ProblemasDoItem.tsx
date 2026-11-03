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
        <Box key={`${problema.onde}-${problema.texto}`}>
          <Typography variant="body2" color={problema.nivel === 'erro' ? 'error' : 'text.primary'}>
            {problema.nivel === 'erro' ? 'Erro' : 'Aviso'} em {problema.onde}
          </Typography>
          <Typography variant="body2">{problema.texto}</Typography>
          {problema.solucao && (
            <Typography variant="caption" color="text.secondary">
              {problema.solucao}
            </Typography>
          )}
        </Box>
      ))}
    </SectionCard>
  );
}
