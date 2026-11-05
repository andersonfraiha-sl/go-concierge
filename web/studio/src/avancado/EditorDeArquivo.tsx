import EditOutlinedIcon from '@mui/icons-material/EditOutlined';
import { Box, Button, Typography, useTheme } from '@mui/material';
import { forwardRef, type ReactNode, type UIEvent, useRef, useState } from 'react';
import { type Control, useForm, useWatch } from 'react-hook-form';
import { excluirArquivo, salvarArquivo, useArquivo } from '@/catalogo/contratos';
import { AvisoDeConflito } from '@/copiloto/AvisoDeConflito';
import { useEdicaoSobOServidor } from '@/copiloto/useEdicaoSobOServidor';
import { useSnackbar } from '@/shared/snackbar/SnackbarContext';
import { CampoTexto } from '@/shared/ui/CampoTexto';
import { ConfirmDialog } from '@/shared/ui/ConfirmDialog';
import {
  FUNDO_DO_EDITOR,
  PADDING_TOPO_DO_EDITOR,
  PERIGO_SUAVE,
  PESOS,
  RAIO,
  TIPOS,
} from '@/theme/tokens';
import { mensagemDeErro, useRecarregarAvancado } from './contratos';
import { ESTILO_DE_CODIGO, ESTILO_DO_EDITOR } from './estilo';
import { FalhaComTentarDeNovo } from './FalhaComTentarDeNovo';
import { realcarYaml } from './realceYaml';

type EditorProps = { agente: string; caminho: string; editar: boolean; onExcluido: () => void };

export function EditorDeArquivo(props: EditorProps): ReactNode {
  const arquivo = useArquivo(props.agente, props.caminho);
  if (arquivo.isPending) return <Typography color="textSecondary">Carregando…</Typography>;
  if (arquivo.isError) {
    return (
      <FalhaComTentarDeNovo
        mensagem="Não foi possível abrir o arquivo."
        onTentarDeNovo={() => void arquivo.refetch()}
      />
    );
  }
  return <EditorCarregado {...props} conteudoGravado={arquivo.data.conteudo} />;
}

type TextoDoArquivo = { texto: string };

function useSincroniaDoArquivo(
  { agente, caminho }: Pick<EditorProps, 'agente' | 'caminho'>,
  conteudoGravado: string,
) {
  const recarregar = useRecarregarAvancado();
  const { control, handleSubmit, reset } = useForm<TextoDoArquivo>({
    defaultValues: { texto: conteudoGravado },
  });
  const sincronia = useEdicaoSobOServidor({
    servidor: conteudoGravado,
    alvos: [`arquivo:${agente}:${caminho}`, `arquivo::${caminho}`],
    versao: null,
    local: useWatch({ control, name: 'texto' }),
    resetar: (texto) => reset({ texto }),
    salvar: async (conteudo) => {
      await salvarArquivo({ agente, caminho, conteudo });
      await recarregar();
      return null;
    },
  });
  const descartar = () => reset({ texto: conteudoGravado });
  return { control, handleSubmit, sincronia, descartar };
}

const bytesDoTexto = (texto: string): number => new TextEncoder().encode(texto).length;

function EditorCarregado({
  agente,
  caminho,
  editar,
  onExcluido,
  conteudoGravado,
}: EditorProps & { conteudoGravado: string }): ReactNode {
  const { mostrar } = useSnackbar();
  const [excluindo, setExcluindo] = useState(false);
  // Abre só para leitura; Editar libera o texto e troca o botão por Salvar e Excluir.
  const [editando, setEditando] = useState(false);
  const { control, handleSubmit, sincronia, descartar } = useSincroniaDoArquivo(
    { agente, caminho },
    conteudoGravado,
  );
  const excluir = useExcluirArquivo({ agente, caminho }, onExcluido);

  const salvar = handleSubmit(async ({ texto }) => {
    try {
      await sincronia.salvar(texto);
      setEditando(false);
      mostrar(`Arquivo ${caminho} salvo`);
    } catch (erro) {
      mostrar(mensagemDeErro(erro), 'error');
    }
  });

  return (
    <Box
      sx={{
        flex: 1,
        minWidth: 0,
        display: 'flex',
        flexDirection: 'column',
        border: 1,
        borderColor: 'border.default',
        borderRadius: RAIO.cartao,
        overflow: 'hidden',
      }}
    >
      <CabecalhoDoEditor
        caminho={caminho}
        bytes={bytesDoTexto(conteudoGravado)}
        modo={modoDoEditor(editar, editando)}
        salvarBloqueado={sincronia.conflito !== null}
        onEditar={() => setEditando(true)}
        onCancelar={() => {
          descartar();
          setEditando(false);
        }}
        onExcluir={() => setExcluindo(true)}
        onSalvar={() => void salvar()}
      />
      <AvisoDeConflito
        conflito={sincronia.conflito}
        onManterOMeu={sincronia.manterOMeu}
        onUsarONovo={sincronia.usarONovo}
      />
      <AreaDoTexto control={control} caminho={caminho} editar={editar && editando} />
      <ConfirmDialog
        aberto={excluindo}
        titulo={`Excluir ${caminho}?`}
        texto="O arquivo sai deste item e não dá para desfazer."
        confirmar="Excluir"
        onConfirmar={() => {
          setExcluindo(false);
          void excluir();
        }}
        onCancelar={() => setExcluindo(false)}
      />
    </Box>
  );
}

