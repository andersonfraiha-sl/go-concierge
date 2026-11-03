import type { Theme } from '@mui/material/styles';
import { createTheme } from '@mui/material/styles';
import { componentOverrides } from './overrides';
import { tokens } from './tokens';

export function criarTema(): Theme {
  return createTheme({
    palette: {
      primary: { main: tokens.color.primary.main, contrastText: tokens.color.primary.contrastText },
      secondary: { main: tokens.color.navy },
      text: tokens.color.text,
      background: tokens.color.background,
      divider: tokens.color.divider,
    },
    typography: tokens.typography,
    shape: tokens.shape,
    components: componentOverrides,
  });
}
