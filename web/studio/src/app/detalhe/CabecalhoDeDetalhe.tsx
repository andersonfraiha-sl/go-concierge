import type { SvgIconComponent } from '@mui/icons-material';
import LockOutlinedIcon from '@mui/icons-material/LockOutlined';
import {
  Box,
  Button,
  IconButton,
  type SxProps,
  Tab,
  Tabs,
  type Theme,
  Typography,
} from '@mui/material';
import type { ReactNode } from 'react';
import { createPortal } from 'react-dom';
import { Link } from 'react-router';
import { formatarHora } from '@/shared/autosave/SaveIndicator';
import { ICONES, PESOS, RAIO, SHELL, TIPOS, tokens } from '@/theme/tokens';
import { BotaoDoCopiloto } from '../BotaoDoCopiloto';
import { IconeDeLinha } from '../iconesDoShell';
import { useDetalhe } from './LayoutDeDetalhe';

export type AbaDoDetalhe<Id extends string> = { id: Id; rotulo: string };

export type AbasDoDetalhe<Id extends string> = {
  /** Nome acessível da lista de abas, ex.: "Abas da skill". */
  rotulo: string;
  itens: readonly AbaDoDetalhe<Id>[];
  atual: Id;
  onTrocar: (id: Id) => void;
};

export type PropsDoCabecalho<Id extends string> = {
  /** Destino do botão voltar (rota do SPA) e o nome acessível dele. */
  voltar: { para: string; rotulo: string };
  /** Ícone ou avatar da entidade, à esquerda do título (ver IconeDaEntidade). */
  icone?: ReactNode;
  titulo: ReactNode;
  subtitulo?: ReactNode;
  /** Itens extras à direita, antes do "Salvo às" (ex.: o aviso do Avançado). */
  acoes?: ReactNode;
  /** Mostra "Testar no Console", link para o Console legado. */
  testarNoConsole?: boolean;
  /** Quem não pode editar vê o selo "Só leitura" no lugar de "Testar no Console". */
  somenteLeitura?: boolean;
  /** Abas em largura total logo abaixo do cabeçalho. */
  abas?: AbasDoDetalhe<Id>;
};

/**
 * Cabeçalho das telas de detalhe: substitui a barra global. Dentro do LayoutDeDetalhe
 * se desenha na vaga do topo (acima do Copiloto); fora dele, no próprio lugar.
 * "Salvo às" vem sozinho do último autosave bem-sucedido da tela.
 */
export function CabecalhoDeDetalhe<Id extends string>(props: PropsDoCabecalho<Id>): ReactNode {
  const detalhe = useDetalhe();
  const cabecalho = <Cabecalho {...props} salvoEm={detalhe?.salvoEm ?? null} />;
  if (detalhe === undefined) return cabecalho;
  if (detalhe.vaga === null) return null;
  return createPortal(cabecalho, detalhe.vaga);
}

const ESTILO_DA_FAIXA: SxProps<Theme> = {
  height: tokens.layout.navbarHeight,
  // Como no desenho (HTML sem reset), altura e largura não contam padding nem borda.
  boxSizing: 'content-box',
  display: 'flex',
  alignItems: 'center',
  gap: 2,
  px: 4,
  borderBottom: 1,
  borderColor: 'border.default',
  bgcolor: 'background.paper',
};

function Cabecalho<Id extends string>({
  salvoEm,
  ...props
}: PropsDoCabecalho<Id> & { salvoEm: Date | null }): ReactNode {
  return (
    <>
      <Box component="header" sx={ESTILO_DA_FAIXA}>
        <BotaoVoltar {...props.voltar} />
        {props.icone}
        <TituloESubtitulo titulo={props.titulo} subtitulo={props.subtitulo} />
        {props.acoes}
        {salvoEm && <SalvoAs em={salvoEm} />}
        {props.somenteLeitura ? <SeloSoLeitura /> : props.testarNoConsole && <TestarNoConsole />}
        <BotaoDoCopiloto variante="detalhe" />
      </Box>
      {props.abas && <AbasEmFaixa {...props.abas} />}
    </>
  );
}

const ESTILO_DO_QUADRADO = {
  width: SHELL.logo,
  height: SHELL.logo,
  flexShrink: 0,
  borderRadius: RAIO.chip,
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
} as const;

function BotaoVoltar({ para, rotulo }: { para: string; rotulo: string }): ReactNode {
  return (
    <IconButton
      component={Link}
      to={para}
      aria-label={rotulo}
      sx={{
        ...ESTILO_DO_QUADRADO,
        boxSizing: 'content-box',
        p: 0,
        border: 1,
        borderColor: 'border.default',
        color: 'text.secondary',
      }}
    >
      <IconeDeLinha nome="voltar" tamanho={ICONES.medio} traco={SHELL.tracoDoVoltar} />
    </IconButton>
  );
}

/** Tom do quadrado da entidade: laranja para skill que registra, azul para o resto. */
export type TomDaEntidade = 'registra' | 'consulta';

