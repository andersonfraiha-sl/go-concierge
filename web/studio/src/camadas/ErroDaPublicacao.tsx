import { Alert, Box } from '@mui/material';
import type { ReactNode } from 'react';
import { useEstado } from '@/condominios/contratos';
import type { CamadaQuebrada, ErroDaPublicacao as Erro } from './regrasDaPublicacao';

/** Nome de cada camada pelo id, para "Condomínio Vila Verde" no lugar do identificador. */
function useNomeDaCamada(): (camada: CamadaQuebrada) => string {
  const { data: estado } = useEstado();
  return (camada) => {
    if (camada.tipo === 'administradora') {
      const nome = estado?.administradoras.find((a) => a.id === camada.id)?.nome ?? camada.id;
      return `Administradora ${nome}`;
    }
    const nome = estado?.mapeamento.condominios.find((c) => c.id === camada.id)?.nome ?? camada.id;
    return `Condomínio ${nome}`;
  };
}

export function ErroDaPublicacao({ erro }: { erro: Erro }): ReactNode {
  const nomeDaCamada = useNomeDaCamada();
  switch (erro.tipo) {
    case 'quebram':
      return (
        <Alert severity="error">
          Esta versão deixaria camadas sem funcionar. Corrija nelas antes de publicar:
          <Box component="ul" sx={{ m: 0, pl: 2.5 }}>
            {erro.camadas.flatMap((camada) =>
              camada.problemas.map((problema) => (
                <li key={`${camada.tipo}-${camada.id}-${problema.codigo}-${problema.mensagem}`}>
                  {nomeDaCamada(camada)}: {problema.mensagem}
                </li>
              )),
            )}
          </Box>
        </Alert>
      );
    case 'problemas':
      return (
        <Alert severity="error">
          {erro.problemas.map((problema) => (
            <div key={`${problema.codigo}-${problema.mensagem}`}>{problema.mensagem}</div>
          ))}
        </Alert>
      );
    case 'outro':
      return <Alert severity="error">{erro.mensagem}</Alert>;
    default:
      return assertNever(erro);
  }
}

function assertNever(valor: never): never {
  throw new Error(`erro não tratado: ${JSON.stringify(valor)}`);
}
