import { describe, expect, it } from 'vitest';
import { formatarData } from './formatarData';

describe('formatarData', () => {
  it.each([
    { entrada: '2026-12-01', esperado: '01/12/2026' },
    { entrada: '2026-11-18T12:00:00Z', esperado: '18/11/2026' },
    { entrada: 'sem data', esperado: 'sem data' },
  ])('formats $entrada as $esperado', ({ entrada, esperado }) => {
    expect(formatarData(entrada)).toBe(esperado);
  });
});
