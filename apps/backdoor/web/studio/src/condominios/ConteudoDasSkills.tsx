import { Accordion, AccordionDetails, AccordionSummary, Box, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import { AjustesDaSkill } from './AjustesDaSkill';
import type { Estado } from './contratos';
import { OrientacaoDaSkill } from './OrientacaoDaSkill';

type Props = { estado: Estado; condominio: string; editavel: boolean };

export function ConteudoDasSkills({ estado, condominio, editavel }: Props): ReactNode {
  return (
    <Box>
      <Typography variant="h5" component="h2" sx={{ mb: 1.5 }}>
        Conteúdo por skill
      </Typography>
      {estado.casos.map((caso) => (
        <Accordion key={caso.nome} slotProps={{ transition: { unmountOnExit: true } }}>
          <AccordionSummary>{caso.titulo || caso.nome}</AccordionSummary>
          <AccordionDetails sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
            <OrientacaoDaSkill
              estado={estado}
              condominio={condominio}
              caso={caso}
              editavel={editavel}
            />
            <AjustesDaSkill
              estado={estado}
              condominio={condominio}
              caso={caso}
              editavel={editavel}
            />
          </AccordionDetails>
        </Accordion>
      ))}
    </Box>
  );
}
