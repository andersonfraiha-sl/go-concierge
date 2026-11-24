import { Box, ToggleButton, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import { PESOS, RAIO } from '@/theme/tokens';
import { paraSentenceCase, rotuloDoPapel } from './listaDeSkills';

const ESTILO_DA_OPCAO = {
  borderRadius: RAIO.pilula,
  textTransform: 'none',
  fontWeight: PESOS.medio,
  px: 1.625,
  py: 0.75,
  border: 1,
  borderColor: 'border.default',
  '&.Mui-selected': {
    bgcolor: 'background.tint',
    borderColor: 'primary.main',
    color: 'primary.main',
    '&:hover': { bgcolor: 'background.tint' },
  },
} as const;

type Props = {
  papeis: readonly string[];
  escolhidos: readonly string[];
  travado: boolean;
  onMudar: (papeis: string[]) => void;
};

function alternar(escolhidos: readonly string[], papel: string): string[] {
  if (escolhidos.includes(papel)) return escolhidos.filter((p) => p !== papel);
  return [...escolhidos, papel];
}

export function QuemPodeUsar(props: Props): ReactNode {
  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
      <Typography variant="subtitle2" component="h3">
        Quem pode usar
      </Typography>
      <Box sx={{ display: 'flex', gap: 1, flexWrap: 'wrap' }}>
        <OpcaoDePapel
          valor="todos"
          rotulo="Todos"
          selecionada={props.escolhidos.length === 0}
          travada={props.travado}
          onEscolher={() => props.onMudar([])}
        />
        <OpcoesPorPapel {...props} />
      </Box>
    </Box>
  );
}

function OpcoesPorPapel({ papeis, escolhidos, travado, onMudar }: Props): ReactNode {
  return papeis.map((papel) => (
    <OpcaoDePapel
      key={papel}
      valor={papel}
      rotulo={paraSentenceCase(rotuloDoPapel(papel))}
      selecionada={escolhidos.includes(papel)}
      travada={travado}
      onEscolher={() => onMudar(alternar(escolhidos, papel))}
    />
  ));
}

type OpcaoProps = {
  valor: string;
  rotulo: string;
  selecionada: boolean;
  travada: boolean;
  onEscolher: () => void;
};

function OpcaoDePapel({ valor, rotulo, selecionada, travada, onEscolher }: OpcaoProps): ReactNode {
  return (
    <ToggleButton
      value={valor}
      selected={selecionada}
      disabled={travada}
      sx={ESTILO_DA_OPCAO}
      onClick={onEscolher}
    >
      {rotulo}
    </ToggleButton>
  );
}
