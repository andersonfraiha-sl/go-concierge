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
import { tokens } from '@/theme/tokens';
import { mensagemDeErro, useRecarregarAvancado } from './contratos';
import { ESTILO_DO_EDITOR, PADDING_TOPO_DO_EDITOR } from './estilo';

type EditorProps = { agente: string; caminho: string; editar: boolean; onExcluido: () => void };

export function EditorDeArquivo(props: EditorProps): ReactNode {
  const arquivo = useArquivo(props.agente, props.caminho);
  if (arquivo.isPending) return <Typography color="text.secondary">Carregando…</Typography>;
  if (arquivo.isError) {
    return (
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
        <Typography role="alert">Não foi possível abrir o arquivo.</Typography>
        <Button size="small" onClick={() => void arquivo.refetch()}>
          Tentar de novo
        </Button>
      </Box>
    );
  }
  return <EditorCarregado {...props} conteudoGravado={arquivo.data.conteudo} />;
}

function EditorCarregado({
  agente,
  caminho,
  editar,
  onExcluido,
  conteudoGravado,
}: EditorProps & { conteudoGravado: string }): ReactNode {
  const { mostrar } = useSnackbar();
  const recarregar = useRecarregarAvancado();
  const [excluindo, setExcluindo] = useState(false);
  const { control, handleSubmit, reset } = useForm({ defaultValues: { texto: conteudoGravado } });
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
  const numerosRef = useRef<HTMLPreElement>(null);

  const sincronizarScroll = (evento: UIEvent<HTMLTextAreaElement>): void => {
    if (numerosRef.current) numerosRef.current.scrollTop = evento.currentTarget.scrollTop;
  };

  const salvar = handleSubmit(async ({ texto }) => {
    try {
      await sincronia.salvar(texto);
      mostrar(`Arquivo ${caminho} salvo`);
    } catch (erro) {
      mostrar(mensagemDeErro(erro), 'error');
    }
  });

  const excluir = async (): Promise<void> => {
    setExcluindo(false);
    try {
      await excluirArquivo(agente, caminho);
      await recarregar();
      onExcluido();
      mostrar(`Arquivo ${caminho} excluído`);
    } catch (erro) {
      mostrar(mensagemDeErro(erro), 'error');
    }
  };

  return (
    <SectionCard
      titulo={caminho}
      ajuda="O texto exatamente como está gravado."
      acao={
        editar && (
          <Box sx={{ display: 'flex', gap: 1 }}>
            <Button color="error" onClick={() => setExcluindo(true)}>
              Excluir
            </Button>
            <Button
              variant="contained"
              disabled={sincronia.conflito !== null}
              onClick={() => void salvar()}
            >
              Salvar
            </Button>
          </Box>
        )
      }
    >
      <AvisoDeConflito
        conflito={sincronia.conflito}
        onManterOMeu={sincronia.manterOMeu}
        onUsarONovo={sincronia.usarONovo}
      />
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
      <ConfirmDialog
        aberto={excluindo}
        titulo={`Excluir ${caminho}?`}
        texto="O arquivo sai deste item e não dá para desfazer."
        confirmar="Excluir"
        onConfirmar={() => void excluir()}
        onCancelar={() => setExcluindo(false)}
      />
    </SectionCard>
  );
}

type NumerosProps = { control: Control<{ texto: string }> };

// ponytail: uma linha lógica é uma linha visual (ESTILO_DO_EDITOR usa `pre`,
// sem quebra), então contar por `\n` não desalinha com texto longo; o
// scroll vertical é sincronizado por `sincronizarScroll` em EditorCarregado.
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
        color: tokens.color.text.muted,
        borderRight: `1px solid ${tokens.color.border.subtle}`,
        overflowY: 'hidden',
        userSelect: 'none',
      }}
    >
      {numeros}
    </Typography>
  );
});
