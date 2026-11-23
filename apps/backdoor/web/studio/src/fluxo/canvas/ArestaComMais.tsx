import AddIcon from '@mui/icons-material/Add';
import { IconButton, useTheme } from '@mui/material';
import {
  BaseEdge,
  type Edge,
  EdgeLabelRenderer,
  type EdgeProps,
  getStraightPath,
} from '@xyflow/react';
import { createContext, type MouseEvent, type ReactNode, useContext } from 'react';
import { ICONES } from '@/theme/tokens';

/** O "+" só pede: quem abre o menu e insere é o canvas. */
export type PedirInsercao = (indice: number, ancora: HTMLElement) => void;

export const PedirInsercaoContext = createContext<PedirInsercao | null>(null);

type Posicao = { x: number; y: number };
type BotaoMaisProps = { indice: number; rotulo: string; posicao: Posicao };

const estiloDoBotao = ({ x, y }: Posicao) => ({
  position: 'absolute',
  transform: `translate(-50%, -50%) translate(${x}px, ${y}px)`,
  pointerEvents: 'all',
  width: 24,
  height: 24,
  bgcolor: 'studio.nodeBg',
  border: 1,
  borderColor: 'studio.nodeBorder',
  color: 'text.secondary',
  transition: 'transform 120ms ease, border-color 120ms ease, color 120ms ease',
  '&:hover, &:focus-visible': {
    bgcolor: 'studio.nodeBg',
    borderColor: 'studio.nodeSelected',
    color: 'studio.nodeSelected',
  },
});

/** Botão "+" centrado em `posicao` (coordenadas do elemento pai). Some quando não há quem insira. */
export function BotaoMais({ indice, rotulo, posicao }: BotaoMaisProps): ReactNode {
  const pedirInsercao = useContext(PedirInsercaoContext);
  if (pedirInsercao === null) return null;
  const pedir = (evento: MouseEvent<HTMLButtonElement>) => {
    evento.stopPropagation();
    pedirInsercao(indice, evento.currentTarget);
  };
  return (
    <IconButton
      size="small"
      aria-label={rotulo}
      className="nodrag nopan"
      onClick={pedir}
      sx={estiloDoBotao(posicao)}
    >
      <AddIcon sx={{ fontSize: ICONES.medio }} />
    </IconButton>
  );
}

export type ArestaComMaisTipo = Edge<{ indice: number }, 'comMais'>;

export function ArestaComMais({ id, data, ...pontas }: EdgeProps<ArestaComMaisTipo>): ReactNode {
  const [caminho, meioX, meioY] = getStraightPath(pontas);
  const { conector } = useTheme().palette.studio;
  return (
    <>
      <BaseEdge id={id} path={caminho} style={{ stroke: conector, strokeWidth: 2 }} />
      {data && (
        <EdgeLabelRenderer>
          <BotaoMais
            indice={data.indice}
            rotulo="Adicionar bloco aqui"
            posicao={{ x: meioX, y: meioY }}
          />
        </EdgeLabelRenderer>
      )}
    </>
  );
}
