import { Box, List, ListItemButton, ListItemText, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import { Link } from 'react-router';
import { useContextoDaTela } from '@/copiloto/useContextoDaTela';
import { ComEstado } from './ComEstado';
import { PADRAO, useEscopo, useEstado } from './contratos';

export function CondominiosPage(): ReactNode {
  const consulta = useEstado();
  const escopo = useEscopo();
  useContextoDaTela({ tela: 'condominios' });
  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
      <Typography variant="h4" component="h1">
        Conteúdo
      </Typography>
      <Typography color="textSecondary">
        Escolha a empresa e o condomínio para editar orientações e ajustes.
      </Typography>
      <ComEstado consulta={consulta}>
        {(estado) => (
          <List>
            {escopo === null && (
              <ListItemButton component={Link} to={`/condominios/${encodeURIComponent(PADRAO)}`}>
                <ListItemText primary="Conteúdo global" />
              </ListItemButton>
            )}
            {estado.administradoras.map((company) => (
              <ListItemButton
                key={company.id}
                component={Link}
                to={`/condominios/${encodeURIComponent(PADRAO)}?company_id=${encodeURIComponent(company.id)}`}
              >
                <ListItemText primary={company.nome} secondary="Conteúdo da empresa" />
              </ListItemButton>
            ))}
            {estado.mapeamento.condominios.map((condo) => (
              <ListItemButton
                key={`${condo.administradora_id}:${condo.id}`}
                component={Link}
                to={`/condominios/${encodeURIComponent(condo.id)}?company_id=${encodeURIComponent(condo.administradora_id)}`}
              >
                <ListItemText
                  primary={condo.nome}
                  secondary={`Empresa ${condo.administradora_id} · Condomínio ${condo.id}`}
                />
              </ListItemButton>
            ))}
          </List>
        )}
      </ComEstado>
    </Box>
  );
}
