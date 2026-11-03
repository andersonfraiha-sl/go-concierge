import ExpandMoreIcon from '@mui/icons-material/ExpandMore';
import { Accordion, AccordionDetails, AccordionSummary } from '@mui/material';
import type { ReactNode } from 'react';

type Props = {
  titulo: string;
  aberta: boolean;
  onAlternar: () => void;
  children: ReactNode;
};

/** Seção recolhível do drawer; o Básico abre sozinho, as demais o usuário abre por conta própria. */
export function SecaoDoDrawer({ titulo, aberta, onAlternar, children }: Props): ReactNode {
  return (
    <Accordion disableGutters variant="outlined" expanded={aberta} onChange={onAlternar}>
      <AccordionSummary expandIcon={<ExpandMoreIcon fontSize="small" />}>{titulo}</AccordionSummary>
      <AccordionDetails sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
        {children}
      </AccordionDetails>
    </Accordion>
  );
}
