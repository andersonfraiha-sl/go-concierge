import ApartmentOutlined from '@mui/icons-material/ApartmentOutlined';
import BuildOutlined from '@mui/icons-material/BuildOutlined';
import ChatBubbleOutlined from '@mui/icons-material/ChatBubbleOutlined';
import ChevronLeft from '@mui/icons-material/ChevronLeft';
import ChevronRight from '@mui/icons-material/ChevronRight';
import ExtensionOutlined from '@mui/icons-material/ExtensionOutlined';
import HeadsetMicOutlined from '@mui/icons-material/HeadsetMicOutlined';
import TuneOutlined from '@mui/icons-material/TuneOutlined';
import {
  Box,
  Divider,
  IconButton,
  List,
  ListItemButton,
  ListItemIcon,
  ListItemText,
  Tooltip,
} from '@mui/material';
import type { SvgIconComponent } from '@mui/icons-material';
import { NavLink, useLocation } from 'react-router';
import { tokens } from '@/theme/tokens';

type ItemMenu = { rotulo: string; destino: string; Icone: SvgIconComponent };

const ITENS_SPA = [
  { rotulo: 'Skills', destino: '/skills', Icone: ExtensionOutlined },
  { rotulo: 'Assistente', destino: '/assistente', Icone: ChatBubbleOutlined },
  { rotulo: 'Ferramentas', destino: '/ferramentas', Icone: BuildOutlined },
  { rotulo: 'Condomínios', destino: '/condominios', Icone: ApartmentOutlined },
  { rotulo: 'Avançado', destino: '/avancado', Icone: TuneOutlined },
] as const satisfies readonly ItemMenu[];

// Fora do SPA: navegação completa, sem o basename do roteador.
const ITEM_ATENDENTE = { rotulo: 'Atendente', destino: '/console', Icone: HeadsetMicOutlined } satisfies ItemMenu;

type Props = { recolhido: boolean; aoAlternar: () => void };

export function Sidebar({ recolhido, aoAlternar }: Props) {
  const { pathname } = useLocation();
  const largura = recolhido ? tokens.layout.sidebarCollapsedWidth : tokens.layout.sidebarWidth;

  return (
    <Box
      component="nav"
      aria-label="Menu principal"
      sx={{
        width: largura,
        flexShrink: 0,
        display: 'flex',
        flexDirection: 'column',
        borderRight: 1,
        borderColor: 'divider',
        bgcolor: 'background.paper',
        transition: (tema) => tema.transitions.create('width', { duration: tema.transitions.duration.shorter }),
      }}
    >
      <List sx={{ p: 1, flex: 1 }}>
        {ITENS_SPA.map((item) => (
          <ItemDoMenu
            key={item.destino}
            item={item}
            recolhido={recolhido}
            selecionado={pathname.startsWith(item.destino)}
            aoNavegar={{ component: NavLink, to: item.destino }}
          />
        ))}
        <Divider sx={{ my: 1 }} />
        <ItemDoMenu
          item={ITEM_ATENDENTE}
          recolhido={recolhido}
          selecionado={false}
          aoNavegar={{ component: 'a', href: ITEM_ATENDENTE.destino }}
        />
      </List>
      <Box sx={{ p: 1, display: 'flex', justifyContent: recolhido ? 'center' : 'flex-end' }}>
        <IconButton
          size="small"
          onClick={aoAlternar}
          aria-label={recolhido ? 'Expandir menu' : 'Recolher menu'}
        >
          {recolhido ? <ChevronRight /> : <ChevronLeft />}
        </IconButton>
      </Box>
    </Box>
  );
}

type ItemProps = {
  item: ItemMenu;
  recolhido: boolean;
  selecionado: boolean;
  aoNavegar: { component: typeof NavLink; to: string } | { component: 'a'; href: string };
};

function ItemDoMenu({ item, recolhido, selecionado, aoNavegar }: ItemProps) {
  const { rotulo, Icone } = item;
  return (
    <Tooltip title={recolhido ? rotulo : ''} placement="right">
      <ListItemButton
        {...aoNavegar}
        selected={selecionado}
        aria-label={rotulo}
        sx={{ mb: 0.5, minHeight: 40, justifyContent: recolhido ? 'center' : 'flex-start' }}
      >
        <ListItemIcon sx={{ minWidth: recolhido ? 0 : 40, color: 'inherit' }}>
          <Icone fontSize="small" />
        </ListItemIcon>
        {!recolhido && <ListItemText primary={rotulo} slotProps={{ primary: { sx: { fontWeight: 'inherit' } } }} />}
      </ListItemButton>
    </Tooltip>
  );
}
