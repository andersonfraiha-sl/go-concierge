import type { Components, Theme } from '@mui/material/styles';
import { RAIOS, tokens } from './tokens';

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
    styleOverrides: { root: { borderRadius: RAIOS.cartao, boxShadow: 'none' } },
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
    styleOverrides: {
      root: { width: 40, height: 22, padding: 0 },
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
