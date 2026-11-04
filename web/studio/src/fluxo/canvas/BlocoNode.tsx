import ErrorOutlineIcon from '@mui/icons-material/ErrorOutlineOutlined';
import WarningAmberIcon from '@mui/icons-material/WarningAmber';
import { Box, Tooltip, Typography } from '@mui/material';
import { Handle, type Node, type NodeProps, Position } from '@xyflow/react';
import type { ReactNode } from 'react';
import { BLOCOS } from '@/fluxo/modelo/catalogo';
import { ALTURA_BLOCO, ESPACO, LARGURA_BLOCO } from '@/fluxo/modelo/edicao';
import type { Bloco, ProblemaFluxo, TipoBloco } from '@/fluxo/modelo/tipos';
import { useCorDoBloco } from '@/fluxo/modelo/useCorDoBloco';
import { ICONES, PESOS, RAIO, RAIOS, SOMBRA_DO_BLOCO_EM_EDICAO, TIPOS } from '@/theme/tokens';
import { BotaoMais } from './ArestaComMais';

export type DadosDoBloco = {
  bloco: Bloco;
  indice: number;
  resumo: string;
  problemas: ProblemaFluxo[];
  selecionado: boolean;
  /** Mudado por uma proposta do Copiloto há pouco. */
  realcado: boolean;
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

type MarcaProps = { indice: number; cor: string; Icone: (typeof BLOCOS)[TipoBloco]['Icone'] };

function MarcaDoBloco({ indice, cor, Icone }: MarcaProps): ReactNode {
  return (
    <>
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
        {indice + 1}.
      </Typography>
      <Box
        sx={{
          width: 32,
          height: 32,
          flexShrink: 0,
          display: 'grid',
          placeItems: 'center',
          borderRadius: RAIO.campo,
          bgcolor: `${cor}1A`,
          color: cor,
        }}
      >
        <Icone sx={{ fontSize: ICONES.grande }} />
      </Box>
    </>
  );
}

export function BlocoNode({ data }: NodeProps<BlocoNodeTipo>): ReactNode {
  const { bloco, indice, resumo, problemas, selecionado, realcado } = data;
  const { rotulo, Icone } = BLOCOS[bloco.tipo];
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
      <MarcaDoBloco indice={indice} cor={cor} Icone={Icone} />
      <Box sx={{ minWidth: 0, flex: 1 }}>
        <Typography variant="body2" noWrap sx={{ fontWeight: PESOS.negrito }}>
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
