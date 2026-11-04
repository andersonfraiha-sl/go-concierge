import ExpandMoreOutlined from '@mui/icons-material/ExpandMoreOutlined';
import { Accordion, AccordionDetails, AccordionSummary, Typography } from '@mui/material';
import type { ReactNode } from 'react';

export function ComoReconhece({ descricao }: { descricao: string }): ReactNode {
  return (
    <Accordion disableGutters sx={{ boxShadow: 'none', border: 1, borderColor: 'divider' }}>
      <AccordionSummary expandIcon={<ExpandMoreOutlined />}>
        <Typography variant="subtitle2" component="h3">
          Como o concierge reconhece o pedido
        </Typography>
      </AccordionSummary>
      <AccordionDetails>
        <Typography variant="body2" color="text.secondary">
          {descricao}
        </Typography>
      </AccordionDetails>
    </Accordion>
  );
}
