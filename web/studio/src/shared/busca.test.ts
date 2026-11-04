import { describe, expect, it } from 'vitest';
import { contemBusca, normalizarBusca } from './busca';

describe('normalizarBusca', () => {
  it('ignoresAccentsAndCase', () => {
    expect(normalizarBusca('  Síndico Ótimo  ')).toBe('sindico otimo');
  });
});

describe('contemBusca', () => {
  it('acceptsEmptySearch', () => {
    expect(contemBusca(['Reserva do Salão'], '')).toBe(true);
    expect(contemBusca([], '')).toBe(true);
  });

  it('findsMatchIgnoringAccentsAndCase', () => {
    expect(contemBusca(['Reserva do Salão'], 'SALAO')).toBe(true);
  });

  it('rejectsWhenNoTargetMatches', () => {
    expect(contemBusca(['Reserva do Salão'], 'mudanca')).toBe(false);
  });
});
