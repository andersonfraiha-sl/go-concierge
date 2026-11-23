import { describe, expect, it } from 'vitest';
import { criarTema } from './theme';
import { tokens } from './tokens';

describe('criarTema', () => {
  it.each([
    [1, '8px'],
    [2, '13px'],
    [3, '21px'],
    [4, '34px'],
    [5, '55px'],
    [0.5, '4px'],
  ])('usesGoldenSpacing(%s) resolves to %s', (fator, esperado) => {
    expect(criarTema().spacing(fator)).toBe(esperado);
  });

  it('usesApprovedTypeScale', () => {
    const { typography } = criarTema();
    expect(typography.h5.fontSize).toBe('21px');
    expect(typography.body1.fontSize).toBe('14px');
    expect(typography.body2.fontSize).toBe('13px');
    expect(typography.overline.fontSize).toBe('11px');
    expect(typography.overline.textTransform).toBe('uppercase');
    expect(typography.overline.letterSpacing).toBe('.06em');
  });

  it('usesApprovedFonts', () => {
    const { typography } = criarTema();
    expect(typography.fontFamily).toBe('"Argumentum", "Inter", system-ui, sans-serif');
    expect(typography.h5.fontFamily).toBe(
      '"Paralucent", "Plus Jakarta Sans", system-ui, sans-serif',
    );
    expect(typography.h5.fontWeight).toBe(700);
  });

  it('cardsHaveNoShadow', () => {
    const { components } = criarTema();
    expect(components?.MuiCard?.defaultProps?.variant).toBe('outlined');
    expect(components?.MuiPaper?.defaultProps?.elevation).toBe(0);
    const paperStyles = components?.MuiPaper?.styleOverrides?.root;
    expect(paperStyles).toMatchObject({ boxShadow: 'none' });
  });

  it('tabsAreSentenceCase', () => {
    const { components } = criarTema();
    expect(components?.MuiTab?.styleOverrides?.root).toMatchObject({ textTransform: 'none' });
    expect(components?.MuiButton?.styleOverrides?.root).toMatchObject({ textTransform: 'none' });
    expect(components?.MuiTabs?.styleOverrides?.root).toMatchObject({ gap: '26px' });
  });

  it('selectedNavItemUsesTint', () => {
    const tema = criarTema();
    const root = tema.components?.MuiListItemButton?.styleOverrides?.root;
    if (typeof root !== 'function') {
      throw new Error('MuiListItemButton.root deveria ser função de tema');
    }
    const estiloFn = root as unknown as (args: { theme: typeof tema }) => Record<string, unknown>;
    const estilos = estiloFn({ theme: tema });
    expect(estilos.borderRadius).toBe(10);
    expect(estilos.minHeight).toBe(40);
    const selecionado = estilos['&.Mui-selected'] as Record<string, unknown>;
    expect(selecionado.backgroundColor).toBe('#EEF1FE');
    expect(selecionado.color).toBe(tema.palette.primary.main);
    expect(selecionado.fontWeight).toBe(600);
  });

  it('layoutMatchesCanvas', () => {
    expect(tokens.layout.navbarHeight).toBe(58);
    expect(tokens.layout.sidebarWidth).toBe(236);
  });
});
