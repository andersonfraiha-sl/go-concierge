import { describe, expect, it } from 'vitest';
import { type ParDoPayload, paresEmPayload, payloadEmPares } from './payloadLivre';

describe('paresEmPayload', () => {
  const casos: [string, ParDoPayload[], Record<string, unknown>][] = [
    ['dropsEmptyKeys', [{ chave: '  ', valor: 'x' }], {}],
    [
      'readsBooleans',
      [
        { chave: 'a', valor: 'true' },
        { chave: 'b', valor: 'false' },
      ],
      { a: true, b: false },
    ],
    [
      'readsFiniteNumbers',
      [
        { chave: 'a', valor: '3' },
        { chave: 'b', valor: '-1.5' },
      ],
      { a: 3, b: -1.5 },
    ],
    [
      'keepsOtherTextAsText',
      [
        { chave: 'a', valor: '7h às 22h' },
        { chave: 'b', valor: 'Infinity' },
        { chave: 'c', valor: '' },
      ],
      { a: '7h às 22h', b: 'Infinity', c: '' },
    ],
  ];
  it.each(casos)('convertsPairsAndValues: %s', (_nome, pares, esperado) => {
    expect(paresEmPayload(pares)).toEqual(esperado);
  });
});

describe('payloadEmPares', () => {
  it('roundTripsPayload', () => {
    const payload = {
      limite: 3,
      ativo: false,
      horario: '7h às 22h',
      faixas: [1, 2],
      extra: { a: 1 },
    };
    expect(paresEmPayload(payloadEmPares(payload))).toEqual({
      ...payload,
      faixas: '[1,2]',
      extra: '{"a":1}',
    });
  });

  it('writesNonTextValuesAsJson', () => {
    expect(payloadEmPares({ faixas: [1], nulo: null })).toEqual([
      { chave: 'faixas', valor: '[1]' },
      { chave: 'nulo', valor: 'null' },
    ]);
  });
});
