import { useEffect, useRef, useState } from 'react';

const NENHUM_ALVO: ReadonlySet<string> = new Set();
const DURACAO_DO_REALCE_MS = 1500;

type Prazos = Map<string, ReturnType<typeof setTimeout>>;

function usePrazosPorAlvo(): Prazos {
  const prazos = useRef<Prazos>(new Map());
  useEffect(() => {
    const prazosAtivos = prazos.current;
    return () => {
      for (const prazo of prazosAtivos.values()) clearTimeout(prazo);
      prazosAtivos.clear();
    };
  }, []);
  return prazos.current;
}

/** Um prazo por alvo: realçar de novo reinicia os 1500 ms daquele alvo; sair da tela limpa todos. */
export function useAlvosRealcados(): [ReadonlySet<string>, (alvos: readonly string[]) => void] {
  const [alvosRealcados, setAlvosRealcados] = useState<ReadonlySet<string>>(NENHUM_ALVO);
  const prazos = usePrazosPorAlvo();

  const apagar = (alvo: string) => {
    prazos.delete(alvo);
    setAlvosRealcados((atuais) => new Set([...atuais].filter((atual) => atual !== alvo)));
  };

  const agendarFim = (alvo: string) => {
    clearTimeout(prazos.get(alvo));
    prazos.set(
      alvo,
      setTimeout(() => apagar(alvo), DURACAO_DO_REALCE_MS),
    );
  };

  const realcar = (alvos: readonly string[]) => {
    setAlvosRealcados((atuais) => new Set([...atuais, ...alvos]));
    alvos.forEach(agendarFim);
  };

  return [alvosRealcados, realcar];
}
