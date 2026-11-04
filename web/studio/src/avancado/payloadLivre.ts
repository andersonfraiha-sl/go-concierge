import type { Valor } from '@/condominios/contratos';

export type ParDoPayload = { chave: string; valor: string };

export const payloadEmPares = (payload: Record<string, Valor>): ParDoPayload[] =>
  Object.entries(payload).map(([chave, valor]) => ({
    chave,
    valor: typeof valor === 'string' ? valor : JSON.stringify(valor),
  }));

function lerValor(texto: string): Valor {
  if (texto === 'true') return true;
  if (texto === 'false') return false;
  const numero = Number(texto);
  return texto.trim() !== '' && Number.isFinite(numero) ? numero : texto;
}

export const paresEmPayload = (pares: readonly ParDoPayload[]): Record<string, Valor> =>
  Object.fromEntries(
    pares
      .filter((par) => par.chave.trim() !== '')
      .map((par) => [par.chave.trim(), lerValor(par.valor)]),
  );
