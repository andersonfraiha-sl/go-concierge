import { Box, Button, Typography } from '@mui/material';
import { forwardRef, type ReactNode, type UIEvent, useRef, useState } from 'react';
import { type Control, useForm, useWatch } from 'react-hook-form';
import { excluirArquivo, salvarArquivo, useArquivo } from '@/catalogo/contratos';
import { AvisoDeConflito } from '@/copiloto/AvisoDeConflito';
import { useEdicaoSobOServidor } from '@/copiloto/useEdicaoSobOServidor';
import { useSnackbar } from '@/shared/snackbar/SnackbarContext';
import { CampoTexto } from '@/shared/ui/CampoTexto';
import { ConfirmDialog } from '@/shared/ui/ConfirmDialog';
import { SectionCard } from '@/shared/ui/SectionCard';
import { PADDING_TOPO_DO_EDITOR } from '@/theme/tokens';
import { mensagemDeErro, useRecarregarAvancado } from './contratos';
import { ESTILO_DO_EDITOR } from './estilo';
import { FalhaComTentarDeNovo } from './FalhaComTentarDeNovo';

type EditorProps = { agente: string; caminho: string; editar: boolean; onExcluido: () => void };

export function EditorDeArquivo(props: EditorProps): ReactNode {
  const arquivo = useArquivo(props.agente, props.caminho);
  if (arquivo.isPending) return <Typography color="text.secondary">Carregando…</Typography>;
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
  return { control, handleSubmit, sincronia };
}

function EditorCarregado({
  agente,
  caminho,
  editar,
  onExcluido,
  conteudoGravado,
}: EditorProps & { conteudoGravado: string }): ReactNode {
  const { mostrar } = useSnackbar();
  const [excluindo, setExcluindo] = useState(false);
  const { control, handleSubmit, sincronia } = useSincroniaDoArquivo(
    { agente, caminho },
    conteudoGravado,
  );
  const excluir = useExcluirArquivo({ agente, caminho }, onExcluido);

  const salvar = handleSubmit(async ({ texto }) => {
    try {
      await sincronia.salvar(texto);
      mostrar(`Arquivo ${caminho} salvo`);
    } catch (erro) {
      mostrar(mensagemDeErro(erro), 'error');
    }
  });

  return (
    <SectionCard
      titulo={caminho}
      ajuda="O texto exatamente como está gravado."
      acao={
        editar && (
          <AcoesDoEditor
            salvarBloqueado={sincronia.conflito !== null}
            onExcluir={() => setExcluindo(true)}
            onSalvar={() => void salvar()}
          />
        )
      }
    >
      <AvisoDeConflito
        conflito={sincronia.conflito}
        onManterOMeu={sincronia.manterOMeu}
        onUsarONovo={sincronia.usarONovo}
      />
      <AreaDoTexto control={control} caminho={caminho} editar={editar} />
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
    </SectionCard>
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

type AcoesProps = { salvarBloqueado: boolean; onExcluir: () => void; onSalvar: () => void };

function AcoesDoEditor({ salvarBloqueado, onExcluir, onSalvar }: AcoesProps): ReactNode {
  return (
    <Box sx={{ display: 'flex', gap: 1 }}>
      <Button color="error" onClick={onExcluir}>
        Excluir
      </Button>
      <Button variant="contained" disabled={salvarBloqueado} onClick={onSalvar}>
        Salvar
      </Button>
    </Box>
  );
}

type AreaProps = { control: Control<TextoDoArquivo>; caminho: string; editar: boolean };

function AreaDoTexto({ control, caminho, editar }: AreaProps): ReactNode {
  const numerosRef = useRef<HTMLPreElement>(null);
  const sincronizarScroll = (evento: UIEvent<HTMLTextAreaElement>): void => {
    if (numerosRef.current) numerosRef.current.scrollTop = evento.currentTarget.scrollTop;
  };
  return (
    <Box sx={{ display: 'flex' }}>
      <NumerosDeLinha ref={numerosRef} control={control} />
      <CampoTexto
        control={control}
        name="texto"
        multiline
        minRows={16}
        maxRows={16}
        fullWidth
        sx={{ overflowX: 'auto' }}
        slotProps={{
          htmlInput: {
            readOnly: !editar,
            'aria-label': `Conteúdo de ${caminho}`,
            style: ESTILO_DO_EDITOR,
            onScroll: sincronizarScroll,
          },
        }}
      />
    </Box>
  );
}

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
