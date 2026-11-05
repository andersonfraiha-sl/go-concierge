import CloseOutlined from '@mui/icons-material/CloseOutlined';
import { Box, IconButton, Switch, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import { iconeDaSkill } from '@/skills/icones';
import { ICONES, RAIOS } from '@/theme/tokens';
import type { Caso } from './contratos';
import { categoriaDe, type Origem, tomDaCategoria } from './listaDeSkills';

const FRASE_DE_ORIGEM: Record<Origem, string> = {
  propria: 'Este condomínio tem decisão própria.',
  padrao: 'Sem decisão própria: vale o Padrão. Mudar aqui cria a decisão deste condomínio.',
  nenhuma: 'Sem decisão: ligada para todos.',
};

const ESTILO_DO_CARTAO_DE_ORIGEM = {
  bgcolor: 'background.subtle',
  borderRadius: `${RAIOS.menu}px`,
  px: 2,
  py: 1.625,
} as const;

type Props = {
  caso: Caso;
  titulo: string;
  habilitada: boolean;
  origem: Origem;
  travado: boolean;
  onMudarHabilitada: (ligado: boolean) => void;
  onFechar: () => void;
};

export function Cabecalho(props: Props): ReactNode {
  return (
    <>
      <LinhaDoTitulo {...props} />
      <Typography variant="body2" color="textSecondary" sx={ESTILO_DO_CARTAO_DE_ORIGEM}>
        {FRASE_DE_ORIGEM[props.origem]}
      </Typography>
    </>
  );
}

const CORES_DO_ICONE = {
  registra: { bgcolor: 'studio.registra.soft', color: 'studio.registra.onSoft' },
  consulta: { bgcolor: 'background.tint', color: 'primary.main' },
} as const;

const ESTILO_DO_BOTAO_DE_FECHAR = {
  width: 30,
  height: 30,
  borderRadius: `${RAIOS.campo - 1}px`,
  bgcolor: 'background.neutralFill',
  color: 'text.secondary',
} as const;

function LinhaDoTitulo(props: Props): ReactNode {
  return (
    <Box sx={{ display: 'flex', alignItems: 'flex-start', gap: 1 }}>
      <TituloDaSkill caso={props.caso} titulo={props.titulo} />
      <Switch
        checked={props.habilitada}
        disabled={props.travado}
        slotProps={{ input: { 'aria-label': 'Ligada neste condomínio' } }}
        onChange={(_, ligado) => props.onMudarHabilitada(ligado)}
      />
      <IconButton
        aria-label="Fechar detalhes"
        onClick={props.onFechar}
        sx={ESTILO_DO_BOTAO_DE_FECHAR}
      >
        <CloseOutlined sx={{ fontSize: ICONES.grande }} />
      </IconButton>
    </Box>
  );
}

const ESTILO_DO_ICONE = {
  width: 30,
  height: 30,
  borderRadius: `${RAIOS.campo}px`,
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  flex: 'none',
} as const;

function TituloDaSkill({ caso, titulo }: Pick<Props, 'caso' | 'titulo'>): ReactNode {
  const Icone = iconeDaSkill(caso.icone);
  const tom = tomDaCategoria(categoriaDe(caso));
  return (
    <>
      <Box sx={[ESTILO_DO_ICONE, CORES_DO_ICONE[tom]]}>
        <Icone sx={{ fontSize: ICONES.grande }} />
      </Box>
      <Box sx={{ flex: 1 }}>
        <Typography variant="overline" color="textTertiary" component="p">
          Neste condomínio
        </Typography>
        <Typography variant="h6" component="h2">
          {titulo}
        </Typography>
      </Box>
    </>
  );
}