function useExcluirArquivo(
  { agente, caminho }: Pick<EditorProps, 'agente' | 'caminho'>,
  onExcluido: () => void,
): () => Promise<void> {
  const { mostrar } = useSnackbar();
  const recarregar = useRecarregarAvancado();
  return async () => {
    try {
      await excluirArquivo(agente, caminho);
      await recarregar();
    } catch (erro) {
      mostrar(mensagemDeErro(erro), 'error');
      return;
    }
    onExcluido();
    mostrar(`Arquivo ${caminho} excluído`);
  };
}

type ModoDoEditor = 'leitura' | 'pode-editar' | 'editando';

const modoDoEditor = (editar: boolean, editando: boolean): ModoDoEditor => {
  if (!editar) return 'leitura';
  return editando ? 'editando' : 'pode-editar';
};

type CabecalhoProps = {
  caminho: string;
  bytes: number;
  modo: ModoDoEditor;
  salvarBloqueado: boolean;
  onEditar: () => void;
  onCancelar: () => void;
  onExcluir: () => void;
  onSalvar: () => void;
};

function CabecalhoDoEditor({ caminho, bytes, modo, ...acoes }: CabecalhoProps): ReactNode {
  return (
    <Box
      sx={{
        height: 50,
        px: 3,
        display: 'flex',
        alignItems: 'center',
        gap: 2,
        borderBottom: 1,
        borderColor: 'border.subtle',
      }}
    >
      <Typography sx={{ fontFamily: ESTILO_DE_CODIGO.fontFamily, fontSize: TIPOS.body2 }}>
        {caminho}
      </Typography>
      <Typography
        component="span"
        sx={{
          fontSize: TIPOS.overline,
          px: 1,
          py: 0.375,
          borderRadius: RAIO.pilula,
          bgcolor: 'background.neutralFill',
          color: 'text.secondary',
        }}
      >
        {bytes} bytes
      </Typography>
      <Box sx={{ flex: 1 }} />
      {modo === 'pode-editar' && (
        <Button
          variant="outlined"
          startIcon={<EditOutlinedIcon fontSize="small" />}
          onClick={acoes.onEditar}
          sx={{ fontWeight: PESOS.semi }}
        >
          Editar
        </Button>
      )}
      {modo === 'editando' && <AcoesDaEdicao {...acoes} />}
    </Box>
  );
}

type AcoesProps = Omit<CabecalhoProps, 'caminho' | 'bytes' | 'modo' | 'onEditar'>;

function AcoesDaEdicao({
  salvarBloqueado,
  onCancelar,
  onExcluir,
  onSalvar,
}: AcoesProps): ReactNode {
  return (
    <>
      <Button onClick={onCancelar} sx={{ color: 'text.secondary' }}>
        Cancelar
      </Button>
      <Button
        variant="outlined"
        onClick={onExcluir}
        sx={{ borderColor: PERIGO_SUAVE.borda, color: PERIGO_SUAVE.texto }}
      >
        Excluir
      </Button>
      <Button
        variant="contained"
        disabled={salvarBloqueado}
        onClick={onSalvar}
        sx={{ fontWeight: PESOS.semi }}
      >
        Salvar
      </Button>
    </>
  );
}

type AreaProps = { control: Control<TextoDoArquivo>; caminho: string; editar: boolean };

