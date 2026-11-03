import { Box, Typography } from '@mui/material';
import { DevRoleSwitch } from '@/acesso/DevRoleSwitch';
import { useEu } from '@/acesso/eu';
import { tokens } from '@/theme/tokens';

export function Topbar() {
  const { data: eu } = useEu();

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
      <Typography variant="h6" component="div" sx={{ color: 'secondary.main' }}>
        Studio
      </Typography>
      {eu && (
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
          {eu.dev && <DevRoleSwitch papelAtual={eu.papel} />}
          <Typography variant="body2">{eu.nome}</Typography>
        </Box>
      )}
    </Box>
  );
}
