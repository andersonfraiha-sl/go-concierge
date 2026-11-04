import '@xyflow/react/dist/style.css';
import { Box, useTheme } from '@mui/material';
import {
  applyNodeChanges,
  Background,
  BackgroundVariant,
  Controls,
  type NodeChange,
  type OnNodeDrag,
  ReactFlow,
} from '@xyflow/react';
import { type ReactNode, useMemo, useState } from 'react';
import { resumoDoBloco } from '@/fluxo/modelo/catalogo';
import { ALTURA_BLOCO, ESPACO, indiceAoSoltar } from '@/fluxo/modelo/edicao';
import type { Fluxo, ProblemaFluxo, TipoBloco } from '@/fluxo/modelo/tipos';
import {
  ArestaComMais,
  type ArestaComMaisTipo,
  type PedirInsercao,
  PedirInsercaoContext,
} from './ArestaComMais';
import { BlocoNode, type BlocoNodeTipo } from './BlocoNode';
import { MenuDeBlocos } from './MenuDeBlocos';

const TIPOS_DE_NO = { bloco: BlocoNode };
const TIPOS_DE_ARESTA = { comMais: ArestaComMais };
const NENHUM_REALCE: ReadonlySet<string> = new Set();

type Props = {
  fluxo: Fluxo;
  problemas: ProblemaFluxo[];
  selecionado: string | null;
  editavel: boolean;
  /** Título que o resumo do bloco mostra para uma ferramenta; precisa ser estável entre renders. */
  tituloDaFerramenta: (nome: string) => string;
  /** Blocos que o Copiloto acabou de mudar; precisa ser estável entre renders. */
  realcados?: ReadonlySet<string>;
  onSelecionar: (id: string | null) => void;
  onInserir: (indice: number, tipo: TipoBloco) => void;
  onMover: (de: number, para: number) => void;
};

type PedidoDeInsercao = { indice: number; ancora: HTMLElement };

function problemasPorBloco(problemas: readonly ProblemaFluxo[]): Map<string, ProblemaFluxo[]> {
  const porBloco = new Map<string, ProblemaFluxo[]>();
  for (const problema of problemas) {
    if (problema.blocoId === undefined) continue;
    porBloco.set(problema.blocoId, [...(porBloco.get(problema.blocoId) ?? []), problema]);
  }
  return porBloco;
}

/** Nós novos herdam a medida dos antigos: sem ela o React Flow esconde o nó até medir de novo. */
const comMedidas = (
  anteriores: readonly BlocoNodeTipo[],
  novos: BlocoNodeTipo[],
): BlocoNodeTipo[] =>
  novos.map((no) => {
    const medido = anteriores.find((anterior) => anterior.id === no.id)?.measured;
    return medido ? { ...no, measured: medido } : no;
  });

type OpcoesDosNos = Pick<Props, 'problemas' | 'selecionado' | 'editavel' | 'tituloDaFerramenta'> & {
  blocos: Fluxo['blocos'];
  realcados: ReadonlySet<string>;
};

function nosDoFluxo(opcoes: OpcoesDosNos): BlocoNodeTipo[] {
  const { blocos, problemas, selecionado, editavel, tituloDaFerramenta, realcados } = opcoes;
  const porBloco = problemasPorBloco(problemas);
  return blocos.map((bloco, indice) => ({
    id: bloco.id,
    type: 'bloco',
    position: { x: 0, y: indice * (ALTURA_BLOCO + ESPACO) },
    draggable: editavel && bloco.tipo !== 'responder',
    selected: bloco.id === selecionado,
    data: {
      bloco,
      indice,
      resumo: resumoDoBloco(bloco, tituloDaFerramenta),
      problemas: porBloco.get(bloco.id) ?? [],
      selecionado: bloco.id === selecionado,
      realcado: realcados.has(bloco.id),
    },
  }));
}

function arestasDoFluxo(blocos: Fluxo['blocos']): ArestaComMaisTipo[] {
  return blocos.slice(1).map((bloco, posicao) => ({
    id: `${blocos[posicao]?.id ?? ''}->${bloco.id}`,
    source: blocos[posicao]?.id ?? '',
    target: bloco.id,
    type: 'comMais',
    data: { indice: posicao + 1 },
  }));
}

