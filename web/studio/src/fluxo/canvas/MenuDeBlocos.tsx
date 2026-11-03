import { Box, ListItemIcon, ListItemText, Menu, MenuItem } from '@mui/material';
import type { ReactNode } from 'react';
import { BLOCOS, TIPOS_INSERIVEIS } from '@/fluxo/modelo/catalogo';
import type { TipoBloco } from '@/fluxo/modelo/tipos';

type Props = {
  ancora: HTMLElement | null;
  onEscolher: (tipo: TipoBloco) => void;
  onFechar: () => void;
};

export function MenuDeBlocos({ ancora, onEscolher, onFechar }: Props): ReactNode {
  return (
    <Menu
      anchorEl={ancora}
      open={ancora !== null}
      onClose={onFechar}
      slotProps={{ paper: { sx: { width: 320 } } }}
    >
      {TIPOS_INSERIVEIS.map((tipo) => {
        const { rotulo, descricao, cor, Icone } = BLOCOS[tipo];
        return (
          <MenuItem
            key={tipo}
            onClick={() => onEscolher(tipo)}
            sx={{ alignItems: 'flex-start', py: 1 }}
          >
            <ListItemIcon sx={{ mt: 0.5 }}>
              <Box
                sx={{
                  width: 28,
                  height: 28,
                  display: 'grid',
                  placeItems: 'center',
                  borderRadius: 1,
                  bgcolor: `${cor}1A`,
                  color: cor,
                }}
              >
                <Icone sx={{ fontSize: 18 }} />
              </Box>
            </ListItemIcon>
            <ListItemText
              primary={rotulo}
              secondary={descricao}
              slotProps={{
                primary: { sx: { fontWeight: 700 } },
                secondary: { sx: { whiteSpace: 'normal' } },
              }}
            />
          </MenuItem>
        );
      })}
    </Menu>
  );
}
