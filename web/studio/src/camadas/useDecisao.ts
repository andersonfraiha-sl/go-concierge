import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from 'react-router';
import { useSnackbar } from '@/shared/snackbar/SnackbarContext';
import { rotaDaCamada } from './camadaDaRota';
import {
  type Atualizacao,
  CHAVE_DAS_ATUALIZACOES,
  chaveDaAtualizacao,
  chaveDaCamada,
  chaveDoHistorico,
  decidir,
} from './contratos';
import { type Escolhas, paraEnvio } from './decisoes';

/** Envia a decisão; ao dar certo, volta ao editor da camada e marca como velho o que a decisão mexeu. */
export function useDecisao(atualizacao: Atualizacao, escolhas: Escolhas) {
  const cliente = useQueryClient();
  const navegar = useNavigate();
  const { mostrar } = useSnackbar();
  const { id, camada, cabeca } = atualizacao;

  return useMutation({
    mutationFn: () => decidir(id, { cabeca, decisoes: paraEnvio(escolhas) }),
    onSuccess: async () => {
      mostrar('Atualização decidida');
      void navegar(rotaDaCamada(camada));
      // Depois de navegar: a tela já saiu, então as consultas só ficam velhas, sem buscar de novo aqui.
      await Promise.all(
        [
          CHAVE_DAS_ATUALIZACOES,
          chaveDaAtualizacao(id),
          chaveDaCamada(camada),
          chaveDoHistorico(camada),
        ].map((queryKey) => cliente.invalidateQueries({ queryKey })),
      );
    },
  });
}
