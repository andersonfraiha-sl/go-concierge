import { useCallback, useRef } from 'react';

type SingleFlight = {
  start(): Promise<void>;
  flush(deNovo: () => boolean): Promise<void>;
};

type EmVoo = { current: Promise<void> | null };

async function esperarEDecidir(
  emVoo: EmVoo,
  start: () => Promise<void>,
  deNovo: () => boolean,
): Promise<void> {
  const rodando = emVoo.current;
  if (!rodando) return start();
  await rodando;
  const outra = emVoo.current;
  if (outra) return outra;
  if (deNovo()) await start();
}

/**
 * Uma gravação por vez. Quem chama `flush` com uma gravação no ar espera ela terminar e só então
 * decide, via `deNovo`, se ainda há o que gravar. `deNovo` é parâmetro de `flush` (e não do hook)
 * porque é construído no instante da chamada, antes do `await`.
 */
export function useSingleFlight(salvar: () => Promise<void>): SingleFlight {
  const emVoo = useRef<Promise<void> | null>(null);

  const start = useCallback((): Promise<void> => {
    const iniciada = salvar().finally(() => {
      emVoo.current = null;
    });
    emVoo.current = iniciada;
    return iniciada;
  }, [salvar]);

  const flush = useCallback(
    (deNovo: () => boolean) => esperarEDecidir(emVoo, start, deNovo),
    [start],
  );

  return { start, flush };
}
