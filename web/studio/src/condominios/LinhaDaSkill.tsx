import ChevronRightOutlined from '@mui/icons-material/ChevronRightOutlined';
import {
  Box,
  ListItemButton,
  Switch,
  type SxProps,
  type Theme,
  Tooltip,
  Typography,
} from '@mui/material';
import type { ReactNode } from 'react';
import { useRealceDoCopiloto } from '@/copiloto/CopilotoProvider';
import { iconeDaSkill } from '@/skills/icones';
import { RAIOS, tokens } from '@/theme/tokens';
import { type LinhaDeSkill, rotuloDePapeis } from './listaDeSkills';

/** Raio de pílula bem acima de qualquer altura real do chip: sempre fica totalmente arredondado. */
const RAIO_DE_PILULA = '999px';

const ESTILO_DA_LINHA = {
  minHeight: 52,
  gap: 1.5,
  px: 2.5,
  borderRadius: 0,
  borderTop: 1,
  borderColor: 'border.subtle',
  '&:hover': { bgcolor: 'background.subtle', color: 'inherit', fontWeight: 'inherit' },
} as const;

const ESTILO_DA_IDENTIDADE = {
  display: 'flex',
  alignItems: 'center',
  gap: 1.5,
  flex: 1,
  minWidth: 0,
} as const;

const ESTILO_DO_ICONE = {
  width: 30,
  height: 30,
  borderRadius: `${RAIOS.campo}px`,
  bgcolor: 'background.tint',
  color: 'primary.main',
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  flex: 'none',
} as const;

const ESTILO_DO_CHIP = {
  px: 1,
  py: 0.25,
  borderRadius: RAIO_DE_PILULA,
  whiteSpace: 'nowrap',
} as const;

const CHIP_DE_PAPEIS = { ...ESTILO_DO_CHIP, bgcolor: 'background.tint', color: 'primary.main' };

const CHIP_DE_DECISAO_PROPRIA = {
  ...ESTILO_DO_CHIP,
  bgcolor: tokens.color.warning.background,
  color: tokens.color.warning.text,
};

type Props = {
  linha: LinhaDeSkill;
  /** Id de realce da linha: `condominio:<id>:skill:<nome>`. */
  alvoDoRealce: string;
  ehPadrao: boolean;
  editavel: boolean;
  gravando: boolean;
  onAbrir: () => void;
  onMudarHabilitado: (habilitado: boolean) => void;
};

/** Linha densa de ~52 px: ícone, título, resumo, chips de exceção, atalho e switch. */
export function LinhaDaSkill(props: Props): ReactNode {
  const { linha, ehPadrao, onAbrir } = props;
  const realce = useRealceDoCopiloto(props.alvoDoRealce);

  return (
    <ListItemButton onClick={onAbrir} sx={[ESTILO_DA_LINHA, realce]}>
      <IdentidadeDaSkill linha={linha} mostraDecisaoPropria={linha.propria && !ehPadrao} />
      <ChevronRightOutlined fontSize="small" sx={{ color: 'text.muted', flex: 'none' }} />
      <Switch
        checked={linha.habilitacao.habilitado}
        disabled={!props.editavel || props.gravando}
        onClick={(evento) => evento.stopPropagation()}
        onChange={(_, ligado) => props.onMudarHabilitado(ligado)}
        slotProps={{ input: { 'aria-label': `${linha.titulo} neste condomínio` } }}
      />
    </ListItemButton>
  );
}

type IdentidadeProps = { linha: LinhaDeSkill; mostraDecisaoPropria: boolean };

function IdentidadeDaSkill({ linha, mostraDecisaoPropria }: IdentidadeProps): ReactNode {
  const Icone = iconeDaSkill(linha.caso.icone);
  const rotuloPapeis = rotuloDePapeis(linha.habilitacao.papeis);
  const opacidade = linha.habilitacao.habilitado ? 1 : 0.6;

  return (
    <Box sx={[ESTILO_DA_IDENTIDADE, { opacity: opacidade }]}>
      <Box sx={ESTILO_DO_ICONE}>
        <Icone sx={{ fontSize: 18 }} />
      </Box>
      <TituloEResumo linha={linha} />
      {rotuloPapeis !== null && <Chip texto={rotuloPapeis} estilo={CHIP_DE_PAPEIS} />}
      {mostraDecisaoPropria && <Chip texto="deste condomínio" estilo={CHIP_DE_DECISAO_PROPRIA} />}
    </Box>
  );
}

function TituloEResumo({ linha }: { linha: LinhaDeSkill }): ReactNode {
  return (
    <Box sx={{ display: 'flex', alignItems: 'baseline', gap: 1.25, flex: 1, minWidth: 0 }}>
      <Typography sx={{ fontWeight: 600, whiteSpace: 'nowrap' }}>{linha.titulo}</Typography>
      <Tooltip title={linha.caso.resumo}>
        <Typography variant="body2" color="text.muted" noWrap sx={{ flex: 1, minWidth: 0 }}>
          {linha.caso.resumo}
        </Typography>
      </Tooltip>
    </Box>
  );
}

function Chip({ texto, estilo }: { texto: string; estilo: SxProps<Theme> }): ReactNode {
  return (
    <Typography variant="caption" sx={estilo}>
      {texto}
    </Typography>
  );
}
