import { describe, expect, it } from 'vitest';
import {
  errosDosPares,
  type ParDoPayload,
  paresEmPayload,
  payloadEmPares,
  tipoInferido,
} from './payloadLivre';

const par = (chave: string, valor: string, tipo: ParDoPayload['tipo']): ParDoPayload => ({
  chave,
  valor,
  tipo,
  fixo: false,
});

describe('paresEmPayload', () => {
  const casos: [string, ParDoPayload[], Record<string, unknown>][] = [
    ['dropsEmptyKeys', [par('  ', 'x', 'texto')], {}],
    [
      'readsBooleans',
      [par('a', 'true', 'sim/não'), par('b', 'false', 'sim/não')],
      { a: true, b: false },
    ],
    ['readsNumbers', [par('a', '3', 'número'), par('b', '-1.5', 'número')], { a: 3, b: -1.5 }],
    [
      'keepsLookalikesAsText',
      [par('cep', '01310', 'texto'), par('b', 'true', 'texto')],
      { cep: '01310', b: 'true' },
    ],
    [
      'parsesJson',
      [par('a', '[1,2]', 'JSON'), par('b', 'null', 'JSON'), par('c', '{"x":1}', 'JSON')],
      { a: [1, 2], b: null, c: { x: 1 } },
    ],
  ];
  it.each(casos)('convertsPairsAndValues: %s', (_nome, pares, esperado) => {
    expect(paresEmPayload(pares)).toEqual(esperado);
  });
});

describe('payloadEmPares', () => {
  it('roundTripsPayloadUnchanged', () => {
    const payload = {
      limite: 3,
      ativo: false,
      cep: '01310',
      verdade: 'true',
      faixas: [1, 2],
      extra: { a: 1 },
      nulo: null,
    };
    expect(paresEmPayload(payloadEmPares(payload))).toEqual(payload);
  });

  it('keepsStoredTypes', () => {
    expect(payloadEmPares({ cep: '01310', nulo: null }).map((p) => p.tipo)).toEqual([
      'texto',
      'JSON',
    ]);
  });
});

describe('tipoInferido', () => {
  it.each([
    ['3', 'número'],
    ['true', 'sim/não'],
    ['abc', 'texto'],
    ['', 'texto'],
  ] as const)('infers %s as %s', (texto, tipo) => expect(tipoInferido(texto)).toBe(tipo));
});

describe('errosDosPares', () => {
  it('flagsDuplicateKeysAndInvalidValues', () => {
    const erros = errosDosPares([
      par('a', '1', 'número'),
      par('a', 'x', 'número'),
      par('b', '{', 'JSON'),
    ]);
    expect(erros).toEqual([
      { indice: 1, campo: 'chave', mensagem: 'Chave repetida' },
      { indice: 1, campo: 'valor', mensagem: 'Número inválido' },
      { indice: 2, campo: 'valor', mensagem: 'JSON inválido' },
    ]);
  });
});

describe('fidelidade do que já está gravado', () => {
  it('keepsStoredKeyWithSpacesOnSave', () => {
    expect(paresEmPayload(payloadEmPares({ ' a': 1 }))).toEqual({ ' a': 1 });
  });

  it.each(['01310', '0x1F', '1e3', ''])('infersTextFor %s', (texto) => {
    expect(tipoInferido(texto)).toBe('texto');
  });

  it.each(['10', '-2.5', '0'])('infersNumberFor %s', (texto) => {
    expect(tipoInferido(texto)).toBe('número');
  });
});
