import { describe, expect, it } from 'vitest';
import { separarUnidade } from './ajustes';

describe('separarUnidade', () => {
  it.each([
    {
      rotulo: 'Antecedência mínima da reserva (dias)',
      esperado: { rotulo: 'Antecedência mínima da reserva', unidade: 'dias' },
    },
    { rotulo: 'Horário da portaria', esperado: { rotulo: 'Horário da portaria', unidade: null } },
    {
      rotulo: 'Antecedência mínima da reserva (até quantos dias antes)',
      esperado: {
        rotulo: 'Antecedência mínima da reserva (até quantos dias antes)',
        unidade: null,
      },
    },
  ])('separatesUnitFromLabel $rotulo', ({ rotulo, esperado }) => {
    expect(separarUnidade(rotulo)).toEqual(esperado);
  });
});