/** Os nós ficam em estado local só para o arrasto andar na tela; a ordem verdadeira é o `fluxo`. */
function useNosArrastaveis(derivados: BlocoNodeTipo[]) {
  const [nos, setNos] = useState(derivados);
  const [origem, setOrigem] = useState(derivados);
  if (origem !== derivados) {
    setOrigem(derivados);
    setNos((anteriores) => comMedidas(anteriores, derivados));
  }

  const aoMudarNos = (mudancas: NodeChange<BlocoNodeTipo>[]) => {
    const aplicaveis = mudancas.filter(
      (mudanca) => mudanca.type !== 'select' && mudanca.type !== 'remove',
    );
    setNos((atuais) => applyNodeChanges(aplicaveis, atuais));
  };
  const remedir = () => setNos((atuais) => comMedidas(atuais, derivados));

  return { nos, aoMudarNos, remedir };
}

export function FluxoCanvas({
  fluxo,
  problemas,
  selecionado,
  editavel,
  tituloDaFerramenta,
  realcados = NENHUM_REALCE,
  onSelecionar,
  onInserir,
  onMover,
}: Props): ReactNode {
  const derivados = useMemo(
    () =>
      nosDoFluxo({
        blocos: fluxo.blocos,
        problemas,
        selecionado,
        editavel,
        tituloDaFerramenta,
        realcados,
      }),
    [fluxo.blocos, problemas, selecionado, editavel, tituloDaFerramenta, realcados],
  );
  const arestas = useMemo(() => arestasDoFluxo(fluxo.blocos), [fluxo.blocos]);
  const { nos, aoMudarNos, remedir } = useNosArrastaveis(derivados);

  const [pedido, setPedido] = useState<PedidoDeInsercao | null>(null);
  const { canvasDot } = useTheme().palette.studio;
  const pedirInsercao = useMemo<PedirInsercao | null>(
    () => (editavel ? (indice, ancora) => setPedido({ indice, ancora }) : null),
    [editavel],
  );

  const aoSoltar: OnNodeDrag<BlocoNodeTipo> = (_, no) => {
    const indiceAtual = fluxo.blocos.findIndex((bloco) => bloco.id === no.id);
    const indiceNovo = indiceAoSoltar(no.position.y, fluxo.blocos.length);
    if (indiceAtual !== indiceNovo) return onMover(indiceAtual, indiceNovo);
    remedir();
  };

  const escolher = (tipo: TipoBloco) => {
    if (pedido) onInserir(pedido.indice, tipo);
    setPedido(null);
  };

  // por que: JSX declarativo do ReactFlow; fatiar só espalharia as props.
  return (
    <PedirInsercaoContext value={pedirInsercao}>
      <Box
        component="section"
        aria-label="Fluxo"
        sx={{ height: '100%', width: '100%', bgcolor: 'studio.canvasBg' }}
      >
        <ReactFlow<BlocoNodeTipo, ArestaComMaisTipo>
          nodes={nos}
          edges={arestas}
          nodeTypes={TIPOS_DE_NO}
          edgeTypes={TIPOS_DE_ARESTA}
          onNodesChange={aoMudarNos}
          onNodeClick={(_, no) => onSelecionar(no.id)}
          onPaneClick={() => onSelecionar(null)}
          onNodeDragStop={aoSoltar}
          nodesDraggable={editavel}
          nodesConnectable={false}
          edgesFocusable={false}
          deleteKeyCode={null}
          fitView
          fitViewOptions={{ padding: 0.3, maxZoom: 1 }}
          minZoom={0.3}
          maxZoom={1.5}
          proOptions={{ hideAttribution: true }}
        >
          <Background variant={BackgroundVariant.Dots} gap={20} size={1.5} color={canvasDot} />
          <Controls showInteractive={false} />
        </ReactFlow>
        <MenuDeBlocos
          ancora={pedido?.ancora ?? null}
          onEscolher={escolher}
          onFechar={() => setPedido(null)}
        />
      </Box>
    </PedirInsercaoContext>
  );
}