function AreaDoTexto({ control, caminho, editar }: AreaProps): ReactNode {
  // O texto é transparente (as cores vêm do realce por baixo); o cursor precisa de cor própria.
  const corDoCursor = useTheme().palette.text.primary;
  const numerosRef = useRef<HTMLPreElement>(null);
  const realceRef = useRef<HTMLPreElement>(null);
  const sincronizarScroll = (evento: UIEvent<HTMLTextAreaElement>): void => {
    if (numerosRef.current) numerosRef.current.scrollTop = evento.currentTarget.scrollTop;
    if (realceRef.current) {
      realceRef.current.scrollTop = evento.currentTarget.scrollTop;
      realceRef.current.scrollLeft = evento.currentTarget.scrollLeft;
    }
  };
  return (
    <Box sx={{ display: 'flex', flex: 1, minHeight: 0, bgcolor: FUNDO_DO_EDITOR }}>
      <NumerosDeLinha ref={numerosRef} control={control} />
      <Box sx={{ position: 'relative', flex: 1, minWidth: 0 }}>
        <RealceDoTexto ref={realceRef} control={control} />
        <CampoTexto
          control={control}
          name="texto"
          multiline
          rows={1}
          fullWidth
          variant="standard"
          slotProps={{
            input: { disableUnderline: true },
            htmlInput: {
              readOnly: !editar,
              'aria-label': `Conteúdo de ${caminho}`,
              style: { ...ESTILO_DO_EDITOR, color: 'transparent', caretColor: corDoCursor },
              onScroll: sincronizarScroll,
            },
          }}
          sx={{
            position: 'absolute',
            inset: 0,
            zIndex: 1,
            bgcolor: 'transparent',
            '& .MuiInputBase-root': { bgcolor: 'transparent', p: 0, height: '100%' },
            // O texto ocupa a área toda: as barras de rolagem ficam na borda do editor.
            // !important vence o height/overflow inline do autosize do MUI. Mesmo recuo e
            // entrelinha do realce, senão o texto digitado sai deslocado das cores.
            '& .MuiInputBase-input': {
              height: '100% !important',
              overflow: 'auto !important',
              boxSizing: 'border-box',
              pt: PADDING_TOPO_DO_EDITOR,
              px: 1.75,
              pb: 2,
            },
          }}
        />
      </Box>
    </Box>
  );
}

// ponytail: realce raso por regex (chave/comentário), não um parser YAML completo;
// cobre o que o desenho aprovado mostra. Upgrade: tokenizer de verdade se o YAML
// real usar blocos mais ricos (listas aninhadas, âncoras).
const RealceDoTexto = forwardRef<HTMLPreElement, { control: Control<TextoDoArquivo> }>(
  function RealceDoTexto({ control }, ref): ReactNode {
    const texto = useWatch({ control, name: 'texto' });
    const corDeComentario = useTheme().palette.text.muted;
    return (
      <Typography
        ref={ref}
        aria-hidden
        component="pre"
        sx={{
          ...ESTILO_DO_EDITOR,
          position: 'absolute',
          inset: 0,
          pt: PADDING_TOPO_DO_EDITOR,
          px: 1.75,
          overflow: 'hidden',
          pointerEvents: 'none',
        }}
      >
        {realcarYaml(texto ?? '', corDeComentario)}
      </Typography>
    );
  },
);

type NumerosProps = { control: Control<TextoDoArquivo> };

// ponytail: uma linha lógica é uma linha visual (ESTILO_DO_EDITOR usa `pre`,
// sem quebra), então contar por `\n` não desalinha com texto longo; o
// scroll vertical é sincronizado por `sincronizarScroll` em AreaDoTexto.
const NumerosDeLinha = forwardRef<HTMLPreElement, NumerosProps>(function NumerosDeLinha(
  { control },
  ref,
): ReactNode {
  const texto = useWatch({ control, name: 'texto' });
  const total = (texto ?? '').split('\n').length;
  const numeros = Array.from({ length: total }, (_, indice) => indice + 1).join('\n');
  return (
    <Typography
      ref={ref}
      aria-hidden
      component="pre"
      sx={{
        ...ESTILO_DO_EDITOR,
        pt: PADDING_TOPO_DO_EDITOR,
        px: 1.5,
        textAlign: 'right',
        color: 'text.muted',
        borderRight: 1,
        borderColor: 'border.subtle',
        overflowY: 'hidden',
        userSelect: 'none',
      }}
    >
      {numeros}
    </Typography>
  );
});
