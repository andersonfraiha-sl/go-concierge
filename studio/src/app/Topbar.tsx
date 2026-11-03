import { Box, Typography } from '@mui/material';
import { tokens } from '@/theme/tokens';

export function Topbar() {
  return (
    <Box
      component="header"
      sx={{
        height: tokens.layout.navbarHeight,
        flexShrink: 0,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        px: 3,
        borderBottom: 1,
        borderColor: 'divider',
        bgcolor: 'background.paper',
      }}
    >
      <Typography variant="h6" component="div" color="primary" sx={{ fontWeight: 700 }}>
        Studio
      </Typography>
      <Box data-slot="usuario" />
    </Box>
  );
}
