import { Box, IconButton, ListItemButton, type SxProps, type Theme, Tooltip } from '@mui/material';
import type { ReactNode } from 'react';
import { NavLink, useLocation } from 'react-router';
import { ErroAcesso } from '@/acesso/ErroAcesso';
import { type Eu, pode, podeVerCondominios, useEu } from '@/acesso/eu';
import { ICONES, RAIO, SHELL, TIPOS, tokens } from '@/theme/tokens';
import { IconeDeLinha, type NomeDoIcone } from './iconesDoShell';

type ItemMenu = { rotulo: string; destino: string; icone: NomeDoIcone };
type ItemSpa = ItemMenu & { visivelPara: (eu: Eu) => boolean };

const DESTINO_DOS_CONDOMINIOS = '/condominios';

const ITENS_SPA = [
  {
    rotulo: 'Concierge',
    destino: '/concierge',
    icone: 'concierge',
    visivelPara: (eu) => pode(eu, 'skills.ler'),
  },
  {
    rotulo: 'Casos de uso',
    destino: '/skills',
    icone: 'skills',
    visivelPara: (eu) => pode(eu, 'skills.ler'),
  },
  {
    rotulo: 'Recursos',
    destino: '/ferramentas',
    icone: 'ferramentas',
    visivelPara: (eu) => pode(eu, 'skills.ler'),
  },
  {
    rotulo: 'Conteúdo',
    destino: DESTINO_DOS_CONDOMINIOS,
    icone: 'condominios',
    visivelPara: podeVerCondominios,
  },
  {
    rotulo: 'Avançado',
    destino: '/avancado',
    icone: 'avancado',
    visivelPara: (eu) => pode(eu, 'studio.avancado'),
  },
] as const satisfies readonly ItemSpa[];

type Props = {
  recolhido: boolean;
  aoAlternar: () => void;
};

type RecolherProps = Pick<Props, 'recolhido' | 'aoAlternar'>;

const estiloDaSidebar = (recolhido: boolean): SxProps<Theme> => ({
  width: recolhido ? tokens.layout.sidebarCollapsedWidth : tokens.layout.sidebarWidth,
  flexShrink: 0,
  display: 'flex',
  flexDirection: 'column',
  gap: 0.375,
  py: 3,
  px: recolhido ? 1 : 2,
  overflowY: 'auto',
  borderRight: 1,
  borderColor: 'border.default',
  bgcolor: 'background.paper',
  transition: (tema) =>
    tema.transitions.create('width', { duration: tema.transitions.duration.shorter }),
});

export function Sidebar(props: Props): ReactNode {
  const { recolhido, aoAlternar } = props;
  return (
    <Box component="nav" aria-label="Menu principal" sx={estiloDaSidebar(recolhido)}>
      <ItensDoSpa recolhido={recolhido} />
      <BotaoDeRecolher recolhido={recolhido} aoAlternar={aoAlternar} />
    </Box>
  );
}

type ItensProps = { recolhido: boolean };

function itemParaOUsuario(eu: Eu, item: ItemSpa): ItemSpa {
  if (item.destino !== DESTINO_DOS_CONDOMINIOS) return item;
  const company = eu.administradoraId;
  if (!eu.condominioId) {
    return company
      ? { ...item, destino: `${DESTINO_DOS_CONDOMINIOS}?company_id=${encodeURIComponent(company)}` }
      : item;
  }
  return {
    ...item,
    rotulo: 'Conteúdo',
    destino: `${DESTINO_DOS_CONDOMINIOS}/${encodeURIComponent(eu.condominioId)}?company_id=${encodeURIComponent(eu.administradoraId ?? '')}`,
  };
}

function ItensDoSpa({ recolhido }: ItensProps): ReactNode {
  const { pathname } = useLocation();
  const { data: eu, isError, refetch } = useEu();
  const itens: readonly ItemSpa[] = ITENS_SPA;
  const itensVisiveis = eu
    ? itens.filter((item) => item.visivelPara(eu)).map((item) => itemParaOUsuario(eu, item))
    : [];

  return (
    <>
      {itensVisiveis.map((item) => (
        <ItemDoMenu
          key={item.destino}
          item={item}
          recolhido={recolhido}
          selecionado={pathname.startsWith(item.destino.split('?')[0] ?? item.destino)}
          aoNavegar={{ component: NavLink, to: item.destino }}
        />
      ))}
      {isError && !recolhido && <ErroAcesso aoTentar={() => void refetch()} />}
    </>
  );
}

/** Fora do desenho aprovado, mas é uma função que já existia: fica discreta, no pé do menu. */
function BotaoDeRecolher({ recolhido, aoAlternar }: RecolherProps): ReactNode {
  const rotulo = recolhido ? 'Expandir menu' : 'Recolher menu';
  return (
    <Box
      sx={{
        mt: recolhido ? 'auto' : 0,
        display: { xs: 'none', md: 'flex' },
        justifyContent: recolhido ? 'center' : 'flex-end',
      }}
    >
      <IconButton
        size="small"
        onClick={aoAlternar}
        aria-label={rotulo}
        sx={{ color: 'text.faint' }}
      >
        <IconeDeLinha nome={recolhido ? 'expandir' : 'recolher'} tamanho={ICONES.medio} />
      </IconButton>
    </Box>
  );
}

type ItemProps = {
  item: ItemMenu;
  recolhido: boolean;
  selecionado: boolean;
  aoNavegar: { component: typeof NavLink; to: string };
};

const estiloDoItem = (recolhido: boolean): SxProps<Theme> => ({
  flex: 'none',
  height: SHELL.alturaDoItemDoMenu,
  minHeight: 0,
  px: recolhido ? 0 : 2,
  gap: 2,
  justifyContent: recolhido ? 'center' : 'flex-start',
  borderRadius: RAIO.menu,
  fontSize: TIPOS.body1,
  color: 'text.strong',
});

function ItemDoMenu({ item, recolhido, selecionado, aoNavegar }: ItemProps): ReactNode {
  const { rotulo, icone } = item;
  return (
    <Tooltip title={recolhido ? rotulo : ''} placement="right">
      <ListItemButton
        {...aoNavegar}
        selected={selecionado}
        aria-label={rotulo}
        sx={estiloDoItem(recolhido)}
      >
        <IconeDeLinha nome={icone} />
        {!recolhido && rotulo}
      </ListItemButton>
    </Tooltip>
  );
}
