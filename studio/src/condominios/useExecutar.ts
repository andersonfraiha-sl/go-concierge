import { useSnackbar } from '@/shared/snackbar/SnackbarContext';
import { useAplicarMapeamento, useEscopo, type Escopo, type RespostaMapeamento } from './contratos';

type Gravacao = (escopo: Escopo) => Promise<RespostaMapeamento>;

/** Roda uma gravação, aplica a resposta no cache e avisa no rodapé; devolve se deu certo. */
export function useExecutar(): (gravacao: Gravacao, sucesso?: string) => Promise<boolean> {
  const aplicar = useAplicarMapeamento();
  const escopo = useEscopo() ?? null;
  const { mostrar } = useSnackbar();

  return async (gravacao, sucesso) => {
    try {
      aplicar(await gravacao(escopo));
    } catch (erro) {
      mostrar(erro instanceof Error ? erro.message : String(erro), 'error');
      return false;
    }
    if (sucesso !== undefined) mostrar(sucesso);
    return true;
  };
}
