import AddIcon from '@mui/icons-material/Add';
import CheckCircleOutlineIcon from '@mui/icons-material/CheckCircleOutlineOutlined';
import ErrorOutlineIcon from '@mui/icons-material/ErrorOutlineOutlined';
import WarningAmberIcon from '@mui/icons-material/WarningAmber';
import { Box, Tooltip, Typography } from '@mui/material';
import { Handle, type Node, type NodeProps, Position } from '@xyflow/react';
import { type MouseEvent, type ReactNode, useContext } from 'react';
import { BLOCOS } from '@/fluxo/modelo/catalogo';
import { ALTURA_BLOCO, ESPACO, LARGURA_BLOCO } from '@/fluxo/modelo/edicao';
import type { Bloco, ProblemaFluxo, TipoBloco } from '@/fluxo/modelo/tipos';
import { useCorDoBloco } from '@/fluxo/modelo/useCorDoBloco';
import { ICONES, PESOS, RAIO, RAIOS, SOMBRA_DO_BLOCO_EM_EDICAO, TIPOS } from '@/theme/tokens';
import { BotaoMais, PedirInsercaoContext } from './ArestaComMais';

export type DadosDoBloco = {
  bloco: Bloco;
  indice: number;
  resumo: string;
  problemas: ProblemaFluxo[];
  selecionado: boolean;
  /** Mudado por uma proposta do Copiloto há pouco. */
  realcado: boolean;
  /** Último bloco da lista: mostra "Adicionar passo" abaixo do cartão. */
  ultimo: boolean;
};
export type BlocoNodeTipo = Node<DadosDoBloco, 'bloco'>;

const ALCA_INVISIVEL = { opacity: 0, pointerEvents: 'none' } as const;

const estiloDoCartao = (selecionado: boolean, realcado: boolean) => ({
  position: 'relative',
  width: LARGURA_BLOCO,
  height: ALTURA_BLOCO,
  display: 'flex',
  alignItems: 'center',
  gap: 1.5,
  pl: 1.5,
  pr: 2,
  bgcolor: realcado ? 'studio.realceDoCopiloto' : 'studio.nodeBg',
  border: selecionado ? 2 : 1,
  borderStyle: 'solid',
  borderColor: selecionado ? 'studio.nodeSelected' : 'studio.nodeBorder',
  borderLeftWidth: 4,
  borderRadius: `${RAIOS.cartao}px`,
  cursor: 'pointer',
  transition: 'border-color 120ms ease, background-color 300ms ease-out',
});

function NumeroDoBloco({ indice, cor }: { indice: number; cor: string }): ReactNode {
  return (
    <Typography
      component="span"
      aria-hidden="true"
      sx={{
        width: 18,
        flexShrink: 0,
        fontSize: TIPOS.overline,
        fontWeight: PESOS.semi,
        color: cor,
      }}
    >
      {indice + 1}
    </Typography>
  );
}

const CONFIRMADO = new Set<TipoBloco>(['gravar_no_sistema']);

/** Abaixo do último cartão: pílula tracejada "Adicionar passo" (o "+" redondo já cobre o meio). */
function BotaoAdicionarPasso({ indice }: { indice: number }): ReactNode {
  const pedirInsercao = useContext(PedirInsercaoContext);
  if (pedirInsercao === null) return null;
  const pedir = (evento: MouseEvent<HTMLButtonElement>) => {
    evento.stopPropagation();
    pedirInsercao(indice, evento.currentTarget);
  };
  return (
    <Box
      component="button"
      type="button"
      onClick={pedir}
      className="nodrag nopan"
      sx={{
        position: 'absolute',
        left: '50%',
        top: ALTURA_BLOCO + ESPACO / 2,
        transform: 'translate(-50%, 0)',
        display: 'flex',
        alignItems: 'center',
        gap: 1,
        height: 38,
        px: 2,
        bgcolor: 'studio.nodeBg',
        border: 1,
        borderStyle: 'dashed',
        borderColor: 'text.faint',
        borderRadius: RAIO.menu,
        color: 'text.secondary',
        fontSize: TIPOS.body2,
        fontWeight: PESOS.medio,
        fontFamily: 'inherit',
        cursor: 'pointer',
      }}
    >
      <AddIcon sx={{ fontSize: ICONES.pequeno }} />
      Adicionar passo
    </Box>
  );
}

export function BlocoNode({ data }: NodeProps<BlocoNodeTipo>): ReactNode {
  const { bloco, indice, resumo, problemas, selecionado, realcado, ultimo } = data;
  const { rotulo } = BLOCOS[bloco.tipo];
  const cor = useCorDoBloco(bloco.tipo);
  return (
    <Box
      component="article"
      aria-label={rotulo}
      sx={estiloDoCartao(selecionado, realcado)}
      style={{
        borderLeftColor: cor,
        boxShadow: selecionado ? SOMBRA_DO_BLOCO_EM_EDICAO : 'none',
      }}
    >
      <Handle type="target" position={Position.Top} isConnectable={false} style={ALCA_INVISIVEL} />
      {indice === 0 && (
        <BotaoMais
          indice={0}
          rotulo="Adicionar bloco no início"
          posicao={{ x: LARGURA_BLOCO / 2, y: -ESPACO / 2 }}
        />
      )}
      <NumeroDoBloco indice={indice} cor={cor} />
      <Box sx={{ minWidth: 0, flex: 1 }}>
        <Typography
          component="p"
          noWrap
          sx={{
            fontSize: TIPOS.overline,
            fontWeight: PESOS.semi,
            letterSpacing: '0.055em',
            textTransform: 'uppercase',
            color: cor,
          }}
        >
          {rotulo}
        </Typography>
        <Typography variant="body2" color="textPrimary" noWrap component="p">
          {resumo}
        </Typography>
      </Box>
      {selecionado && (
        <Typography
          component="span"
          sx={{ fontSize: TIPOS.overline, fontWeight: PESOS.semi, color: 'studio.nodeSelected' }}
        >
          editando
        </Typography>
      )}
      {CONFIRMADO.has(bloco.tipo) && (
        <Box
          component="span"
          role="img"
          aria-label="Tem confirmação antes"
          sx={{ display: 'grid', placeItems: 'center', color: 'success.main' }}
        >
          <CheckCircleOutlineIcon sx={{ fontSize: ICONES.grande }} />
        </Box>
      )}
      <SeloDeProblema problemas={problemas} />
      {ultimo && <BotaoAdicionarPasso indice={indice + 1} />}
      <Handle
        type="source"
        position={Position.Bottom}
        isConnectable={false}
        style={ALCA_INVISIVEL}
      />
    </Box>
  );
}

const corDoProblema = (temErro: boolean): string => (temErro ? 'error.main' : 'warning.main');

function SeloDeProblema({ problemas }: { problemas: ProblemaFluxo[] }): ReactNode {
  if (problemas.length === 0) return null;
  const temErro = problemas.some((problema) => problema.nivel === 'erro');
  const mensagens = problemas.map((problema) => problema.mensagem).join(' ');
  const Icone = temErro ? ErrorOutlineIcon : WarningAmberIcon;
  return (
    <Tooltip title={mensagens}>
      <Box
        component="span"
        role="img"
        aria-label={`${temErro ? 'Erro' : 'Aviso'}: ${mensagens}`}
        sx={{ display: 'grid', placeItems: 'center', color: corDoProblema(temErro) }}
      >
        <Icone sx={{ fontSize: ICONES.destaque }} />
      </Box>
    </Tooltip>
  );
}
