import { Box, Typography } from '@mui/material';
import { DevRoleSwitch } from './DevRoleSwitch';

/** Tela inteira do 401: nada da página anterior nem do Copiloto; em dev, o "Ver como" para voltar. */
export function UsuarioNaoReconhecido({ dev }: { dev: boolean }) {
  return (
    <Box
      component="main"
      sx={{
        minHeight: '100vh',
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        gap: 3,
        bgcolor: 'background.default',
      }}
    >
      <Typography role="alert" variant="h6">
        Não reconhecemos seu usuário.
      </Typography>
      {dev && <DevRoleSwitch usuarioAtual="" />}
    </Box>
  );
}
