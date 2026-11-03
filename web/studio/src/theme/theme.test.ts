import { describe, expect, it } from 'vitest';
import { criarTema } from './theme';

describe('criarTema', () => {
  it('usaAzulDaMarcaComoCorPrimaria', () => {
    expect(criarTema().palette.primary.main).toBe('#1034f2');
  });

  it('usaAsFontesDoConsole', () => {
    const { typography } = criarTema();
    expect(typography.fontFamily).toBe('"Argumentum", "Inter", system-ui, sans-serif');
    expect(typography.h1.fontFamily).toBe(
      '"Paralucent", "Plus Jakarta Sans", system-ui, sans-serif',
    );
  });

  it('usaOsTokensDoConsole', () => {
    const { palette, shape } = criarTema();
    expect(palette.secondary.main).toBe('#071950');
    expect(palette.divider).toBe('#DBDFE5');
    expect(shape.borderRadius).toBe(8);
  });

  it('desenhaCartoesSemSombra', () => {
    const { components } = criarTema();
    expect(components?.MuiCard?.defaultProps?.variant).toBe('outlined');
    expect(components?.MuiPaper?.defaultProps?.elevation).toBe(0);
  });
});
