import AutoAwesomeOutlined from '@mui/icons-material/AutoAwesomeOutlined';
import { Box, Button, type SxProps, type Theme, Typography } from '@mui/material';
import { DevRoleSwitch } from '@/acesso/DevRoleSwitch';
import { useEu } from '@/acesso/eu';
import { useCopiloto } from '@/copiloto/CopilotoProvider';
import { PESOS, RAIOS, TIPOS, tokens } from '@/theme/tokens';

const ESTILO_DA_BARRA: SxProps<Theme> = {
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
};

export function Topbar() {
  return (
    <Box component="header" sx={ESTILO_DA_BARRA}>
      <Typography variant="h6" component="div" sx={{ color: 'secondary.main' }}>
        Studio
      </Typography>
      <Usuario />
    </Box>
  );
}

function Usuario() {
  const { data: eu } = useEu();
  return (
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
      {eu?.dev && <DevRoleSwitch usuarioAtual={eu.id} />}
      {eu && <Typography variant="body2">{eu.nome}</Typography>}
      <BotaoDoCopiloto />
    </Box>
  );
}

const estiloDoBotao = (aberto: boolean): SxProps<Theme> => ({
  height: 34,
  px: 2,
  borderRadius: `${RAIOS.chip}px`,
  fontSize: TIPOS.body2,
  fontWeight: PESOS.semi,
  border: 1,
  borderColor: aberto ? 'secondary.main' : 'border.strong',
  bgcolor: aberto ? 'secondary.main' : 'background.paper',
  color: aberto ? 'primary.contrastText' : 'secondary.main',
  '&:hover': { bgcolor: aberto ? 'secondary.main' : 'background.subtle' },
});

function BotaoDoCopiloto() {
  const { aberto, alternar, botaoDoCopiloto } = useCopiloto();
  return (
    <Button
      ref={botaoDoCopiloto}
      aria-pressed={aberto}
      onClick={alternar}
      startIcon={<AutoAwesomeOutlined />}
      sx={estiloDoBotao(aberto)}
    >
      Copiloto
    </Button>
  );
}