const CORES_DO_TOM: Record<TomDaEntidade, { bgcolor: string; color: string }> = {
  registra: { bgcolor: 'studio.registra.soft', color: 'studio.registra.onSoft' },
  consulta: { bgcolor: 'background.tint', color: 'primary.main' },
};

export function IconeDaEntidade({ Icone, tom }: { Icone: SvgIconComponent; tom: TomDaEntidade }) {
  return (
    <Box aria-hidden sx={{ ...ESTILO_DO_QUADRADO, ...CORES_DO_TOM[tom] }}>
      <Icone sx={{ fontSize: ICONES.entidade }} />
    </Box>
  );
}

function TituloESubtitulo({ titulo, subtitulo }: { titulo: ReactNode; subtitulo: ReactNode }) {
  return (
    <Box
      sx={{
        flex: 1,
        minWidth: 0,
        display: 'flex',
        flexDirection: 'column',
        lineHeight: SHELL.alturaDaLinhaDoTitulo,
      }}
    >
      <Typography
        variant="h6"
        component="h1"
        noWrap
        sx={{ color: 'secondary.main', lineHeight: 'inherit' }}
      >
        {titulo}
      </Typography>
      {subtitulo && (
        <Typography
          component="span"
          noWrap
          sx={{ fontSize: TIPOS.caption, color: 'text.tertiary', lineHeight: 'inherit' }}
        >
          {subtitulo}
        </Typography>
      )}
    </Box>
  );
}

function SalvoAs({ em }: { em: Date }): ReactNode {
  return (
    <Box
      role="status"
      sx={{
        flexShrink: 0,
        display: 'flex',
        alignItems: 'center',
        gap: 0.75,
        fontSize: TIPOS.caption,
        color: 'success.onSoft',
      }}
    >
      <Box
        sx={{
          width: SHELL.pontoDeSalvo,
          height: SHELL.pontoDeSalvo,
          borderRadius: RAIO.pilula,
          bgcolor: 'success.main',
        }}
      />
      Salvo às {formatarHora(em)}
    </Box>
  );
}

function SeloSoLeitura(): ReactNode {
  return (
    <Box
      component="span"
      sx={{
        flexShrink: 0,
        display: 'flex',
        alignItems: 'center',
        gap: 0.75,
        px: 1.25,
        py: 0.5,
        borderRadius: RAIO.pilula,
        bgcolor: 'background.neutralFill',
        color: 'text.secondary',
        fontSize: TIPOS.caption,
        fontWeight: PESOS.medio,
      }}
    >
      <LockOutlinedIcon sx={{ fontSize: ICONES.minimo }} aria-hidden />
      Só leitura
    </Box>
  );
}

// O Console é a tela legada fora do SPA: navegação completa, sem o basename.
function TestarNoConsole(): ReactNode {
  return (
    <Button
      component="a"
      href="/console"
      sx={{
        flexShrink: 0,
        height: SHELL.alturaDoBotaoDoDetalhe,
        px: `${SHELL.paddingDoBotaoDeTeste}px`,
        border: 1,
        borderColor: 'border.default',
        borderRadius: RAIO.chip,
        bgcolor: 'background.paper',
        color: 'text.strong',
        fontSize: TIPOS.body2,
        fontWeight: PESOS.medio,
        lineHeight: 'normal',
      }}
    >
      Testar no Console
    </Button>
  );
}

const ESTILO_DAS_ABAS: SxProps<Theme> = {
  minHeight: 0,
  px: 4,
  borderBottom: 1,
  borderColor: 'border.default',
  bgcolor: 'background.paper',
  '& .MuiTabs-list': { gap: `${SHELL.gapDasAbas}px` },
  '& .MuiTabs-indicator': { height: SHELL.indicadorDaAba },
};

const ESTILO_DA_ABA: SxProps<Theme> = {
  minWidth: 0,
  minHeight: 0,
  px: 0,
  pt: `${SHELL.paddingDaAba}px`,
  pb: `${SHELL.paddingDaAba + SHELL.indicadorDaAba}px`,
  fontSize: TIPOS.body1,
  fontWeight: PESOS.regular,
  lineHeight: 'normal',
  color: 'text.secondary',
  '&.Mui-selected': { color: 'primary.main', fontWeight: PESOS.semi },
};

function AbasEmFaixa<Id extends string>({ rotulo, itens, atual, onTrocar }: AbasDoDetalhe<Id>) {
  return (
    <Tabs
      value={atual}
      onChange={(_, proxima: Id) => onTrocar(proxima)}
      variant="scrollable"
      scrollButtons={false}
      aria-label={rotulo}
      sx={ESTILO_DAS_ABAS}
    >
      {itens.map(({ id, rotulo: texto }) => (
        <Tab key={id} value={id} label={texto} disableRipple sx={ESTILO_DA_ABA} />
      ))}
    </Tabs>
  );
}
