import type { Components, Theme } from '@mui/material/styles';
import { alpha } from '@mui/material/styles';

export const componentOverrides: Components<Theme> = {
  MuiCssBaseline: {
    styleOverrides: {
      ':root': { colorScheme: 'light' },
      '*::-webkit-scrollbar': { width: '6px', height: '6px' },
      '*::-webkit-scrollbar-track': { background: 'transparent' },
      '*::-webkit-scrollbar-thumb': { background: '#c1c1c1', borderRadius: '3px' },
      '*::-webkit-scrollbar-thumb:hover': { background: '#a8a8a8' },
    },
  },
  MuiButton: {
    styleOverrides: { root: { textTransform: 'none', boxShadow: 'none' } },
  },
  MuiAppBar: {
    styleOverrides: { root: { boxShadow: 'none' } },
  },
  MuiCard: { defaultProps: { variant: 'outlined' } },
  MuiPaper: { defaultProps: { elevation: 0 } },
  MuiListItemButton: {
    styleOverrides: {
      root: ({ theme }) => ({
        borderRadius: 1,
        '&:hover': {
          color: theme.palette.primary.main,
          fontWeight: 600,
          '& .MuiListItemIcon-root': { color: theme.palette.primary.main },
        },
        '&.Mui-selected': {
          backgroundColor: alpha(theme.palette.primary.main, 0.08),
          color: theme.palette.primary.main,
          fontWeight: 600,
          '&:hover': { backgroundColor: alpha(theme.palette.primary.main, 0.12) },
          '& .MuiListItemIcon-root': { color: theme.palette.primary.main },
        },
      }),
    },
  },
};
