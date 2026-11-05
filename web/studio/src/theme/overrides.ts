import type { Components, Theme } from '@mui/material/styles';
import { CAMPO, INTERRUPTOR, RAIOS, tokens } from './tokens';

export const componentOverrides: Components<Theme> = {
  MuiCssBaseline: {
    styleOverrides: {
      ':root': { colorScheme: 'light' },
      '*::-webkit-scrollbar': { width: '6px', height: '6px' },
      '*::-webkit-scrollbar-track': { background: 'transparent' },
      '*::-webkit-scrollbar-thumb': {
        background: tokens.color.scrollbar.thumb,
        borderRadius: '3px',
      },
      '*::-webkit-scrollbar-thumb:hover': { background: tokens.color.scrollbar.thumbHover },
    },
  },
  MuiButton: {
    styleOverrides: { root: { textTransform: 'none', boxShadow: 'none' } },
  },
  MuiAppBar: {
    styleOverrides: { root: { boxShadow: 'none' } },
  },
  MuiCard: {
    defaultProps: { variant: 'outlined' },
    styleOverrides: {
      root: {
        borderRadius: RAIOS.cartao,
        borderColor: tokens.color.border.default,
        boxShadow: 'none',
      },
    },
  },
  MuiPaper: {
    defaultProps: { elevation: 0 },
    styleOverrides: { root: { backgroundImage: 'none', boxShadow: 'none' } },
  },
  MuiTab: {
    styleOverrides: { root: { textTransform: 'none' } },
  },
  MuiTabs: {
    styleOverrides: {
      root: { gap: '26px' },
      indicator: { height: 2 },
    },
  },
  MuiSwitch: {
    defaultProps: { disableRipple: true },
    styleOverrides: { root: ({ theme }) => estiloDoInterruptor(theme) },
  },
  MuiTextField: {
    defaultProps: { slotProps: { inputLabel: { shrink: true } }, maxRows: 14 },
    styleOverrides: {
      // Rótulo acima da caixa, nunca flutuando dentro dela: sem o recorte na borda.
      root: ({ theme }) => ({
        '& .MuiInputLabel-shrink': {
          position: 'relative',
          transform: 'none',
          maxWidth: '100%',
          marginBottom: theme.spacing(0.75),
          fontSize: theme.typography.body2.fontSize,
          fontWeight: 600,
          color: tokens.color.text.strong,
          '&.Mui-focused': { color: theme.palette.primary.main },
          '&.Mui-error': { color: theme.palette.error.main },
        },
        '& .MuiOutlinedInput-notchedOutline': { top: 0, '& legend': { display: 'none' } },
      }),
    },
  },
  MuiOutlinedInput: {
    styleOverrides: {
      root: ({ theme }) => ({
        borderRadius: CAMPO.raio,
        backgroundColor: tokens.color.background.subtle,
        transition: theme.transitions.create(['background-color', 'border-color']),
        '& .MuiOutlinedInput-notchedOutline': { borderColor: CAMPO.bordaEmRepouso },
        '&:hover:not(.Mui-disabled):not(.Mui-focused):not(.Mui-error) .MuiOutlinedInput-notchedOutline':
          { borderColor: CAMPO.bordaNoHover },
        '&.Mui-focused': { backgroundColor: theme.palette.background.paper },
        '&.Mui-focused .MuiOutlinedInput-notchedOutline': {
          borderColor: theme.palette.primary.main,
          borderWidth: 2,
        },
        '&.Mui-error .MuiOutlinedInput-notchedOutline': { borderColor: theme.palette.error.main },
        '&.Mui-disabled': { backgroundColor: tokens.color.background.neutralFill },
        // Padding pela raiz: assim tamanho, adorno e multilinha seguem valendo sobre ele.
        '& .MuiOutlinedInput-input': { padding: '12px 14px' },
        '&.MuiInputBase-sizeSmall .MuiOutlinedInput-input': { padding: '8px 12px' },
        '&.MuiInputBase-adornedStart .MuiOutlinedInput-input': { paddingLeft: 0 },
        '&.MuiInputBase-adornedEnd .MuiOutlinedInput-input': { paddingRight: 0 },
        '&.MuiInputBase-multiline': { padding: '12px 14px', lineHeight: 1.6 },
        '&.MuiInputBase-multiline .MuiOutlinedInput-input': { padding: 0 },
      }),
      input: ({ theme }) => ({
        '&::placeholder': { color: theme.palette.text.secondary, opacity: 1 },
      }),
    },
  },
  MuiChip: {
    styleOverrides: { root: { borderRadius: RAIOS.chip } },
  },
  MuiListItemButton: {
    styleOverrides: {
      root: ({ theme }) => ({
        minHeight: 40,
        borderRadius: RAIOS.menu,
        '&:hover': {
          color: theme.palette.primary.main,
          fontWeight: 600,
          '& .MuiListItemIcon-root': { color: theme.palette.primary.main },
        },
        '&.Mui-selected': {
          backgroundColor: tokens.color.background.tint,
          color: theme.palette.primary.main,
          fontWeight: 600,
          '&:hover': { backgroundColor: tokens.color.background.tint },
          '& .MuiListItemIcon-root': { color: theme.palette.primary.main },
        },
      }),
    },
  },
};

