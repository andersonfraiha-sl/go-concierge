import { describe, expect, it } from 'vitest';
import { criarTema } from './theme';

describe('criarTema', () => {
  it('usaAzulDaMarcaComoCorPrimaria', () => {
    expect(criarTema().palette.primary.main).toBe('#1034f2');
  });

  it('usaLatoComoFontePadrao', () => {
    expect(criarTema().typography.fontFamily).toContain('Lato');
  });

  it('desenhaCartoesSemSombra', () => {
    const { components } = criarTema();
    expect(components?.MuiCard?.defaultProps?.variant).toBe('outlined');
    expect(components?.MuiPaper?.defaultProps?.elevation).toBe(0);
  });
});
