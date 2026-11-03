import { Box, Switch, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import { iconeDaSkill } from '@/skills/icones';
import type { Caso } from './contratos';
import type { Origem } from './listaDeSkills';

const FRASE_DE_ORIGEM: Record<Origem, string> = {
  propria: 'Este condomínio tem decisão própria.',
  padrao: 'Sem decisão própria: vale o Padrão. Mudar aqui cria a decisão deste condomínio.',
  nenhuma: 'Sem decisão: ligada para todos.',
};

type Props = {
  caso: Caso;
  titulo: string;
  habilitada: boolean;
  origem: Origem;
  travado: boolean;
  onMudarHabilitada: (ligado: boolean) => void;
};

export function Cabecalho({
  caso,
  titulo,
  habilitada,
  origem,
  travado,
  onMudarHabilitada,
}: Props): ReactNode {
  const Icone = iconeDaSkill(caso.icone);
  return (
    <>
      <Box sx={{ display: 'flex', alignItems: 'flex-start', gap: 1 }}>
        <Icone color="primary" />
        <Box sx={{ flex: 1 }}>
          <Typography variant="h6" component="h2">
            {titulo}
          </Typography>
          {caso.resumo !== '' && (
            <Typography variant="body2" color="text.secondary">
              {caso.resumo}
            </Typography>
          )}
        </Box>
        <Switch
          checked={habilitada}
          disabled={travado}
          slotProps={{ input: { 'aria-label': 'Ligada neste condomínio' } }}
          onChange={(_, ligado) => onMudarHabilitada(ligado)}
        />
      </Box>
      <Typography variant="caption" color="text.secondary">
        {FRASE_DE_ORIGEM[origem]}
      </Typography>
    </>
  );
}
