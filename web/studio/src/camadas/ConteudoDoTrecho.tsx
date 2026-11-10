import { Box, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import { PESOS } from '@/theme/tokens';
import type { ConteudoLegivel } from './leituraDoTrecho';

/** O conteúdo de um trecho como a pessoa lê: texto, cartão de etapa ou lista numerada. */
export function ConteudoDoTrecho({ conteudo }: { conteudo: ConteudoLegivel }): ReactNode {
  switch (conteudo.forma) {
    case 'texto':
      return <Typography sx={{ whiteSpace: 'pre-wrap' }}>{conteudo.texto}</Typography>;
    case 'cartao':
      return (
        <Box>
          <Typography variant="body2" sx={{ fontWeight: PESOS.semi }}>
            {conteudo.titulo}
          </Typography>
          {conteudo.texto !== '' && <Typography>{conteudo.texto}</Typography>}
        </Box>
      );
    case 'lista':
      return (
        <Box component="ol" sx={{ m: 0, pl: 2.5 }}>
          {conteudo.itens.map((item) => (
            <li key={item.id}>{item.rotulo}</li>
          ))}
        </Box>
      );
    default:
      return assertNever(conteudo);
  }
}

function assertNever(valor: never): never {
  throw new Error(`conteúdo não tratado: ${JSON.stringify(valor)}`);
}
