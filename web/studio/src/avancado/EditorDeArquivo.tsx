import { Box, Button, Typography } from '@mui/material';
import { type ReactNode, useState } from 'react';
import { type Control, useForm, useWatch } from 'react-hook-form';
import { excluirArquivo, salvarArquivo, useArquivo } from '@/catalogo/contratos';
import { useSnackbar } from '@/shared/snackbar/SnackbarContext';
import { CampoTexto } from '@/shared/ui/CampoTexto';
import { ConfirmDialog } from '@/shared/ui/ConfirmDialog';
import { SectionCard } from '@/shared/ui/SectionCard';
import { tokens } from '@/theme/tokens';
import { mensagemDeErro, useRecarregarAvancado } from './contratos';
import { ESTILO_DE_CODIGO } from './estilo';

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
  return <EditorCarregado {...props} conteudoInicial={arquivo.data.conteudo} />;
}

function EditorCarregado({
  agente,
  caminho,
  editar,
  onExcluido,
  conteudoInicial,
}: EditorProps & { conteudoInicial: string }): ReactNode {
  const { mostrar } = useSnackbar();
  const recarregar = useRecarregarAvancado();
  const [excluindo, setExcluindo] = useState(false);
  const { control, handleSubmit } = useForm({ defaultValues: { texto: conteudoInicial } });

  const salvar = handleSubmit(async ({ texto }) => {
    try {
      await salvarArquivo({ agente, caminho, conteudo: texto });
      await recarregar();
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
            <Button variant="contained" onClick={() => void salvar()}>
              Salvar
            </Button>
          </Box>
        )
      }
    >
      <Box sx={{ display: 'flex' }}>
        <NumerosDeLinha control={control} />
        <CampoTexto
          control={control}
          name="texto"
          multiline
          minRows={16}
          fullWidth
          slotProps={{
            htmlInput: {
              readOnly: !editar,
              'aria-label': `Conteúdo de ${caminho}`,
              style: ESTILO_DE_CODIGO,
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

// ponytail: a coluna acompanha o número de linhas do texto, não o minRows do
// campo (que força 16 linhas mínimas); alinhamento pixel a pixel com o
// textarea, se precisar, pede sincronizar scroll e altura via ref.
function NumerosDeLinha({ control }: NumerosProps): ReactNode {
  const texto = useWatch({ control, name: 'texto' });
  const total = (texto ?? '').split('\n').length;
  const numeros = Array.from({ length: total }, (_, indice) => indice + 1).join('\n');
  return (
    <Typography
      aria-hidden
      component="pre"
      sx={{
        ...ESTILO_DE_CODIGO,
        pt: '16.5px',
        px: 1.5,
        textAlign: 'right',
        color: tokens.color.text.muted,
        borderRight: `1px solid ${tokens.color.border.subtle}`,
        userSelect: 'none',
      }}
    >
      {numeros}
    </Typography>
  );
}
