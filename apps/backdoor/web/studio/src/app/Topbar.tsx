import { Box, type SxProps, type Theme, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import { PESOS, RAIO, SHELL, TIPOS, tokens } from '@/theme/tokens';
import { BotaoDoCopiloto } from './BotaoDoCopiloto';
import { BuscaGlobal } from './BuscaGlobal';
import { IconeDeLinha } from './iconesDoShell';
import { MenuDoUsuario } from './MenuDoUsuario';

const ESTILO_DA_BARRA: SxProps<Theme> = {
  height: tokens.layout.navbarHeight,
  // Como no desenho (HTML sem reset), altura e largura não contam padding nem borda.
  boxSizing: 'content-box',
  flexShrink: 0,
  display: 'flex',
  alignItems: 'center',
  gap: { xs: 1, md: 3 },
  px: { xs: 1, md: 4 },
  borderBottom: 1,
  borderColor: 'border.default',
  bgcolor: 'background.paper',
};

export function Topbar(): ReactNode {
  return (
    <Box component="header" sx={ESTILO_DA_BARRA}>
      <Marca />
      <Box sx={{ flex: 1 }} />
      <BuscaGlobal />
      <BotaoDoCopiloto variante="barra" />
      <MenuDoUsuario />
    </Box>
  );
}

const ESTILO_DO_LOGO: SxProps<Theme> = {
  width: SHELL.logo,
  height: SHELL.logo,
  borderRadius: RAIO.menu,
  bgcolor: 'primary.main',
  color: 'primary.contrastText',
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
};

function Marca(): ReactNode {
  return (
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
      <Box sx={ESTILO_DO_LOGO}>
        <IconeDeLinha nome="concierge" />
      </Box>
      <Typography
        component="span"
        sx={{
          display: { xs: 'none', sm: 'inline' },
          fontFamily: (tema) => tema.typography.h6.fontFamily,
          fontWeight: PESOS.negrito,
          fontSize: TIPOS.marca,
          lineHeight: 'normal',
          color: 'secondary.main',
        }}
      >
        Superlógica
      </Typography>
      <Box
        sx={{
          display: { xs: 'none', sm: 'block' },
          height: SHELL.divisorDaMarca,
          borderLeft: 1,
          borderColor: 'border.strong',
        }}
      />
      <Typography
        component="span"
        sx={{
          display: { xs: 'none', sm: 'inline' },
          fontSize: TIPOS.subtitle1,
          lineHeight: 'normal',
          color: 'text.secondary',
        }}
      >
        Concierge{' '}
        <Box component="strong" sx={{ color: 'primary.main', fontWeight: PESOS.semi }}>
          Studio
        </Box>
      </Typography>
    </Box>
  );
}