const CLASSES_DO_INTERRUPTOR = {
  base: '& .MuiSwitch-switchBase',
  pino: '& .MuiSwitch-thumb',
  trilho: '& .MuiSwitch-track',
} as const;

function estiloDaBaseDoInterruptor(theme: Theme, transicao: string) {
  return {
    padding: 2,
    transition: `transform ${transicao}`,
    '&:hover': { backgroundColor: 'transparent' },
    '&.Mui-checked': { transform: 'translateX(20px)' },
    '&.Mui-checked + .MuiSwitch-track': {
      backgroundColor: theme.palette.primary.main,
      opacity: 1,
    },
    '&.Mui-disabled + .MuiSwitch-track': { opacity: 0.45 },
  };
}

const estiloDoPinoDoInterruptor = (theme: Theme) => ({
  width: 20,
  height: 20,
  border: 0,
  backgroundColor: theme.palette.background.paper,
  boxShadow: INTERRUPTOR.sombraDoPino,
});

const estiloDoTrilhoDoInterruptor = (transicao: string) => ({
  borderRadius: 12,
  backgroundColor: INTERRUPTOR.trilhoDesligado,
  opacity: 1,
  transition: `background-color ${transicao}, opacity ${transicao}`,
});

const ESTILO_DO_INTERRUPTOR_PEQUENO = {
  width: 36,
  height: 20,
  [CLASSES_DO_INTERRUPTOR.base]: {
    padding: 2,
    '&.Mui-checked': { transform: 'translateX(16px)' },
  },
  [CLASSES_DO_INTERRUPTOR.pino]: { width: 16, height: 16 },
  [CLASSES_DO_INTERRUPTOR.trilho]: { borderRadius: 10 },
  borderRadius: 10,
};

// O MUI desenha o Switch com 12 px de respiro em volta do trilho e o pino com 9 px de
// padding. Zerar só o respiro da raiz (o override antigo) deixava o pino deslocado e
// cortado no canto: aqui pino, trilho e deslocamento são medidos juntos.
function estiloDoInterruptor(theme: Theme) {
  const transicao = `${INTERRUPTOR.duracaoMs}ms ${theme.transitions.easing.easeInOut}`;
  return {
    width: 44,
    height: 24,
    padding: 0,
    borderRadius: 12,
    // O anel fica na raiz: o overflow dela corta a área clicável do input, não o outline.
    '&:has(.Mui-focusVisible)': {
      outline: `2px solid ${theme.palette.primary.main}`,
      outlineOffset: 2,
    },
    [CLASSES_DO_INTERRUPTOR.base]: estiloDaBaseDoInterruptor(theme, transicao),
    [CLASSES_DO_INTERRUPTOR.pino]: estiloDoPinoDoInterruptor(theme),
    [CLASSES_DO_INTERRUPTOR.trilho]: estiloDoTrilhoDoInterruptor(transicao),
    '&.MuiSwitch-sizeSmall': ESTILO_DO_INTERRUPTOR_PEQUENO,
  };
}
