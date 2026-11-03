import { useCallback, useEffect, useRef, useState } from 'react';
import { useSingleFlight } from './useSingleFlight';

export const ATRASO_SALVAMENTO_MS = 1000;

export type EstadoSalvamento =
  | { tipo: 'em_dia' }
  | { tipo: 'salvando' }
  | { tipo: 'salvo'; em: Date }
  | { tipo: 'erro'; mensagem: string }
  | { tipo: 'bloqueado'; motivo: string };

type Opcoes<T> = {
  valor: T;
  salvar: (valor: T) => Promise<void>;
  atrasoMs?: number;
  bloqueio?: string | null;
};

type Resultado = {
  estado: EstadoSalvamento;
  tentarDeNovo: () => Promise<void>;
  salvarAgora: () => Promise<void>;
};

const mensagemDe = (erro: unknown): string => (erro instanceof Error ? erro.message : String(erro));

/**
 * Grava `atrasoMs` depois da última mudança de referência de `valor`, uma gravação por vez.
 * O primeiro `valor` é a base e não grava. Falha para a gravação automática até `tentarDeNovo`.
 */
export function useAutosave<T>({
  valor,
  salvar,
  atrasoMs = ATRASO_SALVAMENTO_MS,
  bloqueio = null,
}: Opcoes<T>): Resultado {
  const [interno, setInterno] = useState<EstadoSalvamento>({ tipo: 'em_dia' });
  const valorAtual = useRef(valor);
  const salvarAtual = useRef(salvar);
  const bloqueioAtual = useRef(bloqueio);
  const valorBase = useRef(valor);
  const pendente = useRef(false);
  const falhou = useRef(false);
  const montado = useRef(true);

  useEffect(() => {
    valorAtual.current = valor;
    salvarAtual.current = salvar;
    bloqueioAtual.current = bloqueio;
  });

  const gravarPendente = useCallback(async (): Promise<void> => {
    if (!pendente.current || bloqueioAtual.current !== null) return;
    const enviado = valorAtual.current;
    pendente.current = false;
    setInterno({ tipo: 'salvando' });
    try {
      await salvarAtual.current(enviado);
      if (montado.current) setInterno({ tipo: 'salvo', em: new Date() });
    } catch (erro) {
      pendente.current = true;
      falhou.current = true;
      if (!montado.current) {
        console.error('salvar ao sair da tela', erro);
        return;
      }
      setInterno({ tipo: 'erro', mensagem: mensagemDe(erro) });
    }
  }, []);

  const { flush } = useSingleFlight(gravarPendente);

  const haOQueGravar = useCallback(
    () => pendente.current && !falhou.current && bloqueioAtual.current === null,
    [],
  );

  const salvarAgora = useCallback((): Promise<void> => {
    falhou.current = false;
    return flush(haOQueGravar);
  }, [flush, haOQueGravar]);

  useEffect(() => {
    if (Object.is(valor, valorBase.current)) return;
    valorBase.current = valor;
    pendente.current = true;
  }, [valor]);

  useEffect(() => {
    if (!pendente.current || falhou.current || bloqueio !== null) return;
    const timer = setTimeout(() => void flush(haOQueGravar), atrasoMs);
    return () => clearTimeout(timer);
  }, [valor, bloqueio, atrasoMs, flush, haOQueGravar]);

  const salvarAgoraRef = useRef(salvarAgora);
  useEffect(() => {
    salvarAgoraRef.current = salvarAgora;
  });

  useEffect(() => {
    montado.current = true;
    return () => {
      montado.current = false;
      void salvarAgoraRef.current();
    };
  }, []);

  const estado: EstadoSalvamento = bloqueio === null ? interno : { tipo: 'bloqueado', motivo: bloqueio };
  return { estado, tentarDeNovo: salvarAgora, salvarAgora };
}
