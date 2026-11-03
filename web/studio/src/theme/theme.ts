import type { Theme } from '@mui/material/styles';
import { createTheme } from '@mui/material/styles';
import { componentOverrides } from './overrides';
import { ESPACOS, tokens } from './tokens';

const MAX_FATOR_NA_TABELA = ESPACOS.length - 1;

// Espaçamento em proporção áurea: inteiro 0-5 usa a tabela do desenho aprovado;
// qualquer outro fator cai no múltiplo de 8 px padrão do MUI.
export function espaco(fator: number): number {
  const estaNaTabela = Number.isInteger(fator) && fator >= 0 && fator <= MAX_FATOR_NA_TABELA;
  const naTabela = estaNaTabela ? ESPACOS[fator] : undefined;
  return naTabela ?? Math.round(fator * 8);
}

export function criarTema(): Theme {
  return createTheme({
    palette: {
      primary: {
        main: tokens.color.primary.main,
        dark: tokens.color.primary.dark,
        contrastText: tokens.color.primary.contrastText,
      },
      secondary: { main: tokens.color.navy },
      text: { primary: tokens.color.text.primary, secondary: tokens.color.text.secondary },
      background: {
        default: tokens.color.background.default,
        paper: tokens.color.background.paper,
      },
      divider: tokens.color.border.strong,
      success: { main: tokens.color.success.main },
      error: { main: tokens.color.danger.main },
    },
    typography: tokens.typography,
    shape: tokens.shape,
    spacing: espaco,
    components: componentOverrides,
  });
}
