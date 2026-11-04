import AutoAwesomeOutlined from '@mui/icons-material/AutoAwesomeOutlined';
import { Box, Button, Typography } from '@mui/material';
import { DevRoleSwitch } from '@/acesso/DevRoleSwitch';
import { useEu } from '@/acesso/eu';
import { useCopiloto } from '@/copiloto/CopilotoProvider';
import { RAIOS, tokens } from '@/theme/tokens';

export function Topbar() {
  const { data: eu } = useEu();

  return (
    <Box
      component="header"
      sx={{
        height: tokens.layout.navbarHeight,
        flexShrink: 0,
        // Fixo no topo: o painel do Copiloto gruda logo abaixo, sem vão ao rolar.
        position: 'sticky',
        top: 0,
        zIndex: (tema) => tema.zIndex.appBar,
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
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
        {eu?.dev && <DevRoleSwitch usuarioAtual={eu.id} />}
        {eu && <Typography variant="body2">{eu.nome}</Typography>}
        <BotaoDoCopiloto />
      </Box>
    </Box>
  );
}

function BotaoDoCopiloto() {
  const { aberto, alternar, botaoDoCopiloto } = useCopiloto();
  return (
    <Button
      ref={botaoDoCopiloto}
      aria-pressed={aberto}
      onClick={alternar}
      startIcon={<AutoAwesomeOutlined />}
      sx={{
        height: 34,
        px: 2,
        borderRadius: `${RAIOS.chip}px`,
        fontSize: 13,
        fontWeight: 600,
        border: 1,
        borderColor: aberto ? tokens.color.navy : tokens.color.border.strong,
        bgcolor: aberto ? tokens.color.navy : 'background.paper',
        color: aberto ? 'primary.contrastText' : tokens.color.navy,
        '&:hover': { bgcolor: aberto ? tokens.color.navy : tokens.color.background.subtle },
      }}
    >
      Copiloto
    </Button>
  );
}
