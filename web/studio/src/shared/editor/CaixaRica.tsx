import { Box } from '@mui/material';
import { EditorContent, useEditor, useEditorState } from '@tiptap/react';
import { type ReactNode, useEffect, useRef } from 'react';
import { CAMPO, FONTE_MONO, tokens } from '@/theme/tokens';
import { BarraDeFormatacao } from './BarraDeFormatacao';
import { EXTENSOES } from './extensoes';
import { type Aberto, salvar } from './markdownFiel';

type Props = {
  aberto: Aberto;
  onChange: (markdown: string) => void;
  rotulo: string;
  editavel: boolean;
  minLinhas: number;
  placeholder?: string;
  /** Realce do Copiloto: o fundo da caixa acende quando uma proposta muda este texto. */
  realce?: { transition?: string; bgcolor?: string };
};

/** O editor de texto com formatação. Só avisa `onChange` quando a pessoa edita. */
export function CaixaRica({
  aberto,
  onChange,
  rotulo,
  editavel,
  minLinhas,
  placeholder,
  realce,
}: Props): ReactNode {
  const abertoAtual = useRef(aberto);
  const onChangeAtual = useRef(onChange);
  useEffect(() => {
    onChangeAtual.current = onChange;
  });

  const editor = useEditor({
    extensions: EXTENSOES,
    content: aberto.doc,
    editable: editavel,
    editorProps: {
      attributes: {
        role: 'textbox',
        'aria-multiline': 'true',
        'aria-label': rotulo,
        'aria-readonly': String(!editavel),
      },
    },
    onUpdate: ({ editor: atual }) =>
      onChangeAtual.current(salvar(atual.getJSON(), abertoAtual.current)),
  });

  // Texto trocado por fora (versão nova do servidor, modelo, tom): mostra sem avisar de volta.
  useEffect(() => {
    if (abertoAtual.current === aberto) return;
    abertoAtual.current = aberto;
    editor.commands.setContent(aberto.doc, { emitUpdate: false });
  }, [aberto, editor]);

  useEffect(() => editor.setEditable(editavel, false), [editor, editavel]);

  const vazio = useEditorState({ editor, selector: ({ editor: atual }) => atual.isEmpty });

  return (
    <Box
      sx={(theme) => ({
        borderRadius: `${CAMPO.raio}px`,
        bgcolor: tokens.color.background.subtle,
        boxShadow: `inset 0 0 0 1px ${CAMPO.bordaEmRepouso}`,
        transition: theme.transitions.create(['background-color', 'box-shadow']),
        '&:hover:not(:focus-within)': { boxShadow: `inset 0 0 0 1px ${CAMPO.bordaNoHover}` },
        '&:focus-within': {
          bgcolor: 'background.paper',
          boxShadow: `inset 0 0 0 2px ${theme.palette.primary.main}`,
        },
        ...realce,
      })}
    >
      {editavel && <BarraDeFormatacao editor={editor} rotulo={rotulo} />}
      <Box sx={{ position: 'relative' }}>
        {vazio && placeholder !== undefined && (
          <Box
            aria-hidden
            sx={{
              position: 'absolute',
              inset: 0,
              ...ESTILO_DO_TEXTO,
              color: 'text.secondary',
              pointerEvents: 'none',
            }}
          >
            {placeholder}
          </Box>
        )}
        <Box
          component={EditorContent}
          editor={editor}
          sx={{
            '& .ProseMirror': {
              ...ESTILO_DO_TEXTO,
              minHeight: `${minLinhas * 1.6}em`,
              outline: 'none',
            },
          }}
        />
      </Box>
    </Box>
  );
}

const ESTILO_DO_TEXTO = {
  px: '14px',
  py: '12px',
  typography: 'body1',
  lineHeight: 1.6,
  '& p, & ul, & ol': { my: 0 },
  '& ul, & ol': { pl: 3 },
  '& h1': { typography: 'h5', mt: 0.5, mb: 1 },
  '& h2': { typography: 'h6', mt: 1, mb: 0.5 },
  '& h3': { typography: 'subtitle1', fontWeight: 700, mt: 1, mb: 0.5 },
  '& code': {
    fontFamily: FONTE_MONO,
    fontSize: '0.9em',
    bgcolor: tokens.color.background.neutralFill,
    borderRadius: 0.5,
    px: 0.5,
  },
} as const;
