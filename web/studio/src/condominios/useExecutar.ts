import { useState } from 'react';
import { mensagemDe } from '@/shared/api/mensagemDe';
import { useSnackbar } from '@/shared/snackbar/SnackbarContext';
import { type Escopo, type RespostaMapeamento, useAplicarMapeamento, useEscopo } from './contratos';

type Gravacao = (escopo: Escopo) => Promise<RespostaMapeamento>;

type Execucao = {
  /** Roda uma gravação, aplica a resposta no cache e avisa no rodapé; devolve se deu certo. */
  executar: (gravacao: Gravacao, sucesso?: string) => Promise<boolean>;
  /** Há gravação em andamento: quem usa desabilita os controles para não enviar dado velho. */
  gravando: boolean;
};

export function useExecutar(): Execucao {
  const aplicar = useAplicarMapeamento();
  const escopo = useEscopo() ?? null;
  const { mostrar } = useSnackbar();
  const [gravando, setGravando] = useState(false);

  async function executar(gravacao: Gravacao, sucesso?: string): Promise<boolean> {
    setGravando(true);
    try {
      aplicar(await gravacao(escopo));
    } catch (erro) {
      mostrar(mensagemDe(erro), 'error');
      return false;
    } finally {
      setGravando(false);
    }
    if (sucesso !== undefined) mostrar(sucesso);
    return true;
  }

  return { executar, gravando };
}
