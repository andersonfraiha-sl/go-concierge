import CodeIcon from '@mui/icons-material/Code';
import FormatBoldIcon from '@mui/icons-material/FormatBold';
import FormatItalicIcon from '@mui/icons-material/FormatItalic';
import FormatListBulletedIcon from '@mui/icons-material/FormatListBulleted';
import FormatListNumberedIcon from '@mui/icons-material/FormatListNumbered';
import FormatUnderlinedIcon from '@mui/icons-material/FormatUnderlined';
import RedoIcon from '@mui/icons-material/Redo';
import UndoIcon from '@mui/icons-material/Undo';
import { Box, Divider, IconButton, Tooltip, Typography } from '@mui/material';
import { type Editor, useEditorState } from '@tiptap/react';
import { Fragment, type ReactNode } from 'react';
import { PESOS, RAIO, TIPOS } from '@/theme/tokens';

type Acao = {
  rotulo: string;
  atalho: string;
  icone: ReactNode;
  ativa?: (editor: Editor) => boolean;
  /** Desfazer e refazer: sem histórico, o botão fica desabilitado. */
  disponivel?: (editor: Editor) => boolean;
  executar: (editor: Editor) => void;
};

const titulo = (nivel: 2 | 3): Acao => ({
  rotulo: nivel === 2 ? 'Título' : 'Subtítulo',
  atalho: `Ctrl+Alt+${nivel}`,
  icone: (
    <Typography sx={{ fontSize: TIPOS.body2, fontWeight: PESOS.negrito, lineHeight: 1 }}>
      H{nivel}
    </Typography>
  ),
  ativa: (editor) => editor.isActive('heading', { level: nivel }),
  executar: (editor) => editor.chain().focus().toggleHeading({ level: nivel }).run(),
});

type DadosDaMarca = { nome: string; rotulo: string; atalho: string; icone: ReactNode };

const marca = ({ nome, rotulo, atalho, icone }: DadosDaMarca): Acao => ({
  rotulo,
  atalho,
  icone,
  ativa: (editor) => editor.isActive(nome),
  executar: (editor) => editor.chain().focus().toggleMark(nome).run(),
});

const GRUPOS: readonly (readonly Acao[])[] = [
  [titulo(2), titulo(3)],
  [
    marca({
      nome: 'bold',
      rotulo: 'Negrito',
      atalho: 'Ctrl+B',
      icone: <FormatBoldIcon fontSize="small" />,
    }),
    marca({
      nome: 'italic',
      rotulo: 'Itálico',
      atalho: 'Ctrl+I',
      icone: <FormatItalicIcon fontSize="small" />,
    }),
    marca({
      nome: 'underline',
      rotulo: 'Sublinhado',
      atalho: 'Ctrl+U',
      icone: <FormatUnderlinedIcon fontSize="small" />,
    }),
  ],
  [
    {
      rotulo: 'Lista com marcadores',
      atalho: 'Ctrl+Shift+8',
      icone: <FormatListBulletedIcon fontSize="small" />,
      ativa: (editor) => editor.isActive('bulletList'),
      executar: (editor) => editor.chain().focus().toggleBulletList().run(),
    },
    {
      rotulo: 'Lista numerada',
      atalho: 'Ctrl+Shift+7',
      icone: <FormatListNumberedIcon fontSize="small" />,
      ativa: (editor) => editor.isActive('orderedList'),
      executar: (editor) => editor.chain().focus().toggleOrderedList().run(),
    },
  ],
  [
    marca({
      nome: 'code',
      rotulo: 'Código',
      atalho: 'Ctrl+E',
      icone: <CodeIcon fontSize="small" />,
    }),
  ],
  [
    {
      rotulo: 'Desfazer',
      atalho: 'Ctrl+Z',
      icone: <UndoIcon fontSize="small" />,
      disponivel: (editor) => editor.can().undo(),
      executar: (editor) => editor.chain().focus().undo().run(),
    },
    {
      rotulo: 'Refazer',
      atalho: 'Ctrl+Shift+Z',
      icone: <RedoIcon fontSize="small" />,
      disponivel: (editor) => editor.can().redo(),
      executar: (editor) => editor.chain().focus().redo().run(),
    },
  ],
];

type EstadoDoBotao = { ativa: boolean; disponivel: boolean };

const estadoDoBotao = (acao: Acao, editor: Editor): EstadoDoBotao => ({
  ativa: acao.ativa?.(editor) ?? false,
  disponivel: acao.disponivel?.(editor) ?? true,
});

const estadoDosBotoes = (editor: Editor): Record<string, EstadoDoBotao> =>
  Object.fromEntries(GRUPOS.flat().map((acao) => [acao.rotulo, estadoDoBotao(acao, editor)]));

const SEM_ESTADO: EstadoDoBotao = { ativa: false, disponivel: true };

type Props = { editor: Editor; rotulo: string };

export function BarraDeFormatacao({ editor, rotulo }: Props): ReactNode {
  // Relê a cada transação: mover a seleção acende ou apaga o que está ativo ali.
  const estados = useEditorState({
    editor,
    selector: ({ editor: atual }) => estadoDosBotoes(atual),
  });

  return (
    <Box role="toolbar" aria-label={`Formatação de ${rotulo}`} sx={ESTILO_DA_BARRA}>
      {GRUPOS.map((grupo, indice) => (
        <GrupoDeBotoes key={grupo[0]?.rotulo} grupo={grupo} separar={indice > 0}>
          {(acao) => (
            <Botao
              key={acao.rotulo}
              acao={acao}
              estado={estados[acao.rotulo] ?? SEM_ESTADO}
              onClick={() => acao.executar(editor)}
            />
          )}
        </GrupoDeBotoes>
      ))}
    </Box>
  );
}

const ESTILO_DA_BARRA = {
  display: 'flex',
  flexWrap: 'wrap',
  alignItems: 'center',
  gap: 0.25,
  px: 0.75,
  py: 0.5,
  borderBottom: 1,
  borderColor: 'divider',
} as const;

type PropsDoGrupo = {
  grupo: readonly Acao[];
  separar: boolean;
  children: (acao: Acao) => ReactNode;
};

function GrupoDeBotoes({ grupo, separar, children }: PropsDoGrupo): ReactNode {
  return (
    <Fragment>
      {separar && <Divider orientation="vertical" flexItem sx={{ mx: 0.5, my: 0.5 }} />}
      {grupo.map(children)}
    </Fragment>
  );
}

type PropsDoBotao = { acao: Acao; estado: EstadoDoBotao; onClick: () => void };

const estiloDoBotao = (ativa: boolean) => ({
  borderRadius: RAIO.campo,
  width: 30,
  height: 30,
  ...(ativa && { color: 'primary.main', bgcolor: 'background.tint' }),
});

function Botao({ acao, estado, onClick }: PropsDoBotao): ReactNode {
  return (
    <Tooltip title={`${acao.rotulo} (${acao.atalho})`}>
      {/* span: o Tooltip precisa de um alvo que receba eventos mesmo com o botão desabilitado. */}
      <span>
        <IconButton
          size="small"
          aria-label={acao.rotulo}
          aria-pressed={acao.ativa === undefined ? undefined : estado.ativa}
          disabled={!estado.disponivel}
          onClick={onClick}
          sx={estiloDoBotao(estado.ativa)}
        >
          {acao.icone}
        </IconButton>
      </span>
    </Tooltip>
  );
}
