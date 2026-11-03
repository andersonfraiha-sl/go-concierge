import ErrorOutlineIcon from '@mui/icons-material/ErrorOutlineOutlined';
import WarningAmberIcon from '@mui/icons-material/WarningAmber';
import { Box, Tooltip, Typography } from '@mui/material';
import { Handle, type Node, type NodeProps, Position } from '@xyflow/react';
import type { ReactNode } from 'react';
import { BLOCOS } from '@/fluxo/modelo/catalogo';
import { ALTURA_BLOCO, ESPACO, LARGURA_BLOCO } from '@/fluxo/modelo/edicao';
import type { Bloco, ProblemaFluxo } from '@/fluxo/modelo/tipos';
import { RAIOS, SOMBRA_DO_BLOCO_EM_EDICAO, tokens } from '@/theme/tokens';
import { BotaoMais } from './ArestaComMais';

export type DadosDoBloco = {
  bloco: Bloco;
  indice: number;
  resumo: string;
  problemas: ProblemaFluxo[];
  selecionado: boolean;
};
export type BlocoNodeTipo = Node<DadosDoBloco, 'bloco'>;

const ALCA_INVISIVEL = { opacity: 0, pointerEvents: 'none' } as const;

export function BlocoNode({ data }: NodeProps<BlocoNodeTipo>): ReactNode {
  const { bloco, indice, resumo, problemas, selecionado } = data;
  const { rotulo, cor, Icone } = BLOCOS[bloco.tipo];
  return (
    <Box
      component="article"
      aria-label={rotulo}
      sx={{
        position: 'relative',
        width: LARGURA_BLOCO,
        height: ALTURA_BLOCO,
        display: 'flex',
        alignItems: 'center',
        gap: 1.5,
        pl: 1.5,
        pr: 2,
        bgcolor: tokens.color.studio.nodeBg,
        border: selecionado ? 2 : 1,
        borderStyle: 'solid',
        borderColor: selecionado
          ? tokens.color.studio.nodeSelected
          : tokens.color.studio.nodeBorder,
        borderLeftWidth: 4,
        borderRadius: `${RAIOS.cartao}px`,
        cursor: 'pointer',
        transition: 'border-color 120ms ease',
      }}
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
      <Typography
        component="span"
        aria-hidden="true"
        sx={{ width: 18, flexShrink: 0, fontSize: 11, fontWeight: 600, color: cor }}
      >
        {indice + 1}.
      </Typography>
      <Box
        sx={{
          width: 32,
          height: 32,
          flexShrink: 0,
          display: 'grid',
          placeItems: 'center',
          borderRadius: 1,
          bgcolor: `${cor}1A`,
          color: cor,
        }}
      >
        <Icone sx={{ fontSize: 18 }} />
      </Box>
      <Box sx={{ minWidth: 0, flex: 1 }}>
        <Typography variant="body2" noWrap sx={{ fontWeight: 700 }}>
          {rotulo}
        </Typography>
        <Typography variant="caption" color="text.secondary" noWrap component="p">
          {resumo}
        </Typography>
      </Box>
      <SeloDeProblema problemas={problemas} />
      <Handle
        type="source"
        position={Position.Bottom}
        isConnectable={false}
        style={ALCA_INVISIVEL}
      />
    </Box>
  );
}

function SeloDeProblema({ problemas }: { problemas: ProblemaFluxo[] }): ReactNode {
  if (problemas.length === 0) return null;
  const temErro = problemas.some((p) => p.nivel === 'erro');
  const mensagens = problemas.map((p) => p.mensagem).join(' ');
  const Icone = temErro ? ErrorOutlineIcon : WarningAmberIcon;
  return (
    <Tooltip title={mensagens}>
      <Box
        component="span"
        role="img"
        aria-label={`${temErro ? 'Erro' : 'Aviso'}: ${mensagens}`}
        sx={{
          display: 'grid',
          placeItems: 'center',
          color: temErro ? tokens.color.studio.issueError : tokens.color.studio.issueWarning,
        }}
      >
        <Icone sx={{ fontSize: 20 }} />
      </Box>
    </Tooltip>
  );
}
