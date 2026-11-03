import '@xyflow/react/dist/style.css';
import { Box } from '@mui/material';
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
import { tokens } from '@/theme/tokens';
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

type Props = {
  fluxo: Fluxo;
  problemas: ProblemaFluxo[];
  selecionado: string | null;
  editavel: boolean;
  /** Título que o resumo do bloco mostra para uma ferramenta; precisa ser estável entre renders. */
  tituloDaFerramenta: (nome: string) => string;
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
    const medido = anteriores.find((a) => a.id === no.id)?.measured;
    return medido ? { ...no, measured: medido } : no;
  });

export function FluxoCanvas({
  fluxo,
  problemas,
  selecionado,
  editavel,
  tituloDaFerramenta,
  onSelecionar,
  onInserir,
  onMover,
}: Props): ReactNode {
  const derivados = useMemo<BlocoNodeTipo[]>(() => {
    const porBloco = problemasPorBloco(problemas);
    return fluxo.blocos.map((bloco, indice) => ({
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
      },
    }));
  }, [fluxo.blocos, problemas, selecionado, editavel, tituloDaFerramenta]);

  const arestas = useMemo<ArestaComMaisTipo[]>(
    () =>
      fluxo.blocos.slice(1).map((bloco, i) => ({
        id: `${fluxo.blocos[i]?.id ?? ''}->${bloco.id}`,
        source: fluxo.blocos[i]?.id ?? '',
        target: bloco.id,
        type: 'comMais',
        data: { indice: i + 1 },
      })),
    [fluxo.blocos],
  );

  // Os nós ficam em estado local só para o arrasto andar na tela; a ordem verdadeira é o `fluxo`.
  const [nos, setNos] = useState(derivados);
  const [origem, setOrigem] = useState(derivados);
  if (origem !== derivados) {
    setOrigem(derivados);
    setNos((anteriores) => comMedidas(anteriores, derivados));
  }

  const [pedido, setPedido] = useState<PedidoDeInsercao | null>(null);
  const pedirInsercao = useMemo<PedirInsercao | null>(
    () => (editavel ? (indice, ancora) => setPedido({ indice, ancora }) : null),
    [editavel],
  );

  const aoMudarNos = (mudancas: NodeChange<BlocoNodeTipo>[]) =>
    setNos((atuais) =>
      applyNodeChanges(
        mudancas.filter((m) => m.type !== 'select' && m.type !== 'remove'),
        atuais,
      ),
    );

  const aoSoltar: OnNodeDrag<BlocoNodeTipo> = (_, no) => {
    const de = fluxo.blocos.findIndex((b) => b.id === no.id);
    const para = indiceAoSoltar(no.position.y, fluxo.blocos.length);
    if (de !== para) return onMover(de, para);
    setNos((atuais) => comMedidas(atuais, derivados));
  };

  const escolher = (tipo: TipoBloco) => {
    if (pedido) onInserir(pedido.indice, tipo);
    setPedido(null);
  };

  return (
    <PedirInsercaoContext value={pedirInsercao}>
      <Box
        component="section"
        aria-label="Fluxo"
        sx={{ height: '100%', width: '100%', bgcolor: tokens.color.studio.canvasBg }}
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
          <Background
            variant={BackgroundVariant.Dots}
            gap={20}
            size={1.5}
            color={tokens.color.studio.canvasDot}
          />
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
