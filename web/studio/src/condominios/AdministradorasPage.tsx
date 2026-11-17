import { Box, Link as LinkExterno, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import { useContextoDaTela } from '@/copiloto/useContextoDaTela';
import { CONDOMINIOS, PESOS } from '@/theme/tokens';
import { AdministradorasAba } from './AdministradorasAba';
import { ComEstado } from './ComEstado';
import { useEstado } from './contratos';

export function AdministradorasPage(): ReactNode {
  const consulta = useEstado();
  // O Copiloto não tem tela própria de administradoras: o contexto é o mesmo de condomínios.
  useContextoDaTela({ tela: 'condominios' });

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
      <Box>
        <Typography
          variant="h4"
          component="h1"
          color="secondary"
          sx={{ letterSpacing: CONDOMINIOS.letterSpacingDoTitulo }}
        >
          Administradoras
        </Typography>
        <Typography variant="subtitle1" color="textSecondary" sx={{ fontWeight: PESOS.regular }}>
          Quem administra cada condomínio.
        </Typography>
      </Box>
      <ComEstado consulta={consulta}>
        {(estado) => (
          <AdministradorasAba
            administradoras={estado.administradoras}
            condominios={estado.mapeamento.condominios}
          />
        )}
      </ComEstado>
      <Box component="nav" aria-label="Tela antiga" sx={{ display: 'flex', gap: 1.5 }}>
        <Typography variant="caption" color="textSecondary">
          Tela antiga:
        </Typography>
        <LinkExterno href="/backdoor/administradoras" variant="caption">
          Renomear administradora
        </LinkExterno>
      </Box>
    </Box>
  );
}
