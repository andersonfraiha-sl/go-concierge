import { useEffect, useRef, useState } from 'react';

const NENHUM_ALVO: ReadonlySet<string> = new Set();
const DURACAO_DO_REALCE_MS = 1500;

/** Um prazo por alvo: realçar de novo reinicia os 1500 ms daquele alvo; sair da tela limpa todos. */
export function useAlvosRealcados(): [ReadonlySet<string>, (alvos: readonly string[]) => void] {
  const [alvosRealcados, setAlvosRealcados] = useState<ReadonlySet<string>>(NENHUM_ALVO);
  const prazos = useRef(new Map<string, ReturnType<typeof setTimeout>>());

  useEffect(() => {
    const prazosAtivos = prazos.current;
    return () => {
      for (const prazo of prazosAtivos.values()) clearTimeout(prazo);
      prazosAtivos.clear();
    };
  }, []);

  const apagar = (alvo: string) => {
    prazos.current.delete(alvo);
    setAlvosRealcados((atuais) => new Set([...atuais].filter((atual) => atual !== alvo)));
  };

  const realcar = (alvos: readonly string[]) => {
    setAlvosRealcados((atuais) => new Set([...atuais, ...alvos]));
    for (const alvo of alvos) {
      clearTimeout(prazos.current.get(alvo));
      prazos.current.set(
        alvo,
        setTimeout(() => apagar(alvo), DURACAO_DO_REALCE_MS),
      );
    }
  };

  return [alvosRealcados, realcar];
}
