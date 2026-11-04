import type { PaletteOptions, Theme } from '@mui/material/styles';
import { createTheme } from '@mui/material/styles';
import { componentOverrides } from './overrides';
import { ESPACOS, tokens } from './tokens';

// Tons do desenho aprovado que os componentes pedem pelo caminho da paleta no `sx`
// ('border.subtle', 'text.muted', 'background.tint'...). Sem eles na paleta, o MUI
// repassava o texto cru ao CSS e a borda caía na cor do texto.
declare module '@mui/material/styles' {
  interface TypeText {
    strong: string;
    tertiary: string;
    muted: string;
  }
  interface TypeBackground {
    subtle: string;
    tint: string;
    neutralFill: string;
  }
  interface Palette {
    border: { default: string; strong: string; subtle: string };
  }
  interface PaletteOptions {
    border?: { default: string; strong: string; subtle: string };
  }
}

const MAX_FATOR_NA_TABELA = ESPACOS.length - 1;

// Espaçamento em proporção áurea: inteiro 0-5 usa a tabela do desenho aprovado;
// qualquer outro fator cai no múltiplo de 8 px padrão do MUI.
export function espaco(fator: number): number {
  const estaNaTabela = Number.isInteger(fator) && fator >= 0 && fator <= MAX_FATOR_NA_TABELA;
  const naTabela = estaNaTabela ? ESPACOS[fator] : undefined;
  return naTabela ?? Math.round(fator * 8);
}

const PALETA: PaletteOptions = {
  primary: {
    main: tokens.color.primary.main,
    dark: tokens.color.primary.dark,
    contrastText: tokens.color.primary.contrastText,
  },
  secondary: { main: tokens.color.navy },
  text: {
    primary: tokens.color.text.primary,
    secondary: tokens.color.text.secondary,
    strong: tokens.color.text.strong,
    tertiary: tokens.color.text.tertiary,
    muted: tokens.color.text.muted,
  },
  background: {
    default: tokens.color.background.default,
    paper: tokens.color.background.paper,
    subtle: tokens.color.background.subtle,
    tint: tokens.color.background.tint,
    neutralFill: tokens.color.background.neutralFill,
  },
  border: tokens.color.border,
  divider: tokens.color.border.strong,
  success: { main: tokens.color.success.main },
  error: { main: tokens.color.danger.main },
};

export function criarTema(): Theme {
  return createTheme({
    palette: PALETA,
    typography: tokens.typography,
    shape: tokens.shape,
    spacing: espaco,
    components: componentOverrides,
  });
}
