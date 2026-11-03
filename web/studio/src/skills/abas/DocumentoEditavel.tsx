import DeleteOutlinedIcon from '@mui/icons-material/DeleteOutlined';
import { Box, IconButton, Paper, Typography } from '@mui/material';
import { useQueryClient } from '@tanstack/react-query';
import type { ReactNode } from 'react';
import { useForm, useWatch } from 'react-hook-form';
import { chaveDoArquivo, salvarArquivo, useArquivo } from '@/catalogo/contratos';
import { SaveIndicator } from '@/shared/autosave/SaveIndicator';
import { useAutosave } from '@/shared/autosave/useAutosave';
import { CampoTexto } from '@/shared/ui/CampoTexto';
import { ErroComRetry } from '@/shared/ui/ErroComRetry';

type Props = {
  agente: string;
  caminho: string;
  titulo: string;
  editavel: boolean;
  onExcluir?: () => void;
  /** Arquivo em exclusão: gravações pendentes são descartadas para não recriá-lo. */
  descartar?: boolean;
};

export function DocumentoEditavel(props: Props): ReactNode {
  const { agente, caminho, titulo } = props;
  const arquivo = useArquivo(agente, caminho);

  if (arquivo.isPending) return <Typography variant="caption">Carregando {titulo}…</Typography>;
  if (arquivo.isError) {
    return (
      <ErroComRetry
        mensagem={`Não foi possível abrir ${titulo}.`}
        onTentarDeNovo={() => void arquivo.refetch()}
      />
    );
  }
  return <Editor {...props} inicial={arquivo.data.conteudo} />;
}

/** Monta só depois da leitura: o texto nasce do conteúdo salvo e é o `valor` estável do autosave. */
function Editor({
  agente,
  caminho,
  titulo,
  editavel,
  onExcluir,
  descartar = false,
  inicial,
}: Props & { inicial: string }): ReactNode {
  const cliente = useQueryClient();
  const { control } = useForm({ defaultValues: { texto: inicial } });
  // Mantém o cache em dia: voltar à aba remonta o editor a partir do que foi salvo, não do que foi lido.
  const salvar = async (conteudo: string): Promise<void> => {
    await salvarArquivo({ agente, caminho, conteudo });
    cliente.setQueryData(chaveDoArquivo(agente, caminho), { caminho, conteudo });
  };
  const { estado, tentarDeNovo } = useAutosave({
    valor: useWatch({ control, name: 'texto' }),
    salvar,
    bloqueio: descartar ? 'arquivo em exclusão' : null,
  });

  return (
    <Paper variant="outlined" sx={{ p: 1.5, display: 'flex', flexDirection: 'column', gap: 1 }}>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
        <Typography variant="subtitle2" sx={{ flex: 1 }}>
          {titulo}
        </Typography>
        {!descartar && <SaveIndicator estado={estado} onTentarDeNovo={() => void tentarDeNovo()} />}
        {editavel && onExcluir && (
          <IconButton size="small" aria-label={`Excluir ${titulo}`} onClick={onExcluir}>
            <DeleteOutlinedIcon fontSize="small" />
          </IconButton>
        )}
      </Box>
      <CampoTexto
        control={control}
        name="texto"
        multiline
        minRows={4}
        fullWidth
        disabled={!editavel}
        slotProps={{ htmlInput: { 'aria-label': titulo } }}
      />
    </Paper>
  );
}
