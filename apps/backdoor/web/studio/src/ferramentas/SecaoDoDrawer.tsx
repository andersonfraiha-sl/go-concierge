import ChevronRightIcon from '@mui/icons-material/ChevronRight';
import { Accordion, AccordionDetails, AccordionSummary, Box, Typography } from '@mui/material';
import type { Theme } from '@mui/material/styles';
import type { ReactNode } from 'react';
import { PESOS, RAIO, TIPOS } from '@/theme/tokens';

type Props = {
  titulo: string;
  /** Contagem curta ao lado do nome, ex. "4" ou "2 campos" (board: cinza, fonte caption). */
  badge?: string | undefined;
  aberta: boolean;
  onAlternar: () => void;
  children: ReactNode;
};

const estiloDoResumo = (theme: Theme) => ({
  fontFamily: theme.typography.fontFamily,
  fontWeight: PESOS.semi,
  fontSize: TIPOS.body1,
  '& .MuiAccordionSummary-expandIconWrapper': { transform: 'rotate(0deg)' },
  '& .MuiAccordionSummary-expandIconWrapper.Mui-expanded': { transform: 'rotate(90deg)' },
});

// por que: JSX declarativo sem lógica; separar o Accordion só espalharia o markup.
/** Seção recolhível do drawer; o Básico abre sozinho, as demais o usuário abre por conta própria. */
export function SecaoDoDrawer({ titulo, badge, aberta, onAlternar, children }: Props): ReactNode {
  return (
    <Accordion
      disableGutters
      variant="outlined"
      expanded={aberta}
      onChange={onAlternar}
      sx={{ borderRadius: RAIO.painel, overflow: 'hidden', '&::before': { display: 'none' } }}
    >
      <AccordionSummary
        expandIcon={<ChevronRightIcon fontSize="small" sx={{ color: 'text.muted' }} />}
        sx={estiloDoResumo}
      >
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, flex: 1 }}>
          <Box sx={{ flex: 1 }}>{titulo}</Box>
          {badge && (
            <Typography component="span" sx={{ fontSize: TIPOS.caption, color: 'text.muted' }}>
              {badge}
            </Typography>
          )}
        </Box>
      </AccordionSummary>
      <AccordionDetails sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
        {children}
      </AccordionDetails>
    </Accordion>
  );
}
