import { Box, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import type { Ferramenta } from '@/catalogo/contratos';
import { tituloDe } from '@/ferramentas/contratos';
import { RAIOS } from '@/theme/tokens';
import { CamposDaFerramenta } from './CamposDaFerramenta';
import { type Caso, type Estado, ferramentasDaSkill } from './contratos';
import { SecaoComSalvamento } from './SecaoComSalvamento';
import { type AjustesEmEdicao, useAjustes } from './useAjustes';

const ESTILO_DO_CARTAO = {
  border: 1,
  borderColor: 'border.subtle',
  borderRadius: `${RAIOS.menu}px`,
  p: 2,
} as const;

type Props = { estado: Estado; condominio: string; caso: Caso; editavel: boolean };

export function AjustesDaSkill({ estado, condominio, caso, editavel }: Props): ReactNode {
  const ferramentas = ferramentasDaSkill(estado, caso);
  const ajustes = useAjustes({ estado, condominio, ferramentas });

  if (ferramentas.length === 0) return null;
  const comum = { ajustes, editavel };

  return (
    <SecaoComSalvamento
      titulo="Ajustes desta skill"
      salvamento={ajustes.salvamento}
      tentarDeNovo={ajustes.tentarDeNovo}
    >
      <Box sx={[ESTILO_DO_CARTAO, { display: 'flex', flexDirection: 'column', gap: 1.5 }]}>
        {ferramentas.map((ferramenta) => (
          <FerramentaDaSkill key={ferramenta.name} ferramenta={ferramenta} {...comum} />
        ))}
      </Box>
    </SecaoComSalvamento>
  );
}

type FerramentaProps = { ferramenta: Ferramenta; ajustes: AjustesEmEdicao; editavel: boolean };

function FerramentaDaSkill({ ferramenta, ajustes, editavel }: FerramentaProps): ReactNode {
  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
      <Typography variant="caption" color="textSecondary">
        {tituloDe(ferramenta)}
      </Typography>
      <CamposDaFerramenta ferramenta={ferramenta} ajustes={ajustes} editavel={editavel} />
    </Box>
  );
}
