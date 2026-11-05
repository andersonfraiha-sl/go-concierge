import { Box, IconButton, ListItemButton, type SxProps, type Theme, Tooltip } from '@mui/material';
import type { ReactNode } from 'react';
import { NavLink, useLocation } from 'react-router';
import { ErroAcesso } from '@/acesso/ErroAcesso';
import { type Eu, pode, podeVerCondominios, useEu } from '@/acesso/eu';
import { ICONES, RAIO, SHELL, TIPOS, tokens } from '@/theme/tokens';
import { IconeDeLinha, type NomeDoIcone } from './iconesDoShell';

type ItemMenu = { rotulo: string; destino: string; icone: NomeDoIcone };
type ItemSpa = ItemMenu & { visivelPara: (eu: Eu) => boolean };

const ITENS_SPA = [
  {
    rotulo: 'Concierge',
    destino: '/concierge',
    icone: 'concierge',
    visivelPara: (eu) => pode(eu, 'skills.ler'),
  },
  {
    rotulo: 'Skills',
    destino: '/skills',
    icone: 'skills',
    visivelPara: (eu) => pode(eu, 'skills.ler'),
  },
  {
    rotulo: 'Ferramentas',
    destino: '/ferramentas',
    icone: 'ferramentas',
    visivelPara: (eu) => pode(eu, 'skills.ler'),
  },
  {
    rotulo: 'Condomínios',
    destino: '/condominios',
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

// Fora do SPA: navegação completa, sem o basename das rotas.
const ITEM_ATENDENTE = {
  rotulo: 'Atendente',
  destino: '/console',
  icone: 'atendente',
} satisfies ItemMenu;

type Props = { recolhido: boolean; aoAlternar: () => void };

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

export function Sidebar({ recolhido, aoAlternar }: Props): ReactNode {
  return (
    <Box component="nav" aria-label="Menu principal" sx={estiloDaSidebar(recolhido)}>
      <ItensDoSpa recolhido={recolhido} />
      <Box sx={{ my: 2, mx: 1, borderTop: 1, borderColor: 'border.subtle' }} />
      <ItemDoMenu
        item={ITEM_ATENDENTE}
        recolhido={recolhido}
        selecionado={false}
        aoNavegar={{ component: 'a', href: ITEM_ATENDENTE.destino }}
      />
      <BotaoDeRecolher recolhido={recolhido} aoAlternar={aoAlternar} />
    </Box>
  );
}

function ItensDoSpa({ recolhido }: { recolhido: boolean }): ReactNode {
  const { pathname } = useLocation();
  const { data: eu, isError, refetch } = useEu();
  const itensVisiveis = eu ? ITENS_SPA.filter((item) => item.visivelPara(eu)) : [];

  return (
    <>
      {itensVisiveis.map((item) => (
        <ItemDoMenu
          key={item.destino}
          item={item}
          recolhido={recolhido}
          selecionado={pathname.startsWith(item.destino)}
          aoNavegar={{ component: NavLink, to: item.destino }}
        />
      ))}
      {isError && !recolhido && <ErroAcesso aoTentar={() => void refetch()} />}
    </>
  );
}

/** Fora do desenho aprovado, mas é uma função que já existia: fica discreta, no pé do menu. */
function BotaoDeRecolher({ recolhido, aoAlternar }: Props): ReactNode {
  const rotulo = recolhido ? 'Expandir menu' : 'Recolher menu';
  return (
    <Box sx={{ mt: 'auto', display: 'flex', justifyContent: recolhido ? 'center' : 'flex-end' }}>
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
  aoNavegar: { component: typeof NavLink; to: string } | { component: 'a'; href: string };
};

const estiloDoItem = (recolhido: boolean, externo: boolean): SxProps<Theme> => ({
  flex: 'none',
  height: SHELL.alturaDoItemDoMenu,
  minHeight: 0,
  px: recolhido ? 0 : 2,
  gap: 2,
  justifyContent: recolhido ? 'center' : 'flex-start',
  borderRadius: RAIO.menu,
  fontSize: TIPOS.body1,
  color: externo ? 'text.secondary' : 'text.strong',
});

function ItemDoMenu({ item, recolhido, selecionado, aoNavegar }: ItemProps): ReactNode {
  const { rotulo, icone, destino } = item;
  const externo = destino === ITEM_ATENDENTE.destino;
  return (
    <Tooltip title={recolhido ? rotulo : ''} placement="right">
      <ListItemButton
        {...aoNavegar}
        selected={selecionado}
        aria-label={rotulo}
        sx={estiloDoItem(recolhido, externo)}
      >
        <IconeDeLinha nome={icone} />
        {!recolhido && rotulo}
        {!recolhido && externo && (
          <IconeDeLinha
            nome="linkExterno"
            tamanho={ICONES.minimo}
            sx={{ ml: 'auto', color: 'text.faint' }}
          />
        )}
      </ListItemButton>
    </Tooltip>
  );
}
