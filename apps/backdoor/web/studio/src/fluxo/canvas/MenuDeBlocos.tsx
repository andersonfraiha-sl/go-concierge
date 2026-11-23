import { Box, ListItemIcon, ListItemText, Menu, MenuItem } from '@mui/material';
import type { ReactNode } from 'react';
import { BLOCOS, TIPOS_INSERIVEIS } from '@/fluxo/modelo/catalogo';
import type { TipoBloco } from '@/fluxo/modelo/tipos';
import { useCorDoBloco } from '@/fluxo/modelo/useCorDoBloco';
import { ICONES, PESOS, RAIO } from '@/theme/tokens';

type Props = {
  ancora: HTMLElement | null;
  onEscolher: (tipo: TipoBloco) => void;
  onFechar: () => void;
};

type ItemProps = { tipo: TipoBloco; onEscolher: (tipo: TipoBloco) => void };

function ItemDeBloco({ tipo, onEscolher }: ItemProps): ReactNode {
  const { rotulo, descricao, Icone } = BLOCOS[tipo];
  const cor = useCorDoBloco(tipo);
  return (
    <MenuItem onClick={() => onEscolher(tipo)} sx={{ alignItems: 'flex-start', py: 1 }}>
      <ListItemIcon sx={{ mt: 0.5 }}>
        <Box
          sx={{
            width: 28,
            height: 28,
            display: 'grid',
            placeItems: 'center',
            borderRadius: RAIO.campo,
            bgcolor: `${cor}1A`,
            color: cor,
          }}
        >
          <Icone sx={{ fontSize: ICONES.grande }} />
        </Box>
      </ListItemIcon>
      <ListItemText
        primary={rotulo}
        secondary={descricao}
        slotProps={{
          primary: { sx: { fontWeight: PESOS.negrito } },
          secondary: { sx: { whiteSpace: 'normal' } },
        }}
      />
    </MenuItem>
  );
}

export function MenuDeBlocos({ ancora, onEscolher, onFechar }: Props): ReactNode {
  return (
    <Menu
      anchorEl={ancora}
      open={ancora !== null}
      onClose={onFechar}
      slotProps={{ paper: { sx: { width: 320 } } }}
    >
      {TIPOS_INSERIVEIS.map((tipo) => (
        <ItemDeBloco key={tipo} tipo={tipo} onEscolher={onEscolher} />
      ))}
    </Menu>
  );
}
